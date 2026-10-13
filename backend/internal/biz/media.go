package biz

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	// Image decoders, registered for image.DecodeConfig so uploads can record
	// their intrinsic size.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"

	"github.com/bishop9910/bs-blog/backend/internal/data"
	"github.com/bishop9910/bs-blog/backend/internal/model"
)

// defaultMaxUploadBytes caps an upload when the config leaves it unset.
const defaultMaxUploadBytes = 10 << 20

// allowedImageTypes maps a MIME type to the extension the file is stored under.
// The stored name never comes from the client, so a file called "shell.php" can
// only ever land on disk as "<uuid>.png".
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// MediaUsecase stores and lists uploaded images.
type MediaUsecase struct {
	store     *data.Store
	uploadDir string
	urlPrefix string
	maxBytes  int64
}

// NewMediaUsecase wires the use case to a directory on disk.
func NewMediaUsecase(store *data.Store, uploadDir, urlPrefix string, maxBytes int64) *MediaUsecase {
	if maxBytes <= 0 {
		maxBytes = defaultMaxUploadBytes
	}
	if urlPrefix == "" {
		urlPrefix = "/uploads"
	}
	return &MediaUsecase{
		store:     store,
		uploadDir: uploadDir,
		urlPrefix: strings.TrimRight(urlPrefix, "/"),
		maxBytes:  maxBytes,
	}
}

// List returns one page of the library, newest first.
func (uc *MediaUsecase) List(ctx context.Context, page, pageSize int, keyword string) (*PaginatedMedia, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 24
	}
	if pageSize > 100 {
		pageSize = 100
	}

	items, total, err := uc.store.ListMedia(ctx, page, pageSize, keyword)
	if err != nil {
		return nil, internal("无法加载图片库", err)
	}

	out := make([]MediaDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toMediaDTO(item))
	}

	return &PaginatedMedia{Items: out, Total: total, Page: page, PageSize: pageSize}, nil
}

// Upload validates an image, writes it under <uploadDir>/<year>/<month>/ and
// records it. The returned URL is what the editor inserts into a document.
func (uc *MediaUsecase) Upload(ctx context.Context, header *multipart.FileHeader) (*MediaDTO, error) {
	if header == nil {
		return nil, errBadRequest("请选择要上传的图片")
	}
	if header.Size <= 0 {
		return nil, errBadRequest("文件内容为空")
	}
	if header.Size > uc.maxBytes {
		return nil, errBadRequest(fmt.Sprintf("图片不能超过 %d MB", uc.maxBytes>>20))
	}

	file, err := header.Open()
	if err != nil {
		return nil, internal("无法读取上传文件", err)
	}
	defer file.Close()

	// Sniff the actual bytes. Neither the declared Content-Type nor the file
	// extension is trustworthy, and the stored extension is taken from the
	// allow-list below rather than from either of them.
	head := make([]byte, 512)
	n, readErr := io.ReadFull(file, head)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return nil, internal("无法读取上传文件", readErr)
	}
	sniffed := normaliseMime(http.DetectContentType(head[:n]))

	ext, ok := allowedImageTypes[sniffed]
	if !ok {
		// DetectContentType has no entry for AVIF, and its WebP rule needs a
		// complete RIFF header. Falling back to the declared type is safe
		// because the value must still be one of the allowed image types —
		// and the extension written to disk comes from this same map.
		declared := normaliseMime(header.Header.Get("Content-Type"))
		ext, ok = allowedImageTypes[declared]
		if !ok {
			return nil, errBadRequest("只支持 JPG、PNG、GIF、WebP 格式的图片")
		}
		sniffed = declared
	}

	// Intrinsic size is advisory — the frontend uses it to reserve layout space
	// so images do not shift the page as they load.
	width, height := 0, 0
	if _, seekErr := file.Seek(0, io.SeekStart); seekErr == nil {
		if cfg, _, decodeErr := image.DecodeConfig(file); decodeErr == nil {
			width, height = cfg.Width, cfg.Height
		}
	}
	if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
		return nil, internal("无法读取上传文件", seekErr)
	}

	now := time.Now().UTC()
	relDir := filepath.Join(now.Format("2006"), now.Format("01"))
	dir := filepath.Join(uc.uploadDir, relDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, internal("无法创建上传目录", err)
	}

	storedName := uuid.NewString() + ext
	diskPath := filepath.Join(dir, storedName)

	dst, err := os.Create(diskPath)
	if err != nil {
		return nil, internal("无法写入图片", err)
	}
	written, copyErr := io.Copy(dst, file)
	closeErr := dst.Close()
	if copyErr != nil || closeErr != nil {
		// Leave nothing half-written behind.
		_ = os.Remove(diskPath)
		if copyErr != nil {
			return nil, internal("无法写入图片", copyErr)
		}
		return nil, internal("无法写入图片", closeErr)
	}

	asset := &model.MediaAsset{
		URL:        path.Join(uc.urlPrefix, filepath.ToSlash(relDir), storedName),
		Filename:   filepath.Base(header.Filename),
		StoredName: filepath.ToSlash(filepath.Join(relDir, storedName)),
		MimeType:   sniffed,
		Size:       written,
		Width:      width,
		Height:     height,
	}

	if err := uc.store.CreateMedia(ctx, asset); err != nil {
		// An orphaned file is worse than no file: the library is the index, so
		// remove the bytes if the index entry could not be written.
		_ = os.Remove(diskPath)
		return nil, internal("无法保存图片记录", err)
	}

	dto := toMediaDTO(*asset)
	return &dto, nil
}

// Delete removes an asset from the library and its bytes from disk.
func (uc *MediaUsecase) Delete(ctx context.Context, id int64) error {
	asset, err := uc.store.DeleteMedia(ctx, id)
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return errNotFoundError("图片不存在")
		}
		return internal("无法删除图片", err)
	}

	// The row is already gone. A failed unlink leaves an unreferenced file,
	// which is the safer of the two possible failures.
	diskPath := filepath.Join(uc.uploadDir, filepath.FromSlash(asset.StoredName))
	if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
		log.Errorf("biz: removing upload %s: %v", diskPath, err)
	}
	return nil
}

// normaliseMime lowercases a media type and drops any parameters.
func normaliseMime(value string) string {
	if i := strings.IndexByte(value, ';'); i >= 0 {
		value = value[:i]
	}
	return strings.ToLower(strings.TrimSpace(value))
}
