package biz

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"

	"github.com/bishop9910/bs-blog/backend/internal/data"
	"github.com/bishop9910/bs-blog/backend/internal/model"
)

// defaultResourceMaxUploadBytes caps an upload when the config leaves it unset.
const defaultResourceMaxUploadBytes = 32 << 20

// Field caps, matched to the column sizes in internal/model.
const (
	maxResourceTitleLength       = 160
	maxResourceDescriptionLength = 2000
	maxResourceCoverURLLength    = 512
	maxResourceFilenameLength    = 200
)

// defaultResourceExtensions is the suffix allow-list used when
// resources.extensions in config.yaml is empty.
//
// Archives and installers are the point of the feature: a finished project is a
// zip, an apk or an exe. Documents and single source files are listed so a file
// can be shared without packing it first.
//
// HTML and SVG are deliberately absent. They are the two formats a browser would
// execute if a download ever ended up rendered inline, and zipping is a
// perfectly good answer for either. Nothing here is trusted anyway — the
// download route always answers application/octet-stream with
// Content-Disposition: attachment — but the list should not invite the case.
var defaultResourceExtensions = []string{
	// Archives and installers.
	"zip", "7z", "rar", "tar", "gz", "tgz", "bz2", "xz", "zst",
	"apk", "exe", "msi", "dmg", "deb", "rpm", "jar", "whl", "nupkg",
	// Documents and data.
	"pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "md", "txt", "csv",
	"json", "yaml", "yml", "sql",
	// Source and scripts.
	"ps1", "sh", "py", "js", "ts", "go", "java", "c", "h", "cpp", "cs", "rs",
	"php", "rb",
}

// DefaultResourceExtensions returns a copy of the built-in allow-list.
func DefaultResourceExtensions() []string {
	return append([]string(nil), defaultResourceExtensions...)
}

// ResourceUsecase stores and serves the downloadable files behind the resources
// page.
//
// The bytes live on disk and the row is only an index, so the use case — not the
// data layer — owns the directory, exactly like the media library. The one
// difference is that nothing here is ever served by the static file handler:
// every byte leaves through Download, which is what keeps a download an
// attachment rather than a page.
type ResourceUsecase struct {
	store      *data.Store
	dir        string
	maxBytes   int64
	extensions map[string]struct{}
	list       []string
}

// NewResourceUsecase wires the use case to a directory on disk.
//
// An empty extension list falls back to DefaultResourceExtensions, so a
// deployment that omits the key still accepts the formats the code documents.
func NewResourceUsecase(store *data.Store, dir string, maxBytes int64, extensions []string) *ResourceUsecase {
	if maxBytes <= 0 {
		maxBytes = defaultResourceMaxUploadBytes
	}
	if len(extensions) == 0 {
		extensions = defaultResourceExtensions
	}

	allowed := make(map[string]struct{}, len(extensions))
	list := make([]string, 0, len(extensions))
	for _, raw := range extensions {
		ext := strings.ToLower(strings.Trim(strings.TrimSpace(raw), "."))
		if ext == "" {
			continue
		}
		if _, seen := allowed[ext]; seen {
			continue
		}
		allowed[ext] = struct{}{}
		list = append(list, ext)
	}
	sort.Strings(list)

	return &ResourceUsecase{
		store:      store,
		dir:        dir,
		maxBytes:   maxBytes,
		extensions: allowed,
		list:       list,
	}
}

// Limits reports the download policy. The frontend quotes these values instead
// of hard-coding a copy, so the drop zone and the server can never disagree.
func (uc *ResourceUsecase) Limits() ResourceLimitsDTO {
	return ResourceLimitsDTO{
		MaxUploadBytes:    uc.maxBytes,
		AllowedExtensions: append([]string(nil), uc.list...),
	}
}

// List returns one page of the collection, newest first.
//
// visibleOnly is true for the public page and false for the admin one: the
// owner has to see hidden files in order to unhide them.
func (uc *ResourceUsecase) List(
	ctx context.Context,
	page, pageSize int,
	keyword string,
	visibleOnly bool,
) (*PaginatedResources, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 24
	}
	if pageSize > 100 {
		pageSize = 100
	}

	items, total, err := uc.store.ListResources(ctx, page, pageSize, keyword, visibleOnly)
	if err != nil {
		return nil, internal("无法加载资源列表", err)
	}

	out := make([]ResourceDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toResourceDTO(item))
	}

	return &PaginatedResources{Items: out, Total: total, Page: page, PageSize: pageSize}, nil
}

// Upload validates a file, writes it under <dir>/<year>/<month>/ and records it.
func (uc *ResourceUsecase) Upload(
	ctx context.Context,
	header *multipart.FileHeader,
	in ResourceInput,
) (*ResourceDTO, error) {
	if header == nil {
		return nil, errBadRequest("请选择要上传的文件")
	}
	if header.Size <= 0 {
		return nil, errBadRequest("文件内容为空")
	}
	if header.Size > uc.maxBytes {
		return nil, errBadRequest(fmt.Sprintf("文件不能超过 %d MB", uc.maxBytes>>20))
	}

	filename, err := normaliseUploadFilename(header.Filename)
	if err != nil {
		return nil, err
	}
	ext, ok := uc.extensionOf(filename)
	if !ok {
		if filepath.Ext(filename) == "" {
			return nil, errBadRequest("文件没有扩展名，无法判断类型")
		}
		return nil, errBadRequest(fmt.Sprintf("不支持 .%s 文件，请改用压缩包或其他支持的格式", extensionOf(filename)))
	}

	// A title is required on the card, but demanding one before the upload
	// would be a pointless round trip: the filename is a serviceable default and
	// the admin list can rename it afterwards.
	title := strings.TrimSpace(coalesce(in.Title, ""))
	if title == "" {
		title = strings.TrimSuffix(filename, filepath.Ext(filename))
	}
	title, description, coverURL, err := resourceMeta(title, coalesce(in.Description, ""), coalesce(in.CoverURL, ""))
	if err != nil {
		return nil, err
	}

	src, err := header.Open()
	if err != nil {
		return nil, internal("无法读取上传文件", err)
	}
	defer src.Close()

	now := time.Now().UTC()
	relDir := path.Join(now.Format("2006"), now.Format("01"))
	diskDir := filepath.Join(uc.dir, filepath.FromSlash(relDir))
	if err := os.MkdirAll(diskDir, 0o755); err != nil {
		return nil, internal("无法创建资源目录", err)
	}

	// The stored name never comes from the client: the UUID is what makes two
	// uploads called "project.zip" coexist, and the allow-listed extension is
	// the only part of the name that survives.
	storedName := uuid.NewString() + "." + ext
	diskPath := filepath.Join(diskDir, storedName)

	dst, err := os.OpenFile(diskPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, internal("无法写入文件", err)
	}
	written, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil || closeErr != nil {
		// Leave nothing half-written behind.
		_ = os.Remove(diskPath)
		if copyErr != nil {
			return nil, internal("无法写入文件", copyErr)
		}
		return nil, internal("无法写入文件", closeErr)
	}

	file := &model.ResourceFile{
		Title:       title,
		Description: description,
		CoverURL:    coverURL,
		Filename:    filename,
		StoredName:  path.Join(relDir, storedName),
		Extension:   ext,
		Size:        written,
		Visible:     in.Visible == nil || *in.Visible,
	}

	if err := uc.store.CreateResource(ctx, file); err != nil {
		// An unreferenced file would be invisible in the admin list and
		// impossible to delete from the UI, so the bytes go with the row.
		_ = os.Remove(diskPath)
		return nil, internal("无法保存资源记录", err)
	}

	dto := toResourceDTO(*file)
	return &dto, nil
}

// Update rewrites the metadata of an existing file. The bytes and the download
// link stay the same.
func (uc *ResourceUsecase) Update(ctx context.Context, id int64, in ResourceInput) (*ResourceDTO, error) {
	file, err := uc.store.GetResource(ctx, id)
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return nil, errNotFoundError("资源不存在")
		}
		return nil, internal("无法加载资源", err)
	}

	title, description, coverURL, err := resourceMeta(
		coalesce(in.Title, file.Title),
		coalesce(in.Description, file.Description),
		coalesce(in.CoverURL, file.CoverURL),
	)
	if err != nil {
		return nil, err
	}

	file.Title = title
	file.Description = description
	file.CoverURL = coverURL
	if in.Visible != nil {
		file.Visible = *in.Visible
	}

	if err := uc.store.UpdateResource(ctx, file); err != nil {
		return nil, internal("无法更新资源", err)
	}

	dto := toResourceDTO(*file)
	return &dto, nil
}

// Delete removes the row and the bytes on disk.
func (uc *ResourceUsecase) Delete(ctx context.Context, id int64) error {
	file, err := uc.store.DeleteResource(ctx, id)
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return errNotFoundError("资源不存在")
		}
		return internal("无法删除资源", err)
	}

	// The row is already gone. A failed unlink leaves an unreferenced file,
	// which is the safer of the two possible failures.
	if err := os.Remove(uc.diskPath(*file)); err != nil && !os.IsNotExist(err) {
		log.Errorf("biz: removing resource file %s: %v", file.StoredName, err)
	}
	return nil
}

// ResourceDownload is an opened file, ready to be handed to http.ServeContent.
type ResourceDownload struct {
	File     *os.File
	Filename string
	Size     int64
	ModTime  time.Time
}

// Download opens a visible file for streaming and, for a GET, counts the hit.
//
// A hidden file answers "not found" rather than "forbidden": the visitor has no
// business knowing that a link exists but is switched off.
func (uc *ResourceUsecase) Download(ctx context.Context, id int64, count bool) (*ResourceDownload, error) {
	file, err := uc.store.GetResource(ctx, id)
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return nil, errNotFoundError("资源不存在")
		}
		return nil, internal("无法加载资源", err)
	}
	if !file.Visible {
		return nil, errNotFoundError("资源不存在")
	}

	handle, err := os.Open(uc.diskPath(*file))
	if err != nil {
		if os.IsNotExist(err) {
			log.Errorf("biz: resource %d is indexed but missing on disk: %v", id, err)
			return nil, errNotFoundError("文件已丢失")
		}
		return nil, internal("无法读取文件", err)
	}

	info, statErr := handle.Stat()
	if statErr != nil {
		_ = handle.Close()
		return nil, internal("无法读取文件", statErr)
	}

	if count {
		// Counting here rather than after the write means an aborted download
		// still counts. The alternative is a counter that only moves when a
		// multi-megabyte transfer completes, which reads as "nothing is being
		// downloaded" to the owner watching the page.
		if err := uc.store.IncrementResourceDownloads(ctx, id); err != nil {
			log.Errorf("biz: counting download for resource %d: %v", id, err)
		}
	}

	return &ResourceDownload{
		File:     handle,
		Filename: file.Filename,
		Size:     file.Size,
		ModTime:  info.ModTime(),
	}, nil
}

// diskPath resolves a row to an absolute path.
//
// StoredName is generated by Upload as "yyyy/mm/<uuid>.<ext>", but it still goes
// through filepath.FromSlash and a base-name check: if a future write path ever
// let a ".." into the column, this is where it stops being a traversal.
func (uc *ResourceUsecase) diskPath(file model.ResourceFile) string {
	clean := path.Clean("/" + file.StoredName)
	return filepath.Join(uc.dir, filepath.FromSlash(strings.TrimPrefix(clean, "/")))
}

// extensionOf returns the lower-case, dotless suffix of a filename.
func (uc *ResourceUsecase) extensionOf(name string) (string, bool) {
	ext := extensionOf(name)
	if ext == "" {
		return "", false
	}
	_, ok := uc.extensions[ext]
	return ext, ok
}

func extensionOf(name string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
}

// normaliseUploadFilename strips any directory part and rejects names that
// cannot be displayed or saved.
//
// Browsers send a basename, but a hand-rolled client need not: the download
// header is built from this value, so a separator or a control character must
// not get through.
func normaliseUploadFilename(raw string) (string, error) {
	name := strings.ReplaceAll(raw, "\\", "/")
	name = strings.TrimSpace(path.Base(name))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)

	if name == "" || name == "." || name == "/" {
		return "", errBadRequest("文件名无效")
	}
	if utf8.RuneCountInString(name) > maxResourceFilenameLength {
		return "", errBadRequest(fmt.Sprintf("文件名不能超过 %d 个字符", maxResourceFilenameLength))
	}
	return name, nil
}

// resourceMeta trims and validates the editable metadata, returning the values
// that should be stored.
func resourceMeta(title, description, coverURL string) (string, string, string, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	coverURL = strings.TrimSpace(coverURL)

	if title == "" {
		return "", "", "", errBadRequest("请填写资源标题")
	}
	if utf8.RuneCountInString(title) > maxResourceTitleLength {
		return "", "", "", errBadRequest(fmt.Sprintf("标题不能超过 %d 个字符", maxResourceTitleLength))
	}
	if utf8.RuneCountInString(description) > maxResourceDescriptionLength {
		return "", "", "", errBadRequest(fmt.Sprintf("简介不能超过 %d 个字符", maxResourceDescriptionLength))
	}
	if utf8.RuneCountInString(coverURL) > maxResourceCoverURLLength {
		return "", "", "", errBadRequest("封面地址过长")
	}
	if coverURL != "" && !isSafeImageTarget(coverURL) {
		return "", "", "", errBadRequest("封面地址无效，只支持站内路径或 http(s) 链接")
	}
	return title, description, coverURL, nil
}

// coalesce resolves an optional input field: an absent key falls back to what is
// already stored, an empty string is a deliberate clear.
func coalesce(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}
