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

## License
MIT (if you add one; currently unspecified).
