#!/usr/bin/env bash
#
# build-all.sh
#
# Simple script to build cross-platform binaries for the yt-dlp-GUI project.
# Place this file at yt-dlp-GUI/scripts/build-all.sh and run it from anywhere.
#
# Defaults:
#   Targets: linux/amd64, windows/amd64, darwin/amd64, darwin/arm64
#   Output dir: dist (created alongside the script's parent repo root)
#
# Overrides:
#   Set TARGETS environment variable to a comma-separated list of os:arch pairs:
#     e.g. TARGETS="linux:amd64,darwin:arm64"
#   Set OUTDIR to change the output directory.
#
# Examples:
#   TARGETS="linux:amd64,windows:amd64" ./yt-dlp-GUI/scripts/build-all.sh
#   ./yt-dlp-GUI/scripts/build-all.sh
#

set -euo pipefail

# Resolve the script directory (supports being invoked from any cwd)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

# Try to locate the repository root by searching for a `go.mod` file.
# First search upward from the script directory (so the script can be invoked
# from anywhere, including CI runners). If not found, search upward from the
# current working directory as a fallback.
SEARCH_DIR="$SCRIPT_DIR"
REPO_ROOT=""
while [ "$SEARCH_DIR" != "/" ]; do
  if [ -f "$SEARCH_DIR/go.mod" ]; then
    REPO_ROOT="$SEARCH_DIR"
    break
  fi
  SEARCH_DIR="$(dirname "$SEARCH_DIR")"
done

if [ -z "$REPO_ROOT" ]; then
  SEARCH_DIR="$(pwd)"
  while [ "$SEARCH_DIR" != "/" ]; do
    if [ -f "$SEARCH_DIR/go.mod" ]; then
      REPO_ROOT="$SEARCH_DIR"
      break
    fi
    SEARCH_DIR="$(dirname "$SEARCH_DIR")"
  done
fi

if [ -z "$REPO_ROOT" ]; then
  echo "Error: could not find go.mod in any parent directory; please run this script from inside the repository." >&2
  exit 2
fi

# Default configuration
DEFAULT_TARGETS=("linux:amd64" "windows:amd64" "darwin:amd64" "darwin:arm64")
TARGETS_ENV="${TARGETS:-}"
OUTDIR="${OUTDIR:-$REPO_ROOT/dist}"

# Determine what package path to build. If the repo contains a subdirectory
# named `yt-dlp-GUI`, build that package explicitly; otherwise build the
# repository root package.
if [ -d "$REPO_ROOT/yt-dlp-GUI" ]; then
  BUILD_PKG="./yt-dlp-GUI"
else
  BUILD_PKG="."
fi

# Ensure 'go' is available
if ! command -v go >/dev/null 2>&1; then
  echo "Error: 'go' is not found in PATH. Install Go and try again." >&2
  exit 2
fi

# Run go mod tidy to ensure modules are fetched (safe to run repeatedly)
echo "Running 'go mod tidy' in repository root ($REPO_ROOT)..."
(
  cd "$REPO_ROOT"
  go mod tidy
)

# Prepare targets list
IFS=',' read -r -a TARGETS_ARRAY <<< "${TARGETS_ENV:-}"
if [ "${#TARGETS_ARRAY[@]}" -eq 0 ] || [ -z "${TARGETS_ARRAY[0]}" ]; then
  TARGETS_ARRAY=("${DEFAULT_TARGETS[@]}")
fi

mkdir -p "$OUTDIR"
echo "Output directory: $OUTDIR"
echo

for pair in "${TARGETS_ARRAY[@]}"; do
  # Trim whitespace
  pair="$(echo "$pair" | tr -d '[:space:]')"
  if [[ ! "$pair" =~ ^[^:]+:[^:]+$ ]]; then
    echo "Skipping invalid target '$pair' (expected format os:arch)" >&2
    continue
  fi

  OS="${pair%%:*}"
  ARCH="${pair##*:}"

  # Choose extension for Windows
  EXT=""
  if [ "$OS" = "windows" ]; then
    EXT=".exe"
  fi

  OUTPUT_NAME="yt-dlp-gui-${OS}-${ARCH}${EXT}"
  OUTPUT_PATH="${OUTDIR}/${OUTPUT_NAME}"

  echo "Building for ${OS}/${ARCH} -> ${OUTPUT_PATH} ..."
  # Use CGO_ENABLED=0 for static-ish builds where possible (avoids requiring cross-compiled C toolchains)
  env GOOS="$OS" GOARCH="$ARCH" CGO_ENABLED=0 go build -o "$OUTPUT_PATH" "$REPO_ROOT/$BUILD_PKG"

  if [ $? -eq 0 ]; then
    echo "Built: $OUTPUT_PATH"
  else
    echo "Build failed for ${OS}/${ARCH}" >&2
    exit 3
  fi
  echo
done

echo "All done. Built artifacts in: $OUTDIR"
ls -lh "$OUTDIR" || true

# End
