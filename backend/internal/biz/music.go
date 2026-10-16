package biz

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strings"
	"time"

	"github.com/bishop9910/bs-blog/backend/internal/data"
)

/*
The music library is a directory of audio files, not a database table, so this
layer is thin on purpose: the index and the types it produces live in the data
package, next to the scanner that fills them in, and are re-exported here.

The aliases are not decoration. biz already imports data for the store, so data
cannot import biz back to share one set of structs — the dependency would be a
cycle — and both layers need to agree on exactly what a track is.
*/
type (
	MusicTrack   = data.MusicTrack
	MusicAlbum   = data.MusicAlbum
	MusicArtist  = data.MusicArtist
	MusicCover   = data.MusicCover
	MusicLibrary = data.MusicLibrary
)

// MusicRepo is the music library as the use case needs it: a snapshot to list,
// lookups for the three media routes, and the two ways of changing what is on
// disk.
type MusicRepo interface {
	// Library returns the current index snapshot.
	Library(ctx context.Context) (*MusicLibrary, error)
	// Track resolves one id, or data.ErrNotFound.
	Track(ctx context.Context, id string) (*MusicTrack, error)
	// OpenStream opens a track's audio file for http.ServeContent.
	OpenStream(ctx context.Context, id string) (io.ReadSeekCloser, *MusicTrack, error)
	// Cover reads a track's embedded or sidecar cover art.
	Cover(ctx context.Context, id string) (*MusicCover, error)
	// Lyrics returns a track's LRC text, or "" when it has none.
	Lyrics(ctx context.Context, id string) (string, error)
	// Rescan rebuilds the index from disk.
	Rescan(ctx context.Context) (*MusicLibrary, error)
	// Save writes an uploaded audio file into the library and indexes it.
	Save(ctx context.Context, header *multipart.FileHeader) (*MusicTrack, error)
	// Delete removes a track's file and its sidecars from disk.
	Delete(ctx context.Context, id string) error
}

// MusicUploadLimits are the size caps the library works with: one for an audio
// upload arriving through the admin API, and two for the sidecar files that are
// read straight off disk. All three come from music.* in config.yaml, because a
// deployment may hold lossless rips or scans of very different sizes.
type MusicUploadLimits struct {
	MaxUploadBytes int64
	MaxCoverBytes  int64
	MaxLyricsBytes int64
}

// DefaultMusicUploadLimits caps audio at 32 MiB, a cover at 5 MiB and a lyrics
// file at 1 MiB when config.yaml leaves a value unset.
func DefaultMusicUploadLimits() MusicUploadLimits {
	return MusicUploadLimits{
		MaxUploadBytes: 32 << 20,
		MaxCoverBytes:  5 << 20,
		MaxLyricsBytes: 1 << 20,
	}
}

// The stream and cover URLs are paths on the server that served the page. That
// is what lets an <audio> element and an <img> tag reach them without a token,
// a CORS preflight or a redirect — the reader's browser is the same origin. The
// two prefixes mirror service.APIPrefix and are part of the JSON contract, so
// the frontend never has to build them itself.
const (
	musicStreamPrefix = "/api/v1/music/stream/"
	musicCoverPrefix  = "/api/v1/music/cover/"
)

// MusicTrackDTO is one row of the library listing. It deliberately stops at what
// the page needs: the file's absolute path, its size and its mime type stay on
// the server, and the frontend reaches the bytes through id-based URLs.
type MusicTrackDTO struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	AlbumArtist string `json:"albumArtist"`
	Year        int    `json:"year"`
	TrackNo     int    `json:"trackNo"`
	DurationMs  int64  `json:"durationMs"`
	HasCover    bool   `json:"hasCover"`
	HasLyrics   bool   `json:"hasLyrics"`
	StreamURL   string `json:"streamUrl"`
	CoverURL    string `json:"coverUrl"`
}

// MusicAlbumDTO is one album group in the listing.
type MusicAlbumDTO struct {
	Name       string `json:"name"`
	Artist     string `json:"artist"`
	Year       int    `json:"year"`
	TrackCount int    `json:"trackCount"`
	CoverURL   string `json:"coverUrl"`
}

// MusicArtistDTO is one artist group in the listing.
type MusicArtistDTO struct {
	Name       string `json:"name"`
	TrackCount int    `json:"trackCount"`
}

// MusicLibraryDTO is the public listing: everything the player page needs in one
// response, because the page cannot render until it has the tracks anyway.
type MusicLibraryDTO struct {
	Tracks     []MusicTrackDTO  `json:"tracks"`
	Albums     []MusicAlbumDTO  `json:"albums"`
	Artists    []MusicArtistDTO `json:"artists"`
	ScannedAt  string           `json:"scannedAt"`
	TrackCount int              `json:"trackCount"`
}

// MusicAdminDTO describes the library itself for the settings page: where it
// lives on disk, whether that place exists, and the full track list.
type MusicAdminDTO struct {
	Dir         string          `json:"dir"`
	Exists      bool            `json:"exists"`
	Extensions  []string        `json:"extensions"`
	Watch       bool            `json:"watch"`
	TrackCount  int             `json:"trackCount"`
	AlbumCount  int             `json:"albumCount"`
	ArtistCount int             `json:"artistCount"`
	ScannedAt   string          `json:"scannedAt"`
	Tracks      []MusicTrackDTO `json:"tracks"`
}

// MusicLyricsDTO is one track's lyrics. An empty lyrics string with
// hasLyrics:false is the normal answer for an instrumental, not an error.
type MusicLyricsDTO struct {
	Lyrics    string `json:"lyrics"`
	HasLyrics bool   `json:"hasLyrics"`
}

// MusicUsecase lists the library and serves one track's bytes, cover and lyrics.
type MusicUsecase struct {
	repo   MusicRepo
	limits MusicUploadLimits
}

// NewMusicUsecase wires the use case to the on-disk library. Any cap left at
// zero falls back to the default, so an unset field cannot mean "no limit".
func NewMusicUsecase(repo MusicRepo, limits MusicUploadLimits) *MusicUsecase {
	defaults := DefaultMusicUploadLimits()
	if limits.MaxUploadBytes <= 0 {
		limits.MaxUploadBytes = defaults.MaxUploadBytes
	}
	if limits.MaxCoverBytes <= 0 {
		limits.MaxCoverBytes = defaults.MaxCoverBytes
	}
	if limits.MaxLyricsBytes <= 0 {
		limits.MaxLyricsBytes = defaults.MaxLyricsBytes
	}
	return &MusicUsecase{repo: repo, limits: limits}
}

// Limits reports the size policy the admin page should quote.
func (uc *MusicUsecase) Limits() MusicLimitsDTO {
	return MusicLimitsDTO{
		MaxUploadBytes: uc.limits.MaxUploadBytes,
		MaxCoverBytes:  uc.limits.MaxCoverBytes,
		MaxLyricsBytes: uc.limits.MaxLyricsBytes,
	}
}

// Library returns the public listing.
func (uc *MusicUsecase) Library(ctx context.Context) (*MusicLibraryDTO, error) {
	library, err := uc.repo.Library(ctx)
	if err != nil {
		return nil, internal("无法加载音乐库", err)
	}
	return toMusicLibraryDTO(library), nil
}

// Admin returns the same library with the settings the player page does not need.
func (uc *MusicUsecase) Admin(ctx context.Context) (*MusicAdminDTO, error) {
	library, err := uc.repo.Library(ctx)
	if err != nil {
		return nil, internal("无法加载音乐库", err)
	}
	return toMusicAdminDTO(library), nil
}

// Rescan rebuilds the index from disk and returns the fresh listing. Files that
// disappeared are dropped, new ones are picked up.
func (uc *MusicUsecase) Rescan(ctx context.Context) (*MusicAdminDTO, error) {
	library, err := uc.repo.Rescan(ctx)
	if err != nil {
		return nil, internal("无法重新扫描音乐库", err)
	}
	return toMusicAdminDTO(library), nil
}

// Lyrics returns one track's LRC text.
func (uc *MusicUsecase) Lyrics(ctx context.Context, id string) (*MusicLyricsDTO, error) {
	text, err := uc.repo.Lyrics(ctx, id)
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return nil, errNotFoundError("曲目不存在")
		}
		return nil, internal("无法读取歌词", err)
	}
	return &MusicLyricsDTO{Lyrics: text, HasLyrics: strings.TrimSpace(text) != ""}, nil
}

// OpenStream opens the audio bytes and the track they belong to, so the handler
// can name the file and set the right Content-Type for a Range request.
func (uc *MusicUsecase) OpenStream(ctx context.Context, id string) (io.ReadSeekCloser, *MusicTrack, error) {
	file, track, err := uc.repo.OpenStream(ctx, id)
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return nil, nil, errNotFoundError("曲目不存在")
		}
		return nil, nil, internal("无法读取音频文件", err)
	}
	return file, track, nil
}

// OpenCover returns a track's cover art.
func (uc *MusicUsecase) OpenCover(ctx context.Context, id string) (*MusicCover, error) {
	cover, err := uc.repo.Cover(ctx, id)
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return nil, errNotFoundError("封面不存在")
		}
		return nil, internal("无法读取封面", err)
	}
	return cover, nil
}

// Save validates an uploaded audio file and hands it to the library. The size
// ceiling is checked here rather than there because the value comes from the
// configuration and the message is user-facing.
func (uc *MusicUsecase) Save(ctx context.Context, header *multipart.FileHeader) (*MusicTrackDTO, error) {
	if header == nil {
		return nil, errBadRequest("请选择要上传的音频文件")
	}
	if header.Size <= 0 {
		return nil, errBadRequest("文件内容为空")
	}
	if header.Size > uc.limits.MaxUploadBytes {
		return nil, errBadRequest(fmt.Sprintf("音频不能超过 %d MB", uc.limits.MaxUploadBytes>>20))
	}

	track, err := uc.repo.Save(ctx, header)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrMusicFormat):
			return nil, errBadRequest("只支持 MP3、FLAC、M4A、OGG、OPUS、WAV 格式的音频")
		case errors.Is(err, data.ErrMusicDisabled):
			return nil, errBadRequest("音乐库未启用，请先配置 music.dir")
		}
		return nil, internal("无法保存音频文件", err)
	}

	dto := toMusicTrackDTO(track)
	return &dto, nil
}

// Delete removes a track from the library and its file from disk.
func (uc *MusicUsecase) Delete(ctx context.Context, id string) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return errNotFoundError("曲目不存在")
		}
		return internal("无法删除音频文件", err)
	}
	return nil
}

// toMusicLibraryDTO shapes the snapshot for the player page.
func toMusicLibraryDTO(library *MusicLibrary) *MusicLibraryDTO {
	dto := &MusicLibraryDTO{
		Tracks:     make([]MusicTrackDTO, 0, len(library.Tracks)),
		Albums:     make([]MusicAlbumDTO, 0, len(library.Albums)),
		Artists:    make([]MusicArtistDTO, 0, len(library.Artists)),
		ScannedAt:  library.ScannedAt.UTC().Format(time.RFC3339),
		TrackCount: library.TrackCount(),
	}
	for _, track := range library.Tracks {
		dto.Tracks = append(dto.Tracks, toMusicTrackDTO(track))
	}
	for _, album := range library.Albums {
		dto.Albums = append(dto.Albums, MusicAlbumDTO{
			Name:       album.Name,
			Artist:     album.Artist,
			Year:       album.Year,
			TrackCount: album.TrackCount,
			CoverURL:   musicCoverPrefix + album.CoverID,
		})
	}
	for _, artist := range library.Artists {
		dto.Artists = append(dto.Artists, MusicArtistDTO{Name: artist.Name, TrackCount: artist.TrackCount})
	}
	return dto
}

// toMusicAdminDTO shapes the snapshot for the settings page.
func toMusicAdminDTO(library *MusicLibrary) *MusicAdminDTO {
	dto := &MusicAdminDTO{
		Dir:         library.Dir,
		Exists:      library.Exists,
		Extensions:  library.Extensions,
		Watch:       library.Watch,
		TrackCount:  library.TrackCount(),
		AlbumCount:  len(library.Albums),
		ArtistCount: len(library.Artists),
		ScannedAt:   library.ScannedAt.UTC().Format(time.RFC3339),
		Tracks:      make([]MusicTrackDTO, 0, len(library.Tracks)),
	}
	for _, track := range library.Tracks {
		dto.Tracks = append(dto.Tracks, toMusicTrackDTO(track))
	}
	return dto
}

// toMusicTrackDTO shapes one track. Missing tags become the same placeholders
// the album and artist groups use, so a track listed under "未知艺术家" also
// reads that way on its own.
func toMusicTrackDTO(track *MusicTrack) MusicTrackDTO {
	artist := musicDisplayText(track.Artist, "未知艺术家")
	return MusicTrackDTO{
		ID:          track.ID,
		Title:       track.Title,
		Artist:      artist,
		Album:       musicDisplayText(track.Album, "未知专辑"),
		AlbumArtist: musicDisplayText(track.AlbumArtist, artist),
		Year:        track.Year,
		TrackNo:     track.TrackNo,
		DurationMs:  track.DurationMs,
		HasCover:    track.HasCover,
		HasLyrics:   track.HasLyrics,
		StreamURL:   musicStreamPrefix + track.ID,
		CoverURL:    musicCoverPrefix + track.ID,
	}
}

// musicDisplayText substitutes a fallback for an empty tag value.
func musicDisplayText(value, fallback string) string {
	if value = strings.TrimSpace(value); value == "" {
		return fallback
	}
	return value
}
