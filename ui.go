package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// --- Palette ---
// A Charm-flavored pink/purple gradient with green/red status accents.
var (
	colorPink   = lipgloss.Color("#FF6AC1")
	colorPurple = lipgloss.Color("#7D56F4")
	colorGreen  = lipgloss.Color("#04B575")
	colorRed    = lipgloss.Color("#FF5F87")
	colorWarn   = lipgloss.Color("#FFB86C")
	colorMuted  = lipgloss.AdaptiveColor{Light: "#909090", Dark: "#6C6C6C"}
	colorSubtle = lipgloss.AdaptiveColor{Light: "#3C3C3C", Dark: "#DCDCDC"}
)

var (
	subtitleStyle   = lipgloss.NewStyle().Foreground(colorMuted).Italic(true)
	panelLabelStyle = lipgloss.NewStyle().Foreground(colorMuted).Width(9)
	panelValueStyle = lipgloss.NewStyle().Foreground(colorSubtle)
	statChipStyle   = lipgloss.NewStyle().Foreground(colorMuted)
	errorTextStyle  = lipgloss.NewStyle().Foreground(colorRed)
	okTextStyle     = lipgloss.NewStyle().Foreground(colorGreen)
	warnTextStyle   = lipgloss.NewStyle().Foreground(colorWarn)
)

// --- Keymap ---
var (
	keyQuitForm = key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit"))
	keyQuit     = key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit"))
	keyBack     = key.NewBinding(key.WithKeys("b", "esc"), key.WithHelp("b", "back"))
	keyClear    = key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "clear"))
)

// --- Session phases ---
const (
	phasePreparing      = "preparing"
	phaseDownloading    = "downloading"
	phasePostprocessing = "postprocessing"
	phaseDone           = "done"
	phaseError          = "error"
)

// gradientText renders s with each character's color linearly interpolated
// between from and to, for a Charm-style wordmark.
func gradientText(s string, from, to lipgloss.Color) string {
	runes := []rune(s)
	n := len(runes)
	if n == 0 {
		return ""
	}
	r1, g1, b1 := hexToRGB(string(from))
	r2, g2, b2 := hexToRGB(string(to))

	var sb strings.Builder
	steps := max(n-1, 1)
	for i, r := range runes {
		if r == ' ' {
			sb.WriteRune(r)
			continue
		}
		t := float64(i) / float64(steps)
		c := lipgloss.Color(fmt.Sprintf("#%02X%02X%02X",
			lerp(r1, r2, t), lerp(g1, g2, t), lerp(b1, b2, t)))
		sb.WriteString(lipgloss.NewStyle().Foreground(c).Bold(true).Render(string(r)))
	}
	return sb.String()
}

func hexToRGB(hex string) (r, g, b int) {
	hex = strings.TrimPrefix(hex, "#")
	v, _ := strconv.ParseInt(hex, 16, 32)
	return int(v >> 16 & 0xFF), int(v >> 8 & 0xFF), int(v & 0xFF)
}

func lerp(a, b int, t float64) int {
	return int(float64(a) + float64(b-a)*t)
}

// truncate shortens s to at most n runes, adding an ellipsis if cut.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func kv(label, value string) string {
	return panelLabelStyle.Render(label) + panelValueStyle.Render(value)
}

// toolStatusLines renders the yt-dlp/ffmpeg preflight check as a couple of
// styled lines for the config screen: green when found (with version if we
// have one), amber for a missing-but-non-fatal ffmpeg, red for a missing
// yt-dlp (nothing can be downloaded at all) — each with an install hint.
func toolStatusLines(yt, ff toolCheck) []string {
	var lines []string

	if yt.available {
		v := yt.version
		if v == "" {
			v = "detected"
		}
		lines = append(lines, okTextStyle.Render("✓ ")+"yt-dlp "+v)
	} else {
		lines = append(lines, errorTextStyle.Render("✗ ")+"yt-dlp not found — install: "+installHint("yt-dlp"))
	}

	if ff.available {
		label := "ffmpeg"
		if ff.version != "" {
			label += " " + ff.version
		}
		label += " — format conversion available"
		lines = append(lines, okTextStyle.Render("✓ ")+label)
	} else {
		lines = append(lines, warnTextStyle.Render("⚠ ")+
			"ffmpeg not found — conversion disabled, original format only. Install: "+installHint("ffmpeg"))
	}

	return lines
}

// --- yt-dlp progress line parsing ---

// e.g. "[download]  57.9% of  118.62MiB at   16.17MiB/s ETA 00:03"
var progressRe = regexp.MustCompile(`^\[download\]\s+([\d.]+)%\s+of\s+(\S+)\s+at\s+(.+?)\s+ETA\s+(\S+)`)

// e.g. "[download] 100% of  246.27KiB in 00:00:00 at 2.11MiB/s" (final summary line)
var progressDoneRe = regexp.MustCompile(`^\[download\]\s+([\d.]+)%\s+of\s+(\S+)\s+in\s+(\S+)\s+at\s+(\S+)`)

type progressInfo struct {
	percent float64
	size    string
	speed   string
	eta     string
}

func parseProgressLine(line string) (progressInfo, bool) {
	if m := progressRe.FindStringSubmatch(line); m != nil {
		pct, _ := strconv.ParseFloat(m[1], 64)
		return progressInfo{percent: pct, size: m[2], speed: m[3], eta: m[4]}, true
	}
	if m := progressDoneRe.FindStringSubmatch(line); m != nil {
		pct, _ := strconv.ParseFloat(m[1], 64)
		return progressInfo{percent: pct, size: m[2], speed: m[4], eta: "00:00"}, true
	}
	return progressInfo{}, false
}
