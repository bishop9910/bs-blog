package data

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// lookup resolves an id against the current snapshot. Both the lock and the
// missing-entry case live here so every caller reports "not found" the same way.
func (r *MusicRepo) lookup(id string) (*MusicTrack, error) {
	r.mu.RLock()
	library := r.library
	r.mu.RUnlock()

	track := library.Track(strings.ToLower(strings.TrimSpace(id)))
	if track == nil {
		return nil, ErrNotFound
	}
	return track, nil
}

// Library returns the current snapshot. The snapshot is immutable, so the caller
// may read it for as long as it likes without holding the lock.
func (r *MusicRepo) Library(_ context.Context) (*MusicLibrary, error) {
	r.mu.RLock()
	library := r.library
	r.mu.RUnlock()
	return library, nil
}

// Track resolves one id.
func (r *MusicRepo) Track(_ context.Context, id string) (*MusicTrack, error) {
	return r.lookup(id)
}

// Rescan rebuilds the index from disk and returns the fresh snapshot. A scan
// reports nothing it could not read — the library is whatever the disk holds
// right now, and a failure is visible as a smaller listing.
func (r *MusicRepo) Rescan(_ context.Context) (*MusicLibrary, error) {
	library := r.scan()
	r.store(library)
	return library, nil
}

// OpenStream opens a track's audio file. The returned handle is seekable because
// the streaming route hands it to http.ServeContent, which is what turns a
// Range request into the byte range a player asked for.
func (r *MusicRepo) OpenStream(_ context.Context, id string) (io.ReadSeekCloser, *MusicTrack, error) {
	track, err := r.lookup(id)
	if err != nil {
		return nil, nil, err
	}
	// Re-check containment: the path comes from the index, but a symlinked
	// directory would otherwise let the stream follow it out of the library.
	if !withinRoot(r.root, track.Path) {
		return nil, nil, ErrNotFound
	}

	f, err := os.Open(track.Path)
	if err != nil {
		if os.IsNotExist(err) {
			// The file was removed behind the index (a sync, a manual delete).
			// The next rescan drops it; until then it reads as not found.
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	return f, track, nil
}

// Cover reads one track's cover art. Nothing is cached: a browser asks for a
// cover once and keeps it for a day, which is cheaper than holding every image
// in memory.
func (r *MusicRepo) Cover(_ context.Context, id string) (*MusicCover, error) {
	track, err := r.lookup(id)
	if err != nil {
		return nil, err
	}
	if !track.HasCover {
		return nil, ErrNotFound
	}

	data, mime, err := resolveCover(r.root, track.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if len(data) == 0 {
		return nil, ErrNotFound
	}
	if mime == "" {
		mime = "image/jpeg"
	}
	return &MusicCover{MIME: mime, Data: data}, nil
}

// Lyrics returns a track's lyrics, decoded to UTF-8. Text comes from the sidecar
// when it carries any and otherwise from the .lrc file next to the audio; a
// track with neither returns an empty string rather than an error, because "no
// lyrics" is a normal state the page has to render anyway.
func (r *MusicRepo) Lyrics(_ context.Context, id string) (string, error) {
	track, err := r.lookup(id)
	if err != nil {
		return "", err
	}

	if sc, err := readSidecar(sidecarPath(track.Path)); err == nil {
		if inline := strings.TrimSpace(sc.Lyrics); inline != "" {
			if !looksLikeLyricsPath(inline) {
				return sc.Lyrics, nil
			}
			full, err := resolveSidecarPath(r.root, filepath.Dir(track.Path), inline)
			if err != nil {
				r.logger.Warnf("data: ignoring sidecar lyrics of %s: %v", track.Path, err)
				return "", nil
			}
			return readLyrics(full), nil
		}
	}
	return readLyrics(lyricsPath(track.Path)), nil
}
