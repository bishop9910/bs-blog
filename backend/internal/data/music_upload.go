package data

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrMusicFormat marks an upload whose bytes are not an audio format the library
// indexes. The use case turns it into the message the user reads; the checks
// stay here because this is where the bytes are actually inspected.
var ErrMusicFormat = errors.New("data: unsupported audio format")

// ErrMusicDisabled marks an upload while music.dir is empty: there is nowhere to
// put the file, so the configuration has to be fixed first.
var ErrMusicDisabled = errors.New("data: music.dir is not configured")

// audioExtByMIME maps a media type onto the extension a stored upload gets.
//
// Go's sniffer can only name formats it has a rule for: it recognises an ID3 tag
// ("audio/mpeg"), an Ogg page ("application/ogg"), a RIFF/WAVE header
// ("audio/wave") and MP4 ("video/mp4"), but has no rule at all for FLAC or for
// an MPEG frame without an ID3 tag. Those arrive as application/octet-stream and
// are resolved from the uploaded name in Save.
var audioExtByMIME = map[string]string{
	"audio/mpeg":      ".mp3",
	"audio/mp3":       ".mp3",
	"audio/flac":      ".flac",
	"audio/x-flac":    ".flac",
	"audio/mp4":       ".m4a",
	"audio/m4a":       ".m4a",
	"video/mp4":       ".m4a",
	"application/ogg": ".ogg",
	"audio/ogg":       ".ogg",
	"audio/opus":      ".opus",
	"audio/wav":       ".wav",
	"audio/wave":      ".wav",
	"audio/x-wav":     ".wav",
	"audio/vnd.wave":  ".wav",
}

// extAllowed reports whether an extension (with or without its dot) is one the
// library indexes.
func (r *MusicRepo) extAllowed(ext string) bool {
	_, ok := r.exts[strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))]
	return ok
}

// Save writes an uploaded audio file into the library and returns the indexed
// track.
//
// The file lands in <dir>/<year>/<month>/<uuid>.<ext>, mirroring the image
// library: the stored name never comes from the client, so an upload cannot
// choose where it goes or what it is called, and the year/month split keeps any
// one directory small. The new track is added to the index directly instead of
// rescanning the directory.
func (r *MusicRepo) Save(_ context.Context, header *multipart.FileHeader) (*MusicTrack, error) {
	if r.root == "" {
		return nil, ErrMusicDisabled
	}
	if header == nil {
		return nil, ErrMusicFormat
	}

	nameExt := filepath.Ext(strings.TrimSpace(header.Filename))
	if !r.extAllowed(nameExt) {
		return nil, ErrMusicFormat
	}

	src, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	// Neither the extension nor the declared Content-Type is trustworthy, so
	// the bytes decide. A format the sniffer cannot name falls back to the
	// uploaded extension, which has already been restricted to the allow-list.
	head := make([]byte, 512)
	n, readErr := io.ReadFull(src, head)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return nil, readErr
	}
	sniffed := normaliseMediaType(http.DetectContentType(head[:n]))

	ext, named := audioExtByMIME[sniffed]
	if !named {
		if sniffed != "application/octet-stream" {
			return nil, ErrMusicFormat
		}
		ext = strings.ToLower(nameExt)
	}
	// A format the library is not configured to index would be written and then
	// skipped by every later scan, so it is refused here instead of becoming an
	// invisible file.
	if !r.extAllowed(ext) {
		return nil, ErrMusicFormat
	}

	now := time.Now().UTC()
	relDir := filepath.Join(now.Format("2006"), now.Format("01"))
	dir := filepath.Join(r.root, relDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	// The first upload is also what creates a music directory that did not exist
	// when the server started.
	storedPath := filepath.Join(dir, uuid.NewString()+ext)

	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	dst, err := os.Create(storedPath)
	if err != nil {
		return nil, err
	}
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(storedPath)
		if copyErr != nil {
			return nil, copyErr
		}
		return nil, closeErr
	}

	track, err := r.indexFile(storedPath)
	if err != nil {
		// A file the index cannot describe would never be listed, so keeping it
		// would leave an orphan nobody can see or delete.
		_ = os.Remove(storedPath)
		return nil, err
	}
	// An untagged upload is titled after the generated name on disk, which says
	// nothing to a reader. The name the client sent is the only metadata left, so
	// it is parsed with the same rule a scanned file name goes through.
	if sent := fileNameTitle(strings.TrimSpace(header.Filename)); track.Title == fileNameTitle(storedPath) && sent != "" {
		track.Title = ""
		applyNameFallback(track, sent)
	}
	r.insert(track)
	return track, nil
}

// Delete removes a track's file from disk and drops it from the index.
//
// Only files the index already knows about can be deleted, and the path is
// checked to be inside the music directory again: the index is the gatekeeper,
// but a symlinked directory could still point outside it.
func (r *MusicRepo) Delete(_ context.Context, id string) error {
	track, err := r.lookup(id)
	if err != nil {
		return err
	}
	if !withinRoot(r.root, track.Path) {
		return ErrNotFound
	}

	if err := os.Remove(track.Path); err != nil && !os.IsNotExist(err) {
		return err
	}
	// The sidecars belong to the track. Leaving them behind would re-attach
	// stale metadata and lyrics to whatever file lands on the same name next.
	for _, side := range []string{sidecarPath(track.Path), lyricsPath(track.Path)} {
		if err := os.Remove(side); err != nil && !os.IsNotExist(err) {
			r.logger.Warnf("data: removing %s: %v", side, err)
		}
	}

	r.forget(id)
	return nil
}

// insert adds one freshly indexed track to the current snapshot, replacing any
// older entry with the same id. The album and artist views are rebuilt from the
// new list, so an upload shows up in the listing without a directory walk.
func (r *MusicRepo) insert(track *MusicTrack) {
	r.mu.RLock()
	current := r.library
	r.mu.RUnlock()

	tracks := make([]*MusicTrack, 0, len(current.Tracks)+1)
	replaced := false
	for _, existing := range current.Tracks {
		if existing.ID == track.ID {
			tracks = append(tracks, track)
			replaced = true
			continue
		}
		tracks = append(tracks, existing)
	}
	if !replaced {
		tracks = append(tracks, track)
	}
	r.store(r.newLibrary(tracks, true))
}

// forget drops one track from the current snapshot.
func (r *MusicRepo) forget(id string) {
	r.mu.RLock()
	current := r.library
	r.mu.RUnlock()

	tracks := make([]*MusicTrack, 0, len(current.Tracks))
	for _, existing := range current.Tracks {
		if existing.ID != id {
			tracks = append(tracks, existing)
		}
	}
	r.store(r.newLibrary(tracks, current.Exists))
}
