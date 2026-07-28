package main

import (
	"fmt"
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
}

// buildYtDlpArgs turns a downloadConfig into yt-dlp CLI arguments. Format
// conversion is delegated entirely to yt-dlp's own postprocessors
// (--audio-format / --recode-video / --merge-output-format), which shell out
// to the system ffmpeg, rather than reimplementing conversion ourselves.
func buildYtDlpArgs(cfg downloadConfig) []string {
	args := []string{"--newline", "--progress", "-P", cfg.dir}

	if cfg.mode == "audio" {
		args = append(args, "-f", "bestaudio/best")
		if cfg.audioFormat != "best" {
			args = append(args, "--extract-audio", "--audio-format", cfg.audioFormat, "--audio-quality", "0")
		}
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
