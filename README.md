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

From a Linux or macOS machine with Go installed, you can cross-compile:

- Build for Linux x86_64:
```sh
GOOS=linux GOARCH=amd64 go build -o yt-dlp-gui-linux ./yt-dlp-GUI
```

- Build for Windows x86_64:
```sh
GOOS=windows GOARCH=amd64 go build -o yt-dlp-gui.exe ./yt-dlp-GUI
```

- Build for macOS (native build or from macOS):
```sh
go build -o yt-dlp-gui ./yt-dlp-GUI
```

Notes:
- The binary is pure Go and should cross-compile cleanly, but runtime behavior (like system sounds) depends on the target OS utilities.
- When packaging, include a short README or installer notes that recommend installing `yt-dlp` and `ffmpeg`.

## CI: GitHub Actions

A suggested CI workflow can build artifacts for macOS, Linux, and Windows. Add a `.github/workflows/build.yml` with a matrix for `GOOS`/`GOARCH` to produce release artifacts. The workflow should:
- Run `go mod tidy`
- Build for each target (linux, windows, darwin)
- Upload build artifacts as part of the job outputs or release assets

## License
MIT (if you add one; currently unspecified).
