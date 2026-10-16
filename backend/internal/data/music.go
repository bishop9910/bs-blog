// Music library: the blog serves the owner's own audio files straight off disk.
//
// Nothing about a track is stored in SQLite — the directory *is* the library —
// so the index below is a snapshot rebuilt from disk and swapped in whole. That
// keeps a music folder that is rearranged, copied or synced by other tools
// correct without any of them having to tell the blog about it.
package data

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-kratos/kratos/v2/log"

	"github.com/bishop9910/bs-blog/backend/internal/data/audio"
)

// MusicTrack is one indexed audio file. Everything the listing and the three
// media routes need is resolved once, at scan time; the bytes of the audio, the
// cover and the lyrics are read again on demand so the index stays small.
type MusicTrack struct {
	ID          string
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Year        int
	TrackNo     int
	DiscNo      int
	DurationMs  int64
	HasCover    bool
	HasLyrics   bool
	Size        int64
	ModTime     time.Time
	// Path is the absolute location on disk. It never leaves the server: the
	// listing hands the frontend an id, and every lookup goes through the
	// index, so a request cannot name a file of its own choosing.
	Path     string
	MIMEType string
	// CoverMIME is the media type of the cover the track would serve, or "" when
	// it has none. The image itself is not kept here.
	CoverMIME string
}

// MusicAlbum aggregates the tracks that share an album name and album artist.
type MusicAlbum struct {
	Name       string
	Artist     string
	Year       int
	TrackCount int
	// CoverID is the track whose cover stands in for the album. When no track of
	// the album carries one it still names a track, because the frontend renders
	// the URL unconditionally and falls back to a placeholder on a 404.
	CoverID string
}

// MusicArtist aggregates the tracks credited to one artist.
type MusicArtist struct {
	Name       string
	TrackCount int
}

// MusicCover is one track's cover art, already read from disk.
type MusicCover struct {
	MIME string
	Data []byte
}

// MusicLibrary is one immutable snapshot of the index. A reader may hold on to
// it after releasing the lock: a scan builds a new snapshot instead of mutating
// this one.
type MusicLibrary struct {
	Dir        string
	Exists     bool
	Extensions []string
	Watch      bool
	Tracks     []*MusicTrack
	Albums     []*MusicAlbum
	Artists    []*MusicArtist
	ScannedAt  time.Time

	byID map[string]*MusicTrack
}

// TrackCount is the number of indexed tracks.
func (l *MusicLibrary) TrackCount() int { return len(l.Tracks) }

// Track finds one track by id.
func (l *MusicLibrary) Track(id string) *MusicTrack { return l.byID[id] }

// defaultMusicExtensions is what the library indexes when music.extensions is
// left empty.
var defaultMusicExtensions = []string{"mp3", "flac", "m4a", "m4b", "ogg", "opus", "wav"}

// musicWatchDebounce collapses the burst of events one file copy produces —
// create, then a write per buffer — into a single rescan once the dust settles.
const musicWatchDebounce = 2 * time.Second

// MusicOptions configures the library.
//
// The three size caps arrive already resolved from music.* in config.yaml. A
// non-positive value falls back to the defaults below, so a hand-written config
// or a zero value cannot end up reading an unbounded sidecar file.
type MusicOptions struct {
	Dir            string
	Extensions     []string
	Watch          bool
	MaxUploadBytes int64
	MaxCoverBytes  int64
	MaxLyricsBytes int64
}

// MusicRepo answers questions about a directory of audio files.
type MusicRepo struct {
	root        string
	exts        map[string]struct{}
	extList     []string
	watch       bool
	maxBytes    int64
	coverBytes  int64
	lyricsBytes int64
	logger      *log.Helper

	mu      sync.RWMutex
	library *MusicLibrary
}

// NewMusicRepo indexes the music directory. An empty dir disables the library
// entirely; a dir that does not exist yet is a supported state too — the index
// is simply empty and the directory is created by the first upload.
func NewMusicRepo(opts MusicOptions, logger *log.Helper) *MusicRepo {
	if logger == nil {
		logger = log.NewHelper(log.DefaultLogger)
	}

	exts := make(map[string]struct{})
	extList := make([]string, 0, len(opts.Extensions))
	for _, ext := range opts.Extensions {
		ext = strings.ToLower(strings.Trim(strings.TrimSpace(ext), "."))
		if ext == "" {
			continue
		}
		if _, seen := exts[ext]; seen {
			continue
		}
		exts[ext] = struct{}{}
		extList = append(extList, ext)
	}
	if len(extList) == 0 {
		for _, ext := range defaultMusicExtensions {
			exts[ext] = struct{}{}
			extList = append(extList, ext)
		}
	}
	sort.Strings(extList)

	r := &MusicRepo{
		root:        strings.TrimSpace(opts.Dir),
		exts:        exts,
		extList:     extList,
		watch:       opts.Watch,
		maxBytes:    positiveOr(opts.MaxUploadBytes, 32<<20),
		coverBytes:  positiveOr(opts.MaxCoverBytes, 5<<20),
		lyricsBytes: positiveOr(opts.MaxLyricsBytes, 1<<20),
		logger:      logger,
	}

	if r.root == "" {
		r.logger.Warn("data: music.dir is empty; the music library is disabled")
		r.store(r.newLibrary(nil, false))
		return r
	}

	// Index before the server starts listening, so the first request sees the
	// real library rather than an empty one that fills in a moment later.
	r.store(r.scan())

	if opts.Watch {
		r.startWatcher()
	}
	return r
}

// positiveOr returns value when it is set, and the fallback otherwise.
func positiveOr(value, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}

// scan walks the music directory and returns a fresh snapshot. It never fails
// the caller: an unreadable file or subdirectory costs only its own tracks, and
// a missing root is reported through Exists rather than as an error.
func (r *MusicRepo) scan() *MusicLibrary {
	if r.root == "" {
		return r.newLibrary(nil, false)
	}

	st, err := os.Stat(r.root)
	if err != nil {
		if !os.IsNotExist(err) {
			r.logger.Warnf("data: music directory unavailable: %v", err)
		}
		return r.newLibrary(nil, false)
	}
	if !st.IsDir() {
		r.logger.Warnf("data: music.dir is not a directory: %s", r.root)
		return r.newLibrary(nil, false)
	}

	tracks := make([]*MusicTrack, 0, 64)
	walkErr := filepath.WalkDir(r.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			r.logger.Warnf("data: skipping %s: %v", path, err)
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !r.isAudio(path) {
			return nil
		}
		track, indexErr := r.indexFile(path)
		if indexErr != nil {
			// One broken file must not blank the library, so it is skipped and
			// noted; the next scan will try it again.
			r.logger.Warnf("data: skipping audio file %s: %v", path, indexErr)
			return nil
		}
		tracks = append(tracks, track)
		return nil
	})
	if walkErr != nil {
		r.logger.Warnf("data: walking the music directory: %v", walkErr)
	}
	return r.newLibrary(tracks, true)
}

// indexFile reads one audio file and its sidecars into a MusicTrack. It is the
// only place that touches the disk during a scan.
func (r *MusicRepo) indexFile(path string) (*MusicTrack, error) {
	info, err := audio.Read(path)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	rel, err := filepath.Rel(r.root, path)
	if err != nil {
		rel = filepath.Base(path)
	}

	track := &MusicTrack{
		ID:          musicID(filepath.ToSlash(rel)),
		Title:       strings.TrimSpace(info.Title),
		Artist:      strings.TrimSpace(info.Artist),
		Album:       strings.TrimSpace(info.Album),
		AlbumArtist: strings.TrimSpace(info.AlbumArtist),
		Year:        info.Year,
		TrackNo:     info.TrackNo,
		DiscNo:      info.DiscNo,
		DurationMs:  info.DurationMs,
		Size:        st.Size(),
		ModTime:     st.ModTime(),
		Path:        path,
		MIMEType:    info.MIMEType,
		CoverMIME:   info.CoverMIME,
	}

	// audio.Read substitutes the bare file name when the tags carry no title.
	// That is a placeholder, not metadata, so it is cleared here and the name is
	// parsed properly below.
	if track.Title == fileNameTitle(path) {
		track.Title = ""
	}
	applyFileNameFallback(track)

	sc, sidecarErr := readSidecar(sidecarPath(path))
	switch {
	case sidecarErr == nil:
		r.applySidecar(track, sc)
	case !os.IsNotExist(sidecarErr):
		r.logger.Warnf("data: ignoring sidecar of %s: %v", path, sidecarErr)
	}

	track.HasLyrics = r.hasLyrics(path, sc)
	track.HasCover = track.CoverMIME != ""
	return track, nil
}

// applySidecar lets a JSON sidecar override the tags of the file it belongs to.
// A malformed sidecar is reported by the caller and otherwise skipped: the audio
// file is still perfectly playable, and dropping it from the library over a
// stray comma would be worse than losing the overrides.
func (r *MusicRepo) applySidecar(track *MusicTrack, sc *trackSidecar) {
	if sc == nil {
		return
	}
	if v := strings.TrimSpace(sc.Title); v != "" {
		track.Title = v
	}
	if v := strings.TrimSpace(sc.Artist); v != "" {
		track.Artist = v
	}
	if v := strings.TrimSpace(sc.Album); v != "" {
		track.Album = v
	}
	if v := strings.TrimSpace(sc.AlbumArtist); v != "" {
		track.AlbumArtist = v
	}
	if sc.Year != 0 {
		track.Year = sc.Year
	}
	if v := strings.TrimSpace(sc.Cover); v != "" {
		mime, err := r.probeCover(filepath.Dir(track.Path), v)
		if err != nil {
			r.logger.Warnf("data: ignoring sidecar cover of %s: %v", track.Path, err)
		} else {
			track.CoverMIME = mime
		}
	}
}

// newLibrary derives the album and artist views and packages the snapshot the
// readers will see.
func (r *MusicRepo) newLibrary(tracks []*MusicTrack, exists bool) *MusicLibrary {
	sortTracks(tracks)

	byID := make(map[string]*MusicTrack, len(tracks))
	for _, t := range tracks {
		byID[t.ID] = t
	}
	albums, artists := buildAlbumViews(tracks)

	return &MusicLibrary{
		Dir:        r.root,
		Exists:     exists,
		Extensions: append([]string(nil), r.extList...),
		Watch:      r.watch,
		Tracks:     tracks,
		Albums:     albums,
		Artists:    artists,
		ScannedAt:  time.Now().UTC(),
		byID:       byID,
	}
}

// store publishes a snapshot. Readers only ever see a complete library.
func (r *MusicRepo) store(library *MusicLibrary) {
	r.mu.Lock()
	r.library = library
	r.mu.Unlock()
}

// isAudio reports whether a path has one of the indexed extensions.
func (r *MusicRepo) isAudio(path string) bool {
	return r.extAllowed(filepath.Ext(path))
}

// trackNumberPrefix matches the "07 - " / "07. " / "07_" numbering that file
// names in a hand-built library carry in front of the artist and title. A plain
// space is deliberately not accepted as the separator: "99 Problems" is a title,
// not track 99.
var trackNumberPrefix = regexp.MustCompile(`^(\d{1,3})\s*[.\-_]\s*`)

// applyFileNameFallback fills in the metadata the tags did not carry from the
// file name, which is all a WAV or an untagged download has. "Artist - Title"
// and an optional leading track number are understood; anything else keeps the
// file name as the title.
func applyFileNameFallback(track *MusicTrack) {
	applyNameFallback(track, fileNameTitle(track.Path))
}

// applyNameFallback is the same rule applied to an arbitrary name, which is how
// an upload keeps the metadata of the name the client sent while the file on disk
// is stored under a generated one.
func applyNameFallback(track *MusicTrack, name string) {
	name = strings.TrimSpace(name)

	if m := trackNumberPrefix.FindStringSubmatch(name); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && track.TrackNo == 0 {
			track.TrackNo = n
		}
		name = strings.TrimSpace(name[len(m[0]):])
	}

	artist, title := splitArtistTitle(name)
	if track.Artist == "" {
		track.Artist = artist
	}
	if track.Title == "" {
		track.Title = title
	}
	if track.Title == "" {
		track.Title = fileNameTitle(track.Path)
	}
}

// splitArtistTitle separates the "Artist - Title" convention. A name without
// the separator is the title of a track whose artist is simply unknown.
func splitArtistTitle(name string) (artist, title string) {
	name = strings.TrimSpace(name)
	if i := strings.Index(name, " - "); i > 0 {
		return strings.TrimSpace(name[:i]), strings.TrimSpace(name[i+len(" - "):])
	}
	return "", name
}

// fileNameTitle is a file name without its extension: the last-resort title.
func fileNameTitle(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// musicID derives a stable id from a track's path relative to the music
// directory. Hashing keeps ids opaque and URL-safe, and because every lookup
// goes through the in-memory index, an id can never name a file by itself.
func musicID(rel string) string {
	sum := sha256.Sum256([]byte(rel))
	return hex.EncodeToString(sum[:])[:24]
}

// sortTracks orders the listing the way the UI shows it: by album, then disc and
// track number, then title. Tracks without an album tag sort last, so the loose
// files in a library do not push the real albums down the page.
func sortTracks(tracks []*MusicTrack) {
	sort.SliceStable(tracks, func(i, j int) bool {
		a, b := tracks[i], tracks[j]
		if (a.Album == "") != (b.Album == "") {
			return b.Album == ""
		}
		if la, lb := strings.ToLower(a.Album), strings.ToLower(b.Album); la != lb {
			return la < lb
		}
		if a.DiscNo != b.DiscNo {
			return a.DiscNo < b.DiscNo
		}
		if a.TrackNo != b.TrackNo {
			return a.TrackNo < b.TrackNo
		}
		return strings.ToLower(a.Title) < strings.ToLower(b.Title)
	})
}

// buildAlbumViews aggregates the track list into the album and artist listings.
//
// An album is keyed by name *and* album artist together, so two artists touring
// an album of the same name stay separate, and a compilation with no album
// artist tag falls back to the track artist. Missing names are display
// fallbacks rather than buckets: a track with no album tag is grouped under
// "未知专辑" and sorts after every named album.
func buildAlbumViews(tracks []*MusicTrack) ([]*MusicAlbum, []*MusicArtist) {
	type albumAgg struct {
		album   *MusicAlbum
		covered bool
	}

	albums := make(map[string]*albumAgg)
	artists := make(map[string]*MusicArtist)

	for _, t := range tracks {
		artistName := displayName(t.Artist, "未知艺术家")
		albumName := displayName(t.Album, "未知专辑")
		albumArtist := displayName(t.AlbumArtist, artistName)

		key := albumArtist + "\x00" + albumName
		agg := albums[key]
		if agg == nil {
			agg = &albumAgg{album: &MusicAlbum{Name: albumName, Artist: albumArtist}}
			albums[key] = agg
		}
		agg.album.TrackCount++
		if agg.album.Year == 0 && t.Year != 0 {
			agg.album.Year = t.Year
		}
		if agg.album.CoverID == "" {
			agg.album.CoverID = t.ID
		}
		if t.HasCover && !agg.covered {
			agg.album.CoverID = t.ID
			agg.covered = true
		}

		artist := artists[artistName]
		if artist == nil {
			artist = &MusicArtist{Name: artistName}
			artists[artistName] = artist
		}
		artist.TrackCount++
	}

	albumList := make([]*MusicAlbum, 0, len(albums))
	for _, agg := range albums {
		albumList = append(albumList, agg.album)
	}
	sort.Slice(albumList, func(i, j int) bool {
		if (albumList[i].Name == "未知专辑") != (albumList[j].Name == "未知专辑") {
			return albumList[j].Name == "未知专辑"
		}
		return strings.ToLower(albumList[i].Name) < strings.ToLower(albumList[j].Name)
	})

	artistList := make([]*MusicArtist, 0, len(artists))
	for _, artist := range artists {
		artistList = append(artistList, artist)
	}
	sort.Slice(artistList, func(i, j int) bool {
		return strings.ToLower(artistList[i].Name) < strings.ToLower(artistList[j].Name)
	})

	return albumList, artistList
}

// displayName normalises a possibly-empty metadata string to one that can be
// shown.
func displayName(s, fallback string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	return s
}

// startWatcher rescans shortly after the music directory changes. Watching
// directories rather than files is enough on its own: files appearing,
// disappearing or being renamed all report as events on the containing
// directory, and a full rescan is cheap next to tracking individual paths.
//
// A root that does not exist yet has nothing to watch. That is not a hole the
// user can fall into: uploads go through Save, which indexes the new file
// directly, and the first scan after the directory appears installs the
// watches.
func (r *MusicRepo) startWatcher() {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		r.logger.Warnf("data: music watch unavailable: %v", err)
		return
	}
	r.addWatches(w)

	// The goroutine lives as long as the process: the repository is built once
	// by main and there is nothing later that could unwind it.
	go func() {
		var timer *time.Timer
		var timerC <-chan time.Time

		trigger := func() {
			if timer == nil {
				timer = time.NewTimer(musicWatchDebounce)
				timerC = timer.C
				return
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(musicWatchDebounce)
		}

		for {
			select {
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				r.logger.Warnf("data: music watch error: %v", err)
			case evt, ok := <-w.Events:
				if !ok {
					return
				}
				if evt.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
					continue
				}
				trigger()
			case <-timerC:
				timer, timerC = nil, nil
				r.store(r.scan())
				// Directories created or removed since the last sync.
				r.addWatches(w)
			}
		}
	}()
}

// addWatches registers a watch on the music directory and everything below it,
// skipping anything that cannot be read so the rest of the tree is still
// covered.
func (r *MusicRepo) addWatches(w *fsnotify.Watcher) {
	if r.root == "" {
		return
	}
	_ = filepath.WalkDir(r.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if err := w.Add(path); err != nil {
			r.logger.Warnf("data: watching %s: %v", path, err)
		}
		return nil
	})
}
