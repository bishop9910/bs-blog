#!/usr/bin/env bash
#
# Packaging script for Linux and macOS -- the shell twin of build-release.ps1.
#
# Builds the single-file release. The frontend is compiled into the Go binary
# with go:embed, so the result runs with no Node and no dist directory next to
# it: one executable plus configs/ and data/.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: backend/scripts/build-release.sh [options]

Options:
  -v, --version VERSION   version string baked into the binary
                          (default: git describe --tags --always --dirty)
  -s, --skip-frontend     reuse the existing dist/ instead of running pnpm build
  -h, --help              show this help

Environment:
  GOOS / GOARCH           cross-compile target. The binary is named bs-blog,
                          or bs-blog.exe when GOOS is windows.
EOF
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BE_DIR="$(dirname "$SCRIPT_DIR")"
ROOT_DIR="$(dirname "$BE_DIR")"

EMBED_DIR="$BE_DIR/internal/web/dist"
RELEASE_DIR="$BE_DIR/release"

VERSION=""
SKIP_FRONTEND=0

while [ $# -gt 0 ]; do
  case "$1" in
    -v|--version)
      if [ $# -lt 2 ]; then
        echo "build-release: --version needs a value" >&2
        exit 2
      fi
      VERSION="$2"
      shift 2
      ;;
    --version=*)
      VERSION="${1#*=}"
      shift
      ;;
    -s|--skip-frontend)
      SKIP_FRONTEND=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "build-release: unknown option '$1'" >&2
      usage >&2
      exit 2
      ;;
  esac
done

# git describe fails outside a repository and again on a repository with no
# tags, so neither case may abort the script under `set -e`.
if [ -z "$VERSION" ]; then
  VERSION="$(git -C "$ROOT_DIR" describe --tags --always --dirty 2>/dev/null || true)"
fi
case "$VERSION" in
  ""|fatal*|usage*) VERSION="dev" ;;
esac

TARGET_GOOS="${GOOS:-$(go env GOOS)}"
TARGET_GOARCH="${GOARCH:-$(go env GOARCH)}"
BIN_NAME="bs-blog"
if [ "$TARGET_GOOS" = "windows" ]; then
  BIN_NAME="bs-blog.exe"
fi

echo "version: $VERSION"
echo "target:  $TARGET_GOOS/$TARGET_GOARCH"

# --- 1. frontend -------------------------------------------------------------
if [ "$SKIP_FRONTEND" -eq 1 ]; then
  echo "=== frontend build ==="
  echo "    skipped (--skip-frontend)"
  if [ ! -f "$ROOT_DIR/dist/index.html" ]; then
    echo "FAIL: --skip-frontend was given but $ROOT_DIR/dist/index.html does not exist" >&2
    exit 1
  fi
else
  echo "=== frontend build ==="
  ( cd "$ROOT_DIR" && pnpm build )
fi

if [ ! -f "$ROOT_DIR/dist/index.html" ]; then
  echo "FAIL: $ROOT_DIR/dist/index.html is missing -- the frontend build produced nothing" >&2
  exit 1
fi

# --- 2. stage the frontend where go:embed can reach it -----------------------
# go:embed cannot reference anything outside the Go module, and dist/ lives at
# the repository root, so the files have to be copied in first.
echo "=== staging dist/ for go:embed ==="
mkdir -p "$EMBED_DIR"
# Clear the staging directory but keep the placeholder that holds it in git: an
# empty dist/ makes `//go:embed all:dist` match no files at all, and the package
# then fails to compile.
find "$EMBED_DIR" -mindepth 1 -maxdepth 1 ! -name '.gitkeep' -exec rm -rf {} +
: > "$EMBED_DIR/.gitkeep"
cp -a "$ROOT_DIR/dist/." "$EMBED_DIR/"

if [ ! -f "$EMBED_DIR/index.html" ]; then
  echo "FAIL: staging did not put index.html into $EMBED_DIR" >&2
  exit 1
fi

# --- 3. windows resources ----------------------------------------------------
# The icon is drawn from code (tools/icongen) and the version resource is
# compiled by goversioninfo into a .syso -- a COFF object Go links straight into
# the executable. Both are regenerated on every build instead of being checked
# in, which is the only way the version Explorer shows in the Properties dialog
# can track the one baked in below by -ldflags.
echo "=== windows resources ==="
if [ "$TARGET_GOOS" != "windows" ]; then
  echo "    skipped: the target is $TARGET_GOOS/$TARGET_GOARCH and .syso resources are Windows-only"
elif ! command -v goversioninfo >/dev/null 2>&1; then
  echo "NOTE: goversioninfo is not on PATH, so the executable gets no icon and no"
  echo "NOTE: version resource. Install it and rebuild:"
  echo "NOTE:   go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest"
else
  if ! ( cd "$BE_DIR" && go run ./tools/icongen ); then
    echo "FAIL: icongen could not draw the icon" >&2
    exit 1
  fi

  # Windows stores a version as four 16-bit numbers, so the git-derived string
  # only maps onto them when it opens with dotted numbers. A bare commit hash or
  # "dev" leaves the placeholders in winres/versioninfo.json alone and overrides
  # just the text fields.
  ver_args=(-file-version "$VERSION" -product-version "$VERSION")
  if [[ "$VERSION" =~ ^v?([0-9]+)\.([0-9]+)\.([0-9]+)(\.([0-9]+))? ]]; then
    major="${BASH_REMATCH[1]}"
    minor="${BASH_REMATCH[2]}"
    patch="${BASH_REMATCH[3]}"
    build="${BASH_REMATCH[5]:-0}"
    ver_args+=(-ver-major "$major" -ver-minor "$minor" -ver-patch "$patch" -ver-build "$build")
    ver_args+=(-product-ver-major "$major" -product-ver-minor "$minor" \
               -product-ver-patch "$patch" -product-ver-build "$build")
  fi

  # goversioninfo resolves the JSON, its IconPath and its output against the
  # working directory, so run it from winres/ and move what it writes into the
  # package directory -- Go only links .syso from the directory it is building.
  if ! ( cd "$BE_DIR/winres" && goversioninfo "${ver_args[@]}" -platform-specific versioninfo.json ); then
    echo "FAIL: goversioninfo could not build the resource objects" >&2
    exit 1
  fi
  mv "$BE_DIR"/winres/resource_windows_*.syso "$BE_DIR/cmd/server/"
  echo "    wrote 4 resource objects for $TARGET_GOARCH (version $VERSION)"
fi

# --- 4. build ----------------------------------------------------------------
echo "=== go build ==="
mkdir -p "$RELEASE_DIR"
(
  cd "$BE_DIR"
  CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$VERSION" \
    -o "$RELEASE_DIR/$BIN_NAME" ./cmd/server
)
chmod +x "$RELEASE_DIR/$BIN_NAME"

# --- 5. assemble the release directory ---------------------------------------
# release/data is never cleared or written to. A live blog database is the one
# thing here that cannot be rebuilt, and clobbering it is far worse than
# starting from an empty directory.
mkdir -p "$RELEASE_DIR/configs" "$RELEASE_DIR/data/uploads"
cp -f "$BE_DIR/configs/config.yaml" "$RELEASE_DIR/configs/config.yaml"

CFG="$RELEASE_DIR/configs/config.yaml"
if ! grep -Eq '^[[:space:]]*staticDir:[[:space:]]*""[[:space:]]*$' "$CFG"; then
  echo "NOTE: the released config.yaml sets a non-empty staticDir, so this binary"
  echo "NOTE: will serve that directory instead of the embedded frontend."
fi
if [ -f "$BE_DIR/data/blog.db" ] && [ ! -f "$RELEASE_DIR/data/blog.db" ]; then
  echo "NOTE: $BE_DIR/data/blog.db exists but $RELEASE_DIR/data/blog.db does not."
  echo "NOTE: copy it across yourself if this release should keep the existing"
  echo "NOTE: blog. This script will not overwrite a live database for you."
fi

echo "=== release ==="
ls -la "$RELEASE_DIR"
echo
echo "run it with:"
echo "  cd '$RELEASE_DIR' && ./$BIN_NAME"
