package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// downloadConfig captures everything the form collects before a download
// starts, translated into the values runYtDlpCmd needs.
type downloadConfig struct {
	url  string
	dir  string
	mode string // "video" or "audio"

	// video mode
	videoQuality string // "2160", "1080", "720", or "best"
	videoFormat  string // "mp4", "mkv", "webm", "mov", "avi", or "none" (no conversion)

	// audio mode
	audioFormat string // "best", "mp3", "flac", "wav", "vorbis", "m4a", "opus", "aac", "alac"

	// ffmpegAvailable gates every flag that requires ffmpeg (merging,
	// recoding, audio extraction) so a missing ffmpeg can never produce a
	// command that silently fails or hangs waiting on a postprocessor.
	ffmpegAvailable bool
}

// toolCheck is the result of probing PATH (and optionally --version) for a
// required external tool at startup.
type toolCheck struct {
	name      string
	available bool
	version   string // best-effort; empty if it couldn't be determined
}

func checkYtDlp() toolCheck {
	path, err := findExecutable()
	if err != nil {
		return toolCheck{name: "yt-dlp", available: false}
	}
	out, _ := exec.Command(path, "--version").Output()
	return toolCheck{name: "yt-dlp", available: true, version: strings.TrimSpace(string(out))}
}

func checkFFmpeg() toolCheck {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return toolCheck{name: "ffmpeg", available: false}
	}
	version := ""
	if out, err := exec.Command(path, "-version").Output(); err == nil {
		firstLine := strings.SplitN(string(out), "\n", 2)[0]
		fields := strings.Fields(firstLine)
		if len(fields) >= 3 {
			version = fields[2]
		}
	}
	return toolCheck{name: "ffmpeg", available: true, version: version}
}

// linuxArchSuffix maps a GOARCH value to the suffix yt-dlp/ffmpeg static
// release builds actually use, so the install hint points at the right file.
func linuxArchSuffix(goarch string) string {
	switch goarch {
	case "amd64":
		return "linux"
	case "arm64":
		return "linux_aarch64"
	case "arm":
		return "linux_armv7l"
	default:
		return "linux_" + goarch
	}
}

// installHintFor returns an OS/architecture-tailored one-liner for installing
// "yt-dlp" or "ffmpeg". Takes goos/goarch explicitly (rather than reading
// runtime.GOOS/GOARCH directly) so the logic can be exercised for every
// platform from a single build.
func installHintFor(goos, goarch, tool string) string {
	switch goos {
	case "darwin":
		return fmt.Sprintf("brew install %s", tool)
	case "windows":
		wingetID := map[string]string{"yt-dlp": "yt-dlp.yt-dlp", "ffmpeg": "Gyan.FFmpeg"}[tool]
		return fmt.Sprintf("winget install %s  (or: choco install %s)", wingetID, tool)
	default: // linux and other unix-likes
		switch tool {
		case "yt-dlp":
			return fmt.Sprintf(
				"sudo apt install yt-dlp  (or dnf/pacman/zypper; static build for your arch: https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_%s)",
				linuxArchSuffix(goarch))
		case "ffmpeg":
			return fmt.Sprintf(
				"sudo apt install ffmpeg  (or dnf/pacman/zypper; static %s build: https://johnvansickle.com/ffmpeg/)",
				linuxArchSuffix(goarch))
		default:
			return "install " + tool + " with your package manager"
		}
	}
}

func installHint(tool string) string {
	return installHintFor(runtime.GOOS, runtime.GOARCH, tool)
}

// buildYtDlpArgs turns a downloadConfig into yt-dlp CLI arguments. Format
// conversion is delegated entirely to yt-dlp's own postprocessors
// (--audio-format / --recode-video / --merge-output-format), which shell out
// to the system ffmpeg, rather than reimplementing conversion ourselves.
func buildYtDlpArgs(cfg downloadConfig) []string {
	args := []string{"--newline", "--progress", "-P", cfg.dir}

	if cfg.mode == "audio" {
		args = append(args, "-f", "bestaudio/best")
		// -x/--audio-format shells out to ffmpeg to extract/convert; without
		// it we're stuck with whatever container the source audio came in.
		if cfg.audioFormat != "best" && cfg.ffmpegAvailable {
			args = append(args, "--extract-audio", "--audio-format", cfg.audioFormat, "--audio-quality", "0")
		}
		return append(args, cfg.url)
	}

	if !cfg.ffmpegAvailable {
		// bestvideo+bestaudio requires ffmpeg to merge the two streams.
		// Without it, fall back to a single pre-muxed format so nothing
		// needs merging or converting at all.
		selector := "best/best"
		if cfg.videoQuality != "best" {
			selector = fmt.Sprintf("best[height<=%s]/best", cfg.videoQuality)
		}
		args = append(args, "-f", selector)
		return append(args, cfg.url)
	}

	formatSelector := "bestvideo+bestaudio/best"
	if cfg.videoQuality != "best" {
		formatSelector = fmt.Sprintf("bestvideo[height<=%s]+bestaudio/best", cfg.videoQuality)
	}
	args = append(args, "-f", formatSelector)

	switch cfg.videoFormat {
	case "none":
		// Keep whatever yt-dlp produces natively; no container conversion.
	case "mp4":
		// mp4 is the fast path: a lossless merge, no re-encode.
		args = append(args, "--merge-output-format", "mp4")
	default:
		// Any other target container gets a real ffmpeg re-encode so the
		// conversion always succeeds, even for codec/container combos that
		// a lossless remux would reject.
		args = append(args, "--recode-video", cfg.videoFormat)
	}

	return append(args, cfg.url)
}

func videoQualityLabel(q string) string {
	if q == "best" {
		return "Best available"
	}
	return q + "p"
}

func videoFormatLabel(f string) string {
	switch f {
	case "none":
		return "Original (no conversion)"
	case "mp4":
		return "MP4 (merge)"
	default:
		return strings.ToUpper(f) + " (convert)"
	}
}

func audioFormatLabel(f string) string {
	switch f {
	case "best":
		return "Best (original)"
	case "vorbis":
		return "OGG (Vorbis)"
	case "m4a":
		return "M4A (AAC)"
	default:
		return strings.ToUpper(f)
	}
}
