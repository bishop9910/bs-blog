// Package web carries the built frontend inside the binary so a deployment is
// a single executable.
//
// The dist directory next to this file is filled in by
// backend/scripts/build-release.ps1, which runs the Vite build at the
// repository root and copies the result here before invoking go build. The copy
// is not optional: go:embed cannot reach outside the Go module, and the Vite
// output lives two levels above it.
//
// Only a .gitkeep is committed. A fresh clone therefore builds a binary with no
// frontend inside it, which is deliberate -- that binary serves the API alone
// instead of failing, so `go build ./...`, `go vet ./...` and `go test ./...`
// all work without a Node toolchain.
package web

import (
	"embed"
	"io/fs"
)

// The all: prefix is load-bearing. Without it, embed skips names beginning with
// "." or "_", which would drop the committed .gitkeep, leave the pattern
// matching nothing, and turn every build from a fresh clone into a compile
// error.
//
//go:embed all:dist
var dist embed.FS

// Dist returns the embedded frontend tree. The result is always usable as an
// fs.FS; use Built to find out whether it actually holds a frontend.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// Unreachable: the directive above embeds exactly that directory.
		panic("web: dist is not embedded: " + err.Error())
	}
	return sub
}

// Built reports whether a real frontend build was embedded, as opposed to the
// bare .gitkeep a fresh clone carries.
func Built() bool {
	info, err := fs.Stat(Dist(), "index.html")
	return err == nil && !info.IsDir()
}
