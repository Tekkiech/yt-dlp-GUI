# yt-dlp GUI (TUI)

A Charmbracelet-based TUI wrapper around the `yt-dlp` project with form-driven configuration, FFmpeg merge toggle, selectable quality presets, directory picker, and live log viewport.

## Features
- Guided form (URL, quality preset, download directory, merge toggle).
- Live yt-dlp log output while downloading.
- FFmpeg merge toggle (mp4) when applicable.
- Keyboard controls: quit, back, clear logs.
- Cross-platform notification sound on completion.

## Requirements
- Go 1.21+ (module target is 1.26.1).
- `yt-dlp` installed and on `PATH`.
- FFmpeg on `PATH` for merging when enabled.

## Install & Run
```/dev/null/commands.sh#L1-5
go mod tidy
go run .
# or build
go build -o yt-dlp-gui
./yt-dlp-gui
```

## Usage (keys)
- Form screen: fill fields, Enter to submit, Ctrl+C to quit.
- Download screen: `q` quit • `b` back to form • `c` clear logs • Ctrl+C quit.

## Quality presets
- 4K (2160p): `bestvideo[height<=2160]+bestaudio/best`
- 1080p: `bestvideo[height<=1080]+bestaudio/best`
- 720p: `bestvideo[height<=720]+bestaudio/best`
- Audio Only: `bestaudio/best` (merge skipped)

## Project layout
```/dev/null/tree.txt#L1-12
yt-dlp GUI/
├─ main.go                # current app entry (form-based TUI)
├─ go.mod / go.sum
├─ versions/
│  ├─ source/
│  │  ├─ iter1/…iter5/    # earlier source iterations
└─ └─ └─ final/main.go    # final iteration source         # original binary snapshots
```

## Iteration history (source)
- `versions/source/iter1/main.go`: initial text-input URL + log viewport + merge toggle.
- `versions/source/iter2/main.go`: (see file) incremental improvements.
- `versions/source/iter3/main.go`: (see file) incremental improvements.
- `versions/source/iter4/main.go`: (see file) incremental improvements.
- `versions/source/iter5/main.go`: (see file) incremental improvements.
- `versions/source/final/main.go`: form-based UI (current design).

## Current entry point
`main.go` matches `versions/source/final/main.go`:
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

This repository already contains a workflow at `.github/workflows/build.yml`. At the moment the included workflow builds macOS binaries (matrix: `amd64`, `arm64`) on `macos-latest` and uploads the `dist` directory as artifacts.

If you want a broader CI that produces cross-platform release assets, consider:
- Expanding the workflow matrix to include `GOOS=linux` and `GOOS=windows` entries.
- Running `go mod tidy` and then building the repository root package (`.`) for each target.
- Using `actions/upload-artifact` for temporary artifacts and/or automating creation of GitHub Releases (via `actions/create-release` + `actions/upload-release-asset`) for stable release assets.
- Optionally using tools like `goreleaser` to simplify packaging and creating release attachments.

Where to obtain binaries from CI:
- Stable releases: use GitHub Releases (release assets attached to version tags).
- Latest or intermediate builds: use the Actions tab, find the workflow run for the commit or branch, and download the uploaded artifacts from that run.

If you'd like, I can:
- Add a short example workflow snippet that builds for linux/windows/darwin and uploads artifacts, or
- Update the README to include direct links to Releases and Actions pages (or example commands to download artifacts) for convenience.

## License
MIT (if you add one; currently unspecified).
