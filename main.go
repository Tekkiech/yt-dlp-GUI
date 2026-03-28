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
	outputDir     string
	mergeFFmpeg   bool
	status        string
	logLines      []string
	logViewport   viewport.Model
	width, height int
}

var (
	defaultQuality = "1080p"
	defaultMerge   = true
	defaultDir, _  = os.Getwd()
)

func newConfigForm() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("url").
				Title("Video URL").
				Placeholder("https://...").
				Validate(func(str string) error {
					if str == "" {
						return fmt.Errorf("URL required")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Key("quality").
				Title("Quality").
				Options(
					huh.NewOption("4K", "4K (2160p)"),
					huh.NewOption("1080p", "1080p"),
					huh.NewOption("720p", "720p"),
					huh.NewOption("Audio", "Audio Only"),
				).
				Value(&defaultQuality),
			huh.NewFilePicker().
				Key("dir").
				Title("Select Download Directory").
				CurrentDirectory(defaultDir).
				DirAllowed(true).
				FileAllowed(false).
				Value(&defaultDir),
			huh.NewConfirm().
				Key("merge").
				Title("Merge with FFmpeg?").
				Value(&defaultMerge),
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
		m.logViewport.Height = m.height - 14
		m.refreshViewport()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			// If not in the form, 'q' quits immediately
			if m.state != stateConfig {
				return m, tea.Quit
			}
		case "c":
			if m.state != stateConfig {
				m.logLines = nil
				m.refreshViewport()
			}
		case "b", "esc":
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
		notifySound()
		if msg.err != nil {
			m.status = "❌ Error: " + msg.err.Error()
		} else {
			m.status = "✅ Download Finished!"
		}
	}

	if m.state == stateConfig {
		form, cmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
			cmds = append(cmds, cmd)
		}

		// Handle explicit exit/abortion from the form screen
		if m.form.State == huh.StateAborted {
			return m, tea.Quit
		}

		if m.form.State == huh.StateCompleted {
			m.url = m.form.GetString("url")
			m.quality = m.form.GetString("quality")
			m.mergeFFmpeg = m.form.GetBool("merge")
			m.outputDir = m.form.GetString("dir")
			m.state = stateDownloading
			m.status = "Downloading..."
			m.logLines = nil
			return m, runYtDlpCmd(m.url, m.quality, m.mergeFFmpeg, m.outputDir)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *model) refreshViewport() {
	m.logViewport.SetContent(strings.Join(m.logLines, "\n"))
	m.logViewport.GotoBottom()
}

func (m model) View() string {
	if m.state == stateConfig {
		// Footer added to form view for clarity
		return lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Padding(1, 2).Render(m.form.View()),
			lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("8")).Render("Press ctrl+c to quit"),
		)
	}

	header := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true).Render("YT-DLP Session")
	info := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(
		fmt.Sprintf("URL: %s\nQuality: %s | Dir: %s", m.url, m.quality, m.outputDir),
	)

	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(" [q] Quit • [b] Back • [c] Clear")

	ui := lipgloss.JoinVertical(lipgloss.Left,
		header,
		info,
		"Status: "+m.status,
		"",
		lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8")).Render(m.logViewport.View()),
		footer,
	)

	return lipgloss.NewStyle().Padding(1, 2).Render(ui)
}

func notifySound() {
	// macOS-only: prefer `afplay` to play the system sound. If not present,
	// fall back to `osascript` notification, then terminal bell.
	if _, err := exec.LookPath("afplay"); err == nil {
		_ = exec.Command("afplay", "/System/Library/Sounds/Glass.aiff").Start()
		return
	}
	// If afplay isn't available, try showing a notification with osascript.
	if _, err := exec.LookPath("osascript"); err == nil {
		_ = exec.Command("osascript", "-e", "display notification \"Download finished\" with title \"yt-dlp-GUI\"").Start()
		return
	}
	// Final fallback: terminal bell.
	fmt.Print("\a")
}

func findExecutable() (string, error) {
	// macOS-only: require the official yt-dlp binary in PATH.
	if path, err := exec.LookPath("yt-dlp"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("yt-dlp not found in PATH; please install via Homebrew: brew install yt-dlp")
}

func runYtDlpCmd(url, quality string, merge bool, dir string) tea.Cmd {
	return func() tea.Msg {
		formatMap := map[string]string{
			"4K (2160p)": "bestvideo[height<=2160]+bestaudio/best",
			"1080p":      "bestvideo[height<=1080]+bestaudio/best",
			"720p":       "bestvideo[height<=720]+bestaudio/best",
			"Audio Only": "bestaudio/best",
		}

		// macOS-only: require the official yt-dlp binary in PATH.
		exe, err := findExecutable()
		if err != nil {
			return doneMsg{err: err}
		}

		args := []string{"--newline", "--progress", "-P", dir, "-f", formatMap[quality], url}
		if merge && quality != "Audio Only" {
			args = append(args, "--merge-output-format", "mp4")
		}

		cmd := exec.Command(exe, args...)
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()
		reader := io.MultiReader(stdout, stderr)

		if err := cmd.Start(); err != nil {
			return doneMsg{err: err}
		}

		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			p.Send(logMsg(scanner.Text()))
		}

		return doneMsg{err: cmd.Wait()}
	}
}

var p *tea.Program

func main() {
	m := initialModel()
	p = tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
