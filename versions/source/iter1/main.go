package main

import (
	"bufio"
	"context"
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

type logMsg string

type doneMsg struct {
	logs []string
	err  error
}

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
	ti.Placeholder = "https://example.com/video"
	ti.Focus()
	ti.CharLimit = 0
	ti.Prompt = "URL > "
	ti.Width = 80

	vp := viewport.New(80, 20)
	vp.SetContent("Logs will appear here after running a download.")

	return model{
		urlInput:    ti,
		mergeFFmpeg: true,
		status:      "Ready",
		logLines:    []string{},
		logViewport: vp,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.logViewport.Width = m.width - 4
		m.logViewport.Height = m.height - 10
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
			if m.running {
				return m, nil
			}
			url := strings.TrimSpace(m.urlInput.Value())
			if url == "" {
				m.appendLog("Please enter a URL before starting.")
				return m, nil
			}
			m.running = true
			m.status = "Running yt-dlp..."
			m.logLines = nil
			m.refreshViewport()
			return m, runYtDlpCmd(url, m.mergeFFmpeg)
		}
	case logMsg:
		m.appendLog(string(msg))
	case doneMsg:
		m.running = false
		m.appendLogs(msg.logs)
		if msg.err != nil {
			m.status = "Failed: " + msg.err.Error()
			m.appendLog("Error: " + msg.err.Error())
		} else {
			m.status = "Done"
			m.appendLog("Download complete.")
		}
	}
	var cmd tea.Cmd
	m.urlInput, cmd = m.urlInput.Update(msg)
	return m, cmd
}

func (m *model) appendLog(line string) {
	m.logLines = append(m.logLines, line)
	m.refreshViewport()
}

func (m *model) appendLogs(lines []string) {
	m.logLines = append(m.logLines, lines...)
	m.refreshViewport()
}

func (m *model) refreshViewport() {
	content := strings.Join(m.logLines, "\n")
	if content == "" {
		content = "Waiting for output..."
	}
	m.logViewport.SetContent(content)
	m.logViewport.GotoBottom()
}

func (m model) View() string {
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true).
		Render("yt-dlp TUI Wrapper")
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Render(m.status)

	merge := "[ ]"
	if m.mergeFFmpeg {
		merge = "[x]"
	}
	mergeLine := fmt.Sprintf("%s Merge with ffmpeg (space to toggle)", merge)

	instructions := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(
		"Enter URL and press Enter to start • q or Ctrl+C to quit",
	)

	logBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1).
		Render(m.logViewport.View())

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		status,
		m.urlInput.View(),
		mergeLine,
		instructions,
		"Logs:",
		logBox,
	)
}

func runYtDlpCmd(url string, merge bool) tea.Cmd {
	return func() tea.Msg {
		args := []string{
			"--newline",
			"-f", "bv*+ba/b",
		}
		if merge {
			args = append(args, "--merge-output-format", "mp4")
		}
		args = append(args, url)

		cmd := exec.CommandContext(context.Background(), "yt-dlp", args...)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return doneMsg{err: err}
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return doneMsg{err: err}
		}

		if err := cmd.Start(); err != nil {
			return doneMsg{err: err}
		}

		reader := bufio.NewScanner(io.MultiReader(stdout, stderr))
		reader.Buffer(make([]byte, 1024), 1024*1024)

		var logs []string
		for reader.Scan() {
			logs = append(logs, reader.Text())
		}
		if err := reader.Err(); err != nil {
			logs = append(logs, "read error: "+err.Error())
		}

		err = cmd.Wait()
		return doneMsg{logs: logs, err: err}
	}
}

func main() {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		fmt.Println("yt-dlp not found in PATH. Please install yt-dlp and ensure it's accessible.")
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Println("error running program:", err)
		os.Exit(1)
	}
}
