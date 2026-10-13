// Package secure handles the two cryptographic jobs this blog has: hashing the
// owner's password and signing session tokens.
package secure

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidToken is returned for any token that cannot be trusted: expired,
// malformed, wrong signature, or signed with a different algorithm.
var ErrInvalidToken = errors.New("secure: invalid token")

// HashPassword returns a bcrypt digest. bcrypt embeds its own salt and cost in
// the output, so the caller only has to store a single string.
func HashPassword(password string, cost int) (string, error) {
	digest, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("secure: hashing password: %w", err)
	}
	return string(digest), nil
}

// VerifyPassword reports whether password matches the stored digest. It returns
// false rather than an error for a mismatch so callers cannot accidentally
// treat "wrong password" as "storage failure".
func VerifyPassword(digest, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(digest), []byte(password)) == nil
}

// Claims is the payload carried by an access token.
type Claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"username"`
	// TokenVersion must match the user's current TokenVersion. Bumping it on
	// the user row is how "change password" logs out every other session.
	TokenVersion int64 `json:"tv"`
	jwt.RegisteredClaims
}

// Signer issues and verifies HS256 tokens.
type Signer struct {
	secret []byte
	ttl    time.Duration
}

// NewSigner builds a Signer over a shared secret.
func NewSigner(secret []byte, ttl time.Duration) *Signer {
	return &Signer{secret: secret, ttl: ttl}
}

// TTL is how long issued tokens last, so the API can tell the client when to
// expect re-authentication.
func (s *Signer) TTL() time.Duration { return s.ttl }

// Sign issues a token for one user, returning it with its expiry.
func (s *Signer) Sign(userID int64, username string, tokenVersion int64) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(s.ttl)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID:       userID,
		Username:     username,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Issuer:    "bs-blog",
		},
	})

	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("secure: signing token: %w", err)
	}
	return signed, expiresAt, nil
}

// Parse verifies a token and returns its claims.
//
// The algorithm is pinned to HS256: without that check a token signed with
// "none", or an RS256 token whose "key" is our HMAC secret, would be accepted.
func (s *Signer) Parse(raw string) (*Claims, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrInvalidToken
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer("bs-blog"))
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// LoadOrCreateSecret reads a hex-encoded signing secret from path, generating
// and persisting one on first run.
//
// Persisting matters: a secret regenerated at every start would invalidate every
// existing login on every restart.
func LoadOrCreateSecret(path string) ([]byte, error) {
	if path != "" {
		raw, err := os.ReadFile(path)
		if err == nil {
			secret, decodeErr := hex.DecodeString(strings.TrimSpace(string(raw)))
			if decodeErr != nil || len(secret) < 32 {
				return nil, fmt.Errorf("secure: %s does not contain a valid hex secret", path)
			}
			return secret, nil
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("secure: reading %s: %w", path, err)
		}
	}

	secret := make([]byte, 48)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("secure: generating secret: %w", err)
	}

	if path == "" {
		// No place to persist to; the caller gets a per-process secret.
		return secret, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("secure: creating directory for %s: %w", path, err)
	}
	// 0600: the secret is as sensitive as the password itself.
	if err := os.WriteFile(path, []byte(hex.EncodeToString(secret)), 0o600); err != nil {
		return nil, fmt.Errorf("secure: writing %s: %w", path, err)
	}
	return secret, nil
}
