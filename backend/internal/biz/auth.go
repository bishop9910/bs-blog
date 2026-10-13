package biz

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/bishop9910/bs-blog/backend/internal/data"
	"github.com/bishop9910/bs-blog/backend/internal/model"
	"github.com/bishop9910/bs-blog/backend/internal/pkg/secure"
)

// Credential rules. The password floor is deliberately not configurable: this
// single password is the only thing standing between the public internet and
// the ability to publish as the owner.
const (
	minUsernameRunes = 2
	maxUsernameRunes = 32
	minPasswordRunes = 8
	maxPasswordRunes = 128
	fallbackBcrypt   = 12
)

// SetupStateDTO tells the login screen whether the blog has an owner yet.
type SetupStateDTO struct {
	Initialised bool `json:"initialised"`
}

// CurrentUser is the identity attached to an authenticated request.
type CurrentUser struct {
	ID       int64
	Username string
}

// AuthUsecase owns sign-in, first-run claim, and password changes.
type AuthUsecase struct {
	store      *data.Store
	signer     *secure.Signer
	bcryptCost int
}

// NewAuthUsecase wires the use case.
func NewAuthUsecase(store *data.Store, signer *secure.Signer, bcryptCost int) *AuthUsecase {
	if bcryptCost < bcrypt.MinCost || bcryptCost > bcrypt.MaxCost {
		bcryptCost = fallbackBcrypt
	}
	return &AuthUsecase{store: store, signer: signer, bcryptCost: bcryptCost}
}

// SetupState reports whether an administrator account exists.
func (uc *AuthUsecase) SetupState(ctx context.Context) (*SetupStateDTO, error) {
	count, err := uc.store.CountUsers(ctx)
	if err != nil {
		return nil, internal("无法读取站点初始化状态", err)
	}
	return &SetupStateDTO{Initialised: count > 0}, nil
}

// Setup creates the single administrator account on first run.
//
// It refuses once any account exists, so the endpoint cannot be replayed to
// take over a blog that is already in use. The account is created as part of
// the same call that signs the owner in, so there is no window where the site
// is initialised but unreachable.
func (uc *AuthUsecase) Setup(ctx context.Context, in CredentialsInput) (*LoginResultDTO, error) {
	count, err := uc.store.CountUsers(ctx)
	if err != nil {
		return nil, internal("无法读取站点初始化状态", err)
	}
	if count > 0 {
		return nil, errForbidden("站点已完成初始化，请直接登录")
	}

	username, err := validateUsername(in.Username)
	if err != nil {
		return nil, err
	}
	if err := validatePassword(in.Password); err != nil {
		return nil, err
	}

	hash, err := secure.HashPassword(in.Password, uc.bcryptCost)
	if err != nil {
		return nil, internal("无法创建账号", err)
	}

	user := &model.User{
		Username:     username,
		Nickname:     username,
		PasswordHash: hash,
	}
	if err := uc.store.CreateUser(ctx, user); err != nil {
		// Two setup requests raced and the unique index rejected this one.
		if n, countErr := uc.store.CountUsers(ctx); countErr == nil && n > 0 {
			return nil, errConflict("站点已完成初始化，请直接登录")
		}
		return nil, internal("无法创建账号", err)
	}

	return uc.issue(user)
}

// Login exchanges credentials for a bearer token.
func (uc *AuthUsecase) Login(ctx context.Context, in CredentialsInput) (*LoginResultDTO, error) {
	username := strings.TrimSpace(in.Username)
	if username == "" || in.Password == "" {
		return nil, errBadRequest("请输入用户名和密码")
	}

	user, err := uc.store.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			// Spend a comparable amount of time hashing so that "no such user"
			// and "wrong password" are not distinguishable by response latency.
			_, _ = secure.HashPassword(in.Password, uc.bcryptCost)
			return nil, errUnauthorized("用户名或密码不正确")
		}
		return nil, internal("无法登录", err)
	}

	if !secure.VerifyPassword(user.PasswordHash, in.Password) {
		return nil, errUnauthorized("用户名或密码不正确")
	}

	now := time.Now().UTC()
	if err := uc.store.TouchLastLogin(ctx, user.ID); err != nil {
		// Not worth failing a valid login over.
		log.Errorf("biz: recording last login for user %d: %v", user.ID, err)
	}
	user.LastLoginAt = &now

	return uc.issue(user)
}

// Me returns the signed-in account.
func (uc *AuthUsecase) Me(ctx context.Context, userID int64) (*UserDTO, error) {
	user, err := uc.store.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return nil, errUnauthorized("登录状态已失效，请重新登录")
		}
		return nil, internal("无法读取账号信息", err)
	}
	dto := toUserDTO(*user)
	return &dto, nil
}

// ChangePassword replaces the owner's password and revokes existing sessions.
func (uc *AuthUsecase) ChangePassword(ctx context.Context, userID int64, in ChangePasswordInput) error {
	user, err := uc.store.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return errUnauthorized("登录状态已失效，请重新登录")
		}
		return internal("无法读取账号信息", err)
	}

	if !secure.VerifyPassword(user.PasswordHash, in.OldPassword) {
		return errBadRequest("当前密码不正确")
	}
	if err := validatePassword(in.NewPassword); err != nil {
		return err
	}
	if in.NewPassword == in.OldPassword {
		return errBadRequest("新密码不能与当前密码相同")
	}

	hash, err := secure.HashPassword(in.NewPassword, uc.bcryptCost)
	if err != nil {
		return internal("无法更新密码", err)
	}
	if err := uc.store.UpdatePasswordAndRevokeTokens(ctx, user.ID, hash); err != nil {
		return internal("无法更新密码", err)
	}
	return nil
}

// Verify resolves a bearer token to an identity.
//
// Two checks happen here that a signature check alone would miss: the account
// must still exist, and its TokenVersion must match the one baked into the
// token. The second is what makes "change password" log other sessions out.
func (uc *AuthUsecase) Verify(ctx context.Context, token string) (*CurrentUser, error) {
	claims, err := uc.signer.Parse(token)
	if err != nil {
		return nil, errUnauthorized("登录状态已失效，请重新登录")
	}

	user, err := uc.store.GetUserByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return nil, errUnauthorized("登录状态已失效，请重新登录")
		}
		return nil, internal("无法校验登录状态", err)
	}

	if user.TokenVersion != claims.TokenVersion {
		return nil, errUnauthorized("密码已变更，请重新登录")
	}

	return &CurrentUser{ID: user.ID, Username: user.Username}, nil
}

/* -------------------------------------------------------------------------- */
/* Helpers                                                                    */
/* -------------------------------------------------------------------------- */

func (uc *AuthUsecase) issue(user *model.User) (*LoginResultDTO, error) {
	token, expiresAt, err := uc.signer.Sign(user.ID, user.Username, user.TokenVersion)
	if err != nil {
		return nil, internal("无法签发登录凭证", err)
	}
	return &LoginResultDTO{
		Token:     token,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
		User:      toUserDTO(*user),
	}, nil
}

func validateUsername(raw string) (string, error) {
	username := strings.TrimSpace(raw)
	length := len([]rune(username))
	if length < minUsernameRunes || length > maxUsernameRunes {
		return "", errBadRequest("用户名长度需在 2 到 32 个字符之间")
	}
	if strings.ContainsAny(username, " \t\r\n") {
		return "", errBadRequest("用户名不能包含空格")
	}
	return username, nil
}

func validatePassword(password string) error {
	length := len([]rune(password))
	if length < minPasswordRunes {
		return errBadRequest("密码至少需要 8 个字符")
	}
	if length > maxPasswordRunes {
		return errBadRequest("密码过长")
	}
	return nil
}
