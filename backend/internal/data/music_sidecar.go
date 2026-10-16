package data

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/dhowden/tag"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// Cover art and lyrics are read within the caps the repo was built with
// (music.maxCoverBytes / music.maxLyricsBytes, see MusicOptions). A cap matters
// because a sidecar is a local file that a mistaken path could point anywhere:
// the size is checked before anything is read into memory.

// trackSidecar is the optional JSON metadata that may sit next to an audio file
// (song.mp3 → song.json). Every field is optional and a field that is present
// wins over the file's own tags, which is what makes the sidecar worth having:
// the blog can describe a track without rewriting the audio file, and formats
// that carry no tags at all (WAV) can still be organised.
type trackSidecar struct {
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	AlbumArtist string `json:"albumArtist"`
	Year        int    `json:"year"`
	// Cover is a path to an image file, relative to the sidecar's directory. It
	// must stay inside the music directory; absolute paths and ".." escapes are
	// rejected so a sidecar cannot make the server read arbitrary files.
	Cover string `json:"cover"`
	// Lyrics is inline LRC text, or a relative path (ending in .lrc/.txt) to a
	// lyrics file. A path follows the same rules as Cover.
	Lyrics string `json:"lyrics"`
}

// sidecarPath is the JSON override that belongs to an audio file.
func sidecarPath(audioPath string) string {
	return strings.TrimSuffix(audioPath, filepath.Ext(audioPath)) + ".json"
}

// lyricsPath is the .lrc file that belongs to an audio file. It is the plain
// convention players and editors already write, so a library tagged elsewhere
// picks up its lyrics without any extra step.
func lyricsPath(audioPath string) string {
	return strings.TrimSuffix(audioPath, filepath.Ext(audioPath)) + ".lrc"
}

// readSidecar loads a JSON sidecar. A missing file is reported as an
// os.IsNotExist error so the caller can tell "absent" from "malformed" and only
// warn about the latter.
func readSidecar(path string) (*trackSidecar, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s trackSidecar
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("data: parsing sidecar %s: %w", path, err)
	}
	return &s, nil
}

// resolveSidecarPath turns a path written inside a sidecar into an absolute one,
// rejecting absolute paths and anything that escapes root. Both Cover and
// Lyrics go through it.
func resolveSidecarPath(root, dir, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("data: empty sidecar path")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("data: sidecar path must be relative: %q", rel)
	}
	full := filepath.Clean(filepath.Join(dir, rel))
	if !withinRoot(root, full) {
		return "", fmt.Errorf("data: sidecar path escapes the music directory: %q", rel)
	}
	return full, nil
}

// looksLikeLyricsPath reports whether a sidecar lyrics value names a file rather
// than holding inline LRC text.
func looksLikeLyricsPath(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.HasSuffix(s, ".lrc") || strings.HasSuffix(s, ".txt")
}

// loadCover reads a cover image referenced by a sidecar.
func (r *MusicRepo) loadCover(dir, rel string) ([]byte, string, error) {
	full, err := resolveSidecarPath(r.root, dir, rel)
	if err != nil {
		return nil, "", err
	}
	st, err := os.Stat(full)
	if err != nil {
		return nil, "", err
	}
	if st.Size() > r.coverBytes {
		return nil, "", fmt.Errorf("data: sidecar cover too large (%d bytes): %q", st.Size(), rel)
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return nil, "", err
	}
	return b, http.DetectContentType(b), nil
}

// probeCover reports whether a sidecar cover file exists and which media type it
// claims, without reading the image. A scan only needs to know that a cover is
// there — the bytes are read when the cover route is called — and reading every
// cover of a large library just to record its type would dwarf the index.
func (r *MusicRepo) probeCover(dir, rel string) (string, error) {
	full, err := resolveSidecarPath(r.root, dir, rel)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if st.Size() > r.coverBytes {
		return "", fmt.Errorf("data: sidecar cover too large (%d bytes): %q", st.Size(), rel)
	}

	f, err := os.Open(full)
	if err != nil {
		return "", err
	}
	defer f.Close()

	head := make([]byte, 512)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", err
	}
	return http.DetectContentType(head[:n]), nil
}

// resolveCover returns the cover art of one audio file: the picture embedded in
// the file wins, otherwise a cover named by its sidecar.
//
// The bytes are read on every call rather than kept in the index. Covers are
// served with a long cache lifetime, so a browser asks once, while holding one
// image per track in memory would dwarf everything else the library keeps.
func (r *MusicRepo) resolveCover(audioPath string) ([]byte, string, error) {
	if f, err := os.Open(audioPath); err == nil {
		m, tagErr := tag.ReadFrom(f)
		_ = f.Close()
		if tagErr == nil && m != nil {
			if p := m.Picture(); p != nil && len(p.Data) > 0 {
				mime := normaliseMediaType(p.MIMEType)
				if mime == "" {
					// Some taggers leave the field empty even though every
					// player in practice stores JPEG here.
					mime = "image/jpeg"
				}
				return p.Data, mime, nil
			}
		}
	}

	sc, err := readSidecar(sidecarPath(audioPath))
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(sc.Cover) == "" {
		return nil, "", os.ErrNotExist
	}
	return r.loadCover(filepath.Dir(audioPath), sc.Cover)
}

// readLyrics loads a .lrc file next to a track, returning "" when there is none.
// Lyrics are best-effort: a missing or unreadable file is not an error, it just
// means the track has no lyrics to show.
func (r *MusicRepo) readLyrics(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	b, err := io.ReadAll(io.LimitReader(f, r.lyricsBytes))
	if err != nil {
		return ""
	}
	return decodeLyrics(b)
}

// hasLyrics reports whether a track has lyrics without reading them, so a
// directory scan does not pull the text of every song into memory. Only
// existence and size are checked; the text itself is decoded when the lyrics
// route is actually called.
func (r *MusicRepo) hasLyrics(audioPath string, sc *trackSidecar) bool {
	if sc != nil {
		if inline := strings.TrimSpace(sc.Lyrics); inline != "" {
			if !looksLikeLyricsPath(inline) {
				return true
			}
			full, err := resolveSidecarPath(r.root, filepath.Dir(audioPath), inline)
			if err != nil {
				return false
			}
			if st, err := os.Stat(full); err == nil {
				return st.Size() > 0
			}
			return false
		}
	}
	st, err := os.Stat(lyricsPath(audioPath))
	return err == nil && st.Size() > 0
}

// decodeLyrics normalises lyric text to UTF-8. Chinese .lrc files are commonly
// written in GBK/GB2312 (or carry a UTF-8 BOM) rather than UTF-8, and serving
// those bytes as-is produces mojibake in the JSON response. Valid UTF-8 is kept
// untouched and anything else is decoded as GB18030, a superset of GBK/GB2312.
func decodeLyrics(b []byte) string {
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
	if utf8.Valid(b) {
		return string(b)
	}
	if out, err := simplifiedchinese.GB18030.NewDecoder().Bytes(b); err == nil {
		return string(out)
	}
	return string(b)
}

// normaliseMediaType lowercases a media type and drops any parameters.
func normaliseMediaType(value string) string {
	if i := strings.IndexByte(value, ';'); i >= 0 {
		value = value[:i]
	}
	return strings.ToLower(strings.TrimSpace(value))
}

// withinRoot reports whether path is at or below root. Both are cleaned first,
// so a path built from a relative name cannot slip out with "..".
func withinRoot(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
