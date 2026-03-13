package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// --- Custom Messages ---
type logMsg string
type doneMsg struct{ err error }

type model struct {
	url           string
	quality       string
	mergeFFmpeg   bool
	status        string
	logLines      []string
	logViewport   viewport.Model
	running       bool
	width, height int
}

func initialModel(url, quality string, merge bool) model {
	vp := viewport.New(80, 10)
	vp.SetContent("Initializing stream...")

	return model{
		url:         url,
		quality:     quality,
		mergeFFmpeg: merge,
		status:      "Downloading...",
		logViewport: vp,
		running:     true,
	}
}

func (m model) Init() tea.Cmd {
	return runYtDlpCmd(m.url, m.quality, m.mergeFFmpeg)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.logViewport.Width = m.width - 4
		m.logViewport.Height = m.height - 8
		m.refreshViewport()

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}

	case logMsg:
		m.logLines = append(m.logLines, string(msg))
		m.refreshViewport()

	case doneMsg:
		m.running = false
		if msg.err != nil {
			m.status = "❌ Error: " + msg.err.Error()
		} else {
			m.status = "✅ Download Finished!"
		}
	}
	return m, nil
}

func (m *model) refreshViewport() {
	content := strings.Join(m.logLines, "\n")
	m.logViewport.SetContent(content)
	m.logViewport.GotoBottom()
}

func (m model) View() string {
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true).Render("YT-DLP Downloader")
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Render(m.status)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"Target: "+m.url,
		"Quality: "+m.quality,
		status,
		"",
		lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Render(m.logViewport.View()),
		lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(" Press 'q' to quit"),
	)
}

// --- Logic ---

var program *tea.Program

func runYtDlpCmd(url, quality string, merge bool) tea.Cmd {
	return func() tea.Msg {
		// Map user-friendly labels to yt-dlp format strings
		formatMap := map[string]string{
			"4K (2160p)": "bestvideo[height<=2160]+bestaudio/best",
			"1080p":      "bestvideo[height<=1080]+bestaudio/best",
			"720p":       "bestvideo[height<=720]+bestaudio/best",
			"Audio Only": "bestaudio/best",
		}

		args := []string{"--newline", "--progress", "-f", formatMap[quality], url}
		if merge && quality != "Audio Only" {
			args = append(args, "--merge-output-format", "mp4")
		}

		cmd := exec.Command("yt-dlp", args...)
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()
		reader := io.MultiReader(stdout, stderr)

		if err := cmd.Start(); err != nil {
			return doneMsg{err: err}
		}

		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			program.Send(logMsg(scanner.Text()))
		}

		return doneMsg{err: cmd.Wait()}
	}
}

func main() {
	var url string
	var quality string
	var merge bool = true

	// 1. Configuration Form using Huh
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Video URL").
				Value(&url).
				Placeholder("Paste link here...").
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("URL is required")
					}
					return nil
				}),

			huh.NewSelect[string]().
				Title("Select Quality").
				Options(
					huh.NewOption("4K (2160p)", "4K (2160p)"),
					huh.NewOption("1080p Full HD", "1080p"),
					huh.NewOption("720p HD", "720p"),
					huh.NewOption("Audio Only (Best MP3/M4A)", "Audio Only"),
				).
				Value(&quality),

			huh.NewConfirm().
				Title("Merge with FFmpeg?").
				Value(&merge),
		),
	)

	err := form.Run()
	if err != nil {
		fmt.Println("User cancelled.")
		return
	}

	// 2. Main Download TUI
	m := initialModel(url, quality, merge)
	program = tea.NewProgram(m, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
