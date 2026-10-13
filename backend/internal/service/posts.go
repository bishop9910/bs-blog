package service

import (
	"net/http"
	"strings"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/bishop9910/bs-blog/backend/internal/biz"
)

/* -------------------------------------------------------------------------- */
/* Public                                                                     */
/* -------------------------------------------------------------------------- */

func (h *Handler) listPosts(ctx khttp.Context) error {
	result, err := h.posts.ListPublic(ctx, queryOf(ctx))
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, result)
}

func (h *Handler) getPost(ctx khttp.Context) error {
	slug := strings.TrimSpace(ctx.Vars().Get("slug"))
	if slug == "" {
		return kerrors.BadRequest("INVALID_ARGUMENT", "缺少文章标识")
	}

	detail, err := h.posts.GetPublished(ctx, slug)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, detail)
}

func (h *Handler) getArchive(ctx khttp.Context) error {
	archive, err := h.posts.Archive(ctx)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, archive)
}

func (h *Handler) listTags(ctx khttp.Context) error {
	tags, err := h.posts.Tags(ctx)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, tags)
}

/* -------------------------------------------------------------------------- */
/* Admin                                                                      */
/* -------------------------------------------------------------------------- */

func (h *Handler) listAdminPosts(ctx khttp.Context, _ *biz.CurrentUser) error {
	result, err := h.posts.ListAdmin(ctx, queryOf(ctx))
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, result)
}

func (h *Handler) getAdminPost(ctx khttp.Context, _ *biz.CurrentUser) error {
	id, err := pathID(ctx)
	if err != nil {
		return err
	}

	post, err := h.posts.GetAdmin(ctx, id)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, post)
}

func (h *Handler) createPost(ctx khttp.Context, _ *biz.CurrentUser) error {
	var input biz.PostInput
	if err := decodeBody(ctx, &input); err != nil {
		return err
	}

	// Creation always lands as a draft; publishing is a separate, deliberate
	// action so a half-written article can never go live by accident.
	created, err := h.posts.Create(ctx, input)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusCreated, created)
}

func (h *Handler) updatePost(ctx khttp.Context, _ *biz.CurrentUser) error {
	id, err := pathID(ctx)
	if err != nil {
		return err
	}

	var input biz.PostInput
	if err := decodeBody(ctx, &input); err != nil {
		return err
	}

	updated, err := h.posts.Update(ctx, id, input)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, updated)
}

func (h *Handler) deletePost(ctx khttp.Context, _ *biz.CurrentUser) error {
	id, err := pathID(ctx)
	if err != nil {
		return err
	}
	if err := h.posts.Delete(ctx, id); err != nil {
		return err
	}
	return noContent(ctx)
}

func (h *Handler) publishPost(ctx khttp.Context, _ *biz.CurrentUser) error {
	id, err := pathID(ctx)
	if err != nil {
		return err
	}

	post, err := h.posts.Publish(ctx, id)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, post)
}

func (h *Handler) unpublishPost(ctx khttp.Context, _ *biz.CurrentUser) error {
	id, err := pathID(ctx)
	if err != nil {
		return err
	}

	post, err := h.posts.Unpublish(ctx, id)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, post)
}

func (h *Handler) checkSlug(ctx khttp.Context, _ *biz.CurrentUser) error {
	q := ctx.Query()
	slug := strings.TrimSpace(q.Get("slug"))
	if slug == "" {
		return kerrors.BadRequest("INVALID_ARGUMENT", "缺少 slug 参数")
	}

	result, err := h.posts.CheckSlug(ctx, slug, int64(intArg(q.Get("excludeId"), 0)))
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, result)
}
