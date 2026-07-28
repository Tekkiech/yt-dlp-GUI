# yt-dlp GUI (TUI)

A Charmbracelet-based TUI wrapper around the `yt-dlp` project with form-driven configuration, video/audio format conversion via ffmpeg, a directory picker, and a live animated progress view.

## Features
- Guided, conditional form: pick video or audio-only, then the relevant options for that mode.
- Video: quality presets (4K/1080p/720p/best) and an output container — MP4 (fast merge) or MKV/WebM/MOV/AVI (real ffmpeg re-encode).
- Audio-only: extract to MP3, FLAC, WAV, OGG (Vorbis), M4A (AAC), Opus, ALAC, or keep the original best audio codec — all via yt-dlp's ffmpeg-backed audio extractor.
- Live animated, gradient progress bar and speed/ETA/size stats parsed from yt-dlp's own output, plus the full raw log.
- Keyboard controls: quit, back, clear logs.
- Cross-platform notification sound on completion.

## Requirements
- Go 1.21+ (module target is 1.26.1).
- `yt-dlp` installed and on `PATH` — required; the app checks at startup and shows an OS/arch-specific install command if it's missing.
- `ffmpeg` on `PATH` for format conversion (merging, MKV/WebM/MOV/AVI re-encoding, MP3/FLAC/WAV/etc. audio extraction). Also checked at startup: if missing, the app still works but only offers the original, unconverted format, with an install hint shown on screen.

## Install & Run
```/dev/null/commands.sh#L1-5
go mod tidy
go run .
# or build
go build -o yt-dlp-gui
./yt-dlp-gui
```

### Install to /usr/local/bin (macOS)
```sh
make install    # builds and installs to /usr/local/bin/yt-dlp-gui (prompts for sudo)
make uninstall   # removes it
```
Check the installed version with `yt-dlp-gui --version`.

## Usage (keys)
- Form screen: fill fields, Enter to submit, Ctrl+C to quit.
- Download screen: `q` quit • `b` back to form • `c` clear logs • Ctrl+C quit.

## Video quality & format
- Quality: 4K (2160p), 1080p, 720p, or best available (`bestvideo[height<=N]+bestaudio/best`).
- Format: MP4 merges the downloaded streams losslessly (`--merge-output-format mp4`); MKV/WebM/MOV/AVI trigger a real ffmpeg re-encode (`--recode-video`) so the conversion always succeeds, even across incompatible codec/container pairs; "Original" skips conversion entirely.

## Audio formats
Selecting "Audio only" downloads the best audio stream, then optionally converts it with yt-dlp's `--extract-audio --audio-format <fmt> --audio-quality 0` (ffmpeg under the hood): MP3, FLAC, WAV, OGG (Vorbis), M4A (AAC), Opus, ALAC, or "Best (original)" to skip conversion and keep the source codec/container.

## Project layout
```/dev/null/tree.txt#L1-6
yt-dlp GUI/
├─ main.go                # app entry (form-based TUI)
├─ go.mod / go.sum
├─ Makefile                # build/install targets
├─ scripts/build-all.sh    # cross-platform build script
└─ .github/workflows/      # CI: build + auto-release
```

## Current entry point
`main.go`:
- Uses `huh` for form.
- `bubbletea` + `viewport` for logs.
- `lipgloss` for styling.
- Completion sound per OS.

## Troubleshooting
- “yt-dlp not found”: install `yt-dlp` and ensure it’s on `PATH`.
- Merge fails: verify FFmpeg on `PATH`.
- Terminal too small: resize; viewport auto-resizes on `WindowSizeMsg`.

## Cross-platform notes

This project aims to be cross-platform (macOS, Linux, Windows). A few runtime behaviors depend on external system utilities; here's what to expect and how to prepare each platform.

External dependencies
- `yt-dlp` (preferred) or `youtube-dl` must be installed and available on PATH. The program will attempt to find `yt-dlp`, then `yt_dlp`, then `youtube-dl`.
- `ffmpeg` should be installed if you want yt-dlp to merge video + audio.
- Notifications & sounds:
  - The app uses the `beeep` library for cross-platform desktop notifications and a short beep where supported.
  - If `beeep` cannot deliver a notification on a system, the binary will attempt a series of platform-specific players/commands (e.g., `afplay` on macOS, `paplay`/`aplay`/`play` on Linux, PowerShell on Windows) and finally fall back to the terminal bell.

Installing useful packages per platform
- macOS (Homebrew):
```sh
brew install yt-dlp ffmpeg sox
```
- Ubuntu/Debian:
```sh
sudo apt update
sudo apt install -y yt-dlp ffmpeg libasound2-utils sox libcanberra-gtk-module libcanberra-gtk3-module
```
- Windows:
  - Install `yt-dlp.exe` (winget or manual download) and `ffmpeg`, and add them to PATH. PowerShell is typically available on modern Windows and is used for a simple notification sound.

## Building cross-platform binaries (examples)

This repository's main package is located at the repository root (there is no `./yt-dlp-GUI` subpackage). Build from the repo root (`.`). Example builds:

- Build for Linux x86_64:
```sh
GOOS=linux GOARCH=amd64 go build -o yt-dlp-gui-linux .
```

- Build for Windows x86_64:
```sh
GOOS=windows GOARCH=amd64 go build -o yt-dlp-gui.exe .
```

- Build for macOS (native build or from macOS):
```sh
go build -o yt-dlp-gui .
```

Notes:
- Build from `.` (the repository root) — do not use `./yt-dlp-GUI` unless you move the entrypoint into that subdirectory.
- The binary is pure Go and generally cross-compiles, but runtime behaviour (notifications, sounds, merge steps) depends on the target OS utilities being present.
- When packaging, include a short README or installer notes that recommend installing `yt-dlp` and `ffmpeg`.

Where to get builds
- Stable releases: check this repository's "Releases" page for published release assets (these are intended to be stable binaries).
- More up-to-date builds: check the "Actions" tab (GitHub Actions) for workflow runs — many workflows upload build artifacts for recent commits which you can download if you need a newer build than the latest release.

## CI: GitHub Actions

The workflow at `.github/workflows/build.yml` builds cross-platform binaries (`linux`/`windows`/`darwin` × `amd64`/`arm64`) on every push and pull request, and auto-releases:

- **Push to `main`**: the workflow computes the next patch version from the highest existing `vX.Y.Z` tag (e.g. `v1.0.1` -> `v1.0.2`), builds all targets with that version embedded (`main.version`), creates the git tag, and publishes a GitHub Release with all binaries attached — no manual tagging required.
- **Manual tag push** (`git tag vX.Y.Z && git push --tags`) or `workflow_dispatch`: also produces/updates a release using that tag.
- **Pull requests**: binaries are built and uploaded as workflow artifacts for validation only — no release is created.

Where to obtain binaries:
- Stable releases: GitHub Releases (auto-published on every `main` push).
- Latest or intermediate builds: the Actions tab, for the workflow run matching a specific commit/branch.

## License
[MIT](LICENSE)
