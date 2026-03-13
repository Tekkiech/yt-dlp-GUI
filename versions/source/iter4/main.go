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

type sessionState int

const (
	stateConfig sessionState = iota
	stateDownloading
	stateFinished
)

type model struct {
	state         sessionState
	form          *huh.Form
	url           string
	quality       string
	mergeFFmpeg   bool
	status        string
	logLines      []string
	logViewport   viewport.Model
	width, height int
}

// Global default variables to satisfy huh's pointer requirements
var (
	defaultQuality = "1080p"
	defaultMerge   = true
)

func newConfigForm() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("url").
				Title("Video URL").
				Placeholder("https://www.youtube.com/watch?...").
				Validate(func(str string) error {
					if str == "" {
						return fmt.Errorf("URL cannot be empty")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Key("quality").
				Title("Select Quality").
				Options(
					huh.NewOption("4K (2160p)", "4K (2160p)"),
					huh.NewOption("1080p Full HD", "1080p"),
					huh.NewOption("720p HD", "720p"),
					huh.NewOption("Audio Only", "Audio Only"),
				).
				Value(&defaultQuality), // Pointer fix
			huh.NewConfirm().
				Key("merge").
				Title("Merge with FFmpeg?").
				Value(&defaultMerge), // Pointer fix
		),
	).WithTheme(huh.ThemeCharm())
}

func initialModel() model {
	vp := viewport.New(80, 10)
	return model{
		state:       stateConfig,
		form:        newConfigForm(),
		logViewport: vp,
	}
}

func (m model) Init() tea.Cmd {
	return m.form.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.logViewport.Width = m.width - 4
		m.logViewport.Height = m.height - 12
		m.refreshViewport()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			// Allow 'q' to quit during download or when finished
			if m.state != stateConfig {
				return m, tea.Quit
			}
		case "c": // Clear Logs
			if m.state != stateConfig {
				m.logLines = nil
				m.refreshViewport()
				return m, nil
			}
		case "b", "esc": // Return to Config
			if m.state == stateFinished {
				m.state = stateConfig
				m.form = newConfigForm()
				return m, m.form.Init()
			}
		}

	case logMsg:
		m.logLines = append(m.logLines, string(msg))
		m.refreshViewport()

	case doneMsg:
		m.state = stateFinished
		if msg.err != nil {
			m.status = "❌ Error: " + msg.err.Error()
		} else {
			m.status = "✅ Download Finished!"
		}
	}

	// Logic Switch based on State
	if m.state == stateConfig {
		form, cmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
			cmds = append(cmds, cmd)
		}

		if m.form.State == huh.StateCompleted {
			m.url = m.form.GetString("url")
			m.quality = m.form.GetString("quality")
			m.mergeFFmpeg = m.form.GetBool("merge")
			m.state = stateDownloading
			m.status = "Downloading..."
			m.logLines = nil
			return m, runYtDlpCmd(m.url, m.quality, m.mergeFFmpeg)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *model) refreshViewport() {
	content := strings.Join(m.logLines, "\n")
	m.logViewport.SetContent(content)
	m.logViewport.GotoBottom()
}

func (m model) View() string {
	if m.state == stateConfig {
		return lipgloss.NewStyle().Padding(1, 2).Render(m.form.View())
	}

	header := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true).Render("YT-DLP Active Session")
	info := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fmt.Sprintf("URL: %s | Quality: %s", m.url, m.quality))

	footer := " [q] Quit "
	if m.state == stateFinished {
		footer += "• [b] Back to Menu "
	}
	footer += "• [c] Clear Logs"

	ui := lipgloss.JoinVertical(lipgloss.Left,
		header,
		info,
		"Status: "+m.status,
		"",
		lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8")).Render(m.logViewport.View()),
		lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(footer),
	)

	return lipgloss.NewStyle().Padding(1, 2).Render(ui)
}

// --- Logic ---

func runYtDlpCmd(url, quality string, merge bool) tea.Cmd {
	return func() tea.Msg {
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
			// We use p.Send to stream logs back to the main loop
			p.Send(logMsg(scanner.Text()))
		}

		return doneMsg{err: cmd.Wait()}
	}
}

var p *tea.Program

func main() {
	// Pre-flight check
	for _, bin := range []string{"yt-dlp", "ffmpeg"} {
		if _, err := exec.LookPath(bin); err != nil {
			fmt.Printf("Error: %s not found in PATH.\n", bin)
			os.Exit(1)
		}
	}

	m := initialModel()
	p = tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
