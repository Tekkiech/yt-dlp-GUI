package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Custom Messages ---
type logMsg string
type doneMsg struct{ err error }

// --- Model ---
type model struct {
	urlInput      textinput.Model
	mergeFFmpeg   bool
	status        string
	logLines      []string
	logViewport   viewport.Model
	running       bool
	width, height int
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "https://www.youtube.com/watch?v=..."
	ti.Focus()
	ti.Prompt = "🔗 URL: "
	ti.Width = 60

	vp := viewport.New(80, 10)
	vp.SetContent("Terminal output will appear here...")

	return model{
		urlInput:    ti,
		mergeFFmpeg: true,
		status:      "Ready to download",
		logViewport: vp,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.logViewport.Width = m.width - 4
		m.logViewport.Height = m.height - 12 // Adjust based on header height
		m.refreshViewport()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case " ":
			if !m.running {
				m.mergeFFmpeg = !m.mergeFFmpeg
			}
		case "enter":
			if !m.running && strings.TrimSpace(m.urlInput.Value()) != "" {
				m.running = true
				m.status = "Downloading..."
				m.logLines = nil // Clear previous logs
				// Note: We pass the tea.Program pointer later in main
				return m, runYtDlpCmd(m.urlInput.Value(), m.mergeFFmpeg)
			}
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

	m.urlInput, cmd = m.urlInput.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *model) refreshViewport() {
	content := strings.Join(m.logLines, "\n")
	if content == "" {
		content = "Waiting for yt-dlp..."
	}
	m.logViewport.SetContent(content)
	m.logViewport.GotoBottom()
}

// --- View ---
var (
	titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true).Padding(0, 1)
	infoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	boxStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8"))
)

func (m model) View() string {
	mergeCheckbox := "[ ]"
	if m.mergeFFmpeg {
		mergeCheckbox = "[x]"
	}

	ui := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("YT-DLP TUI Wrapper"),
		"Status: "+m.status,
		"",
		m.urlInput.View(),
		fmt.Sprintf("%s Merge with FFmpeg (Press Space to toggle)", mergeCheckbox),
		"",
		infoStyle.Render("Press Enter to start • q to quit"),
		"",
		"Logs:",
		boxStyle.Render(m.logViewport.View()),
	)

	return lipgloss.NewStyle().Padding(1, 2).Render(ui)
}

// --- yt-dlp Execution ---
// We use a global variable to store the program instance for p.Send()
var program *tea.Program

func runYtDlpCmd(url string, merge bool) tea.Cmd {
	return func() tea.Msg {
		args := []string{"--newline", "--progress", "-f", "bv*+ba/b", url}
		if merge {
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
			// Real-time streaming to the UI
			program.Send(logMsg(scanner.Text()))
		}

		err := cmd.Wait()
		return doneMsg{err: err}
	}
}

func main() {
	// Check dependencies
	for _, bin := range []string{"yt-dlp", "ffmpeg"} {
		if _, err := exec.LookPath(bin); err != nil {
			fmt.Printf("Error: %s not found. Please install it to continue.\n", bin)
			os.Exit(1)
		}
	}

	m := initialModel()
	program = tea.NewProgram(m, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
