package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// version is set at build time via -ldflags "-X main.version=vX.Y.Z"
var version = "dev"

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
	state       sessionState
	form        *huh.Form
	url         string
	quality     string
	outputDir   string
	mergeFFmpeg bool
	status      string

	phase              string
	percent            float64
	speed, eta, size   string
	sawDownloadPercent bool

	logLines      []string
	logViewport   viewport.Model
	spinnerModel  spinner.Model
	progressModel progress.Model
	help          help.Model
	width, height int
}

var (
	defaultQuality = "1080p"
	defaultMerge   = true
	defaultDir, _  = os.Getwd()
)

// runningCmd tracks the in-flight yt-dlp process so it can be killed if the
// user quits mid-download instead of leaving it orphaned in the background.
var (
	runningCmd   *exec.Cmd
	runningCmdMu sync.Mutex
)

func setRunningCmd(cmd *exec.Cmd) {
	runningCmdMu.Lock()
	runningCmd = cmd
	runningCmdMu.Unlock()
}

func killRunningCmd() {
	runningCmdMu.Lock()
	defer runningCmdMu.Unlock()
	if runningCmd != nil && runningCmd.Process != nil {
		_ = runningCmd.Process.Kill()
	}
	runningCmd = nil
}

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
	sp := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(colorPink)),
	)
	pm := progress.New(
		progress.WithGradient(string(colorPink), string(colorPurple)),
		progress.WithWidth(40),
	)
	return model{
		state:         stateConfig,
		form:          newConfigForm(),
		logViewport:   vp,
		spinnerModel:  sp,
		progressModel: pm,
		help:          help.New(),
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
		m.logViewport.Height = m.height - 18
		if m.logViewport.Height < 4 {
			m.logViewport.Height = 4
		}
		contentWidth := m.width - 8
		if contentWidth < 20 {
			contentWidth = 20
		}
		m.progressModel.Width = contentWidth
		m.help.Width = m.width
		m.refreshViewport()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			killRunningCmd()
			return m, tea.Quit
		case "q":
			// If not in the form, 'q' quits immediately
			if m.state != stateConfig {
				killRunningCmd()
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

	case spinner.TickMsg:
		if m.state == stateDownloading && (m.phase == phasePreparing || m.phase == phasePostprocessing) {
			var cmd tea.Cmd
			m.spinnerModel, cmd = m.spinnerModel.Update(msg)
			cmds = append(cmds, cmd)
		}

	case progress.FrameMsg:
		newProgress, cmd := m.progressModel.Update(msg)
		if pm, ok := newProgress.(progress.Model); ok {
			m.progressModel = pm
		}
		cmds = append(cmds, cmd)

	case logMsg:
		line := string(msg)
		m.logLines = append(m.logLines, line)
		m.refreshViewport()

		if info, ok := parseProgressLine(line); ok {
			m.sawDownloadPercent = true
			m.phase = phaseDownloading
			m.percent = info.percent
			m.size, m.speed, m.eta = info.size, info.speed, info.eta
			cmds = append(cmds, m.progressModel.SetPercent(info.percent/100))
		} else if m.sawDownloadPercent && m.phase == phaseDownloading && m.percent >= 99.9 {
			m.phase = phasePostprocessing
			cmds = append(cmds, spinner.Tick)
		}

	case doneMsg:
		m.state = stateFinished
		notifySound()
		if msg.err != nil {
			m.phase = phaseError
			m.status = msg.err.Error()
		} else {
			m.phase = phaseDone
			m.status = "Download finished"
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
			m.phase = phasePreparing
			m.percent = 0
			m.speed, m.eta, m.size = "", "", ""
			m.sawDownloadPercent = false
			m.logLines = nil
			return m, tea.Batch(
				runYtDlpCmd(m.url, m.quality, m.mergeFFmpeg, m.outputDir),
				spinner.Tick,
				m.progressModel.SetPercent(0),
			)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *model) refreshViewport() {
	m.logViewport.SetContent(strings.Join(m.logLines, "\n"))
	m.logViewport.GotoBottom()
}

func (m model) View() string {
	logo := gradientText("YT-DLP GUI", colorPink, colorPurple)

	if m.state == stateConfig {
		header := lipgloss.JoinVertical(lipgloss.Left,
			logo,
			subtitleStyle.Render("Download videos, beautifully."),
		)

		footer := m.help.ShortHelpView([]key.Binding{keyQuitForm})

		return lipgloss.NewStyle().Padding(1, 2).Render(
			lipgloss.JoinVertical(lipgloss.Left, header, "", m.form.View(), footer),
		)
	}

	border := colorPurple
	statusIcon := m.spinnerModel.View()
	statusText := "Preparing…"
	switch m.phase {
	case phaseDownloading:
		statusIcon = ""
		statusText = fmt.Sprintf("Downloading… %.1f%%", m.percent)
	case phasePostprocessing:
		statusText = "Finishing up (merging/postprocessing)…"
	case phaseDone:
		border = colorGreen
		statusIcon = lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("✓")
		statusText = "Download finished"
	case phaseError:
		border = colorRed
		statusIcon = lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render("✗")
		statusText = errorTextStyle.Render("Error: " + m.status)
	}
	statusLine := lipgloss.JoinHorizontal(lipgloss.Top, statusIcon, " ", statusText)

	// Match the log panel's outer width (border only, no padding: m.width-4
	// content + 2 border cols) so the two panels line up as a single stack.
	infoContentWidth := m.width - 4
	if infoContentWidth < 20 {
		infoContentWidth = 20
	}
	valueWidth := infoContentWidth - 11
	if valueWidth < 10 {
		valueWidth = 10
	}
	infoPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorMuted).
		Padding(0, 2).
		Width(infoContentWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			kv("URL", truncate(m.url, valueWidth)),
			kv("Quality", m.quality)+"   "+kv("Merge", yesNo(m.mergeFFmpeg)),
			kv("Output", truncate(m.outputDir, valueWidth)),
		))

	var progressBlock string
	if m.phase == phaseDownloading || m.phase == phasePostprocessing || m.phase == phaseDone {
		lines := []string{m.progressModel.View()}
		if m.speed != "" {
			lines = append(lines, statChipStyle.Render(
				fmt.Sprintf("%s  •  ETA %s  •  %s", m.speed, m.eta, m.size)))
		}
		progressBlock = lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	logPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Render(m.logViewport.View())

	bindings := []key.Binding{keyQuit, keyClear}
	if m.state == stateFinished {
		bindings = append(bindings, keyBack)
	}
	footer := m.help.ShortHelpView(bindings)

	body := lipgloss.JoinVertical(lipgloss.Left,
		logo,
		statusLine,
		"",
		infoPanel,
		"",
		progressBlock,
		"",
		logPanel,
		footer,
	)

	return lipgloss.NewStyle().Padding(1, 2).Render(body)
}

func notifySound() {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("afplay"); err == nil {
			_ = exec.Command("afplay", "/System/Library/Sounds/Glass.aiff").Start()
			return
		}
		if _, err := exec.LookPath("osascript"); err == nil {
			_ = exec.Command("osascript", "-e", `display notification "Download finished" with title "yt-dlp-GUI"`).Start()
			return
		}
	case "linux":
		sounds := []string{
			"/usr/share/sounds/freedesktop/stereo/complete.oga",
			"/usr/share/sounds/freedesktop/stereo/bell.oga",
		}
		for _, player := range []string{"paplay", "aplay", "play"} {
			playerPath, err := exec.LookPath(player)
			if err != nil {
				continue
			}
			for _, sound := range sounds {
				if _, err := os.Stat(sound); err == nil {
					_ = exec.Command(playerPath, sound).Start()
					return
				}
			}
		}
	case "windows":
		if path, err := exec.LookPath("powershell"); err == nil {
			_ = exec.Command(path, "-NoProfile", "-Command", "[console]::beep(800,300)").Start()
			return
		}
	}
	// Final fallback: terminal bell.
	fmt.Print("\a")
}

func findExecutable() (string, error) {
	for _, name := range []string{"yt-dlp", "yt_dlp", "youtube-dl"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	switch runtime.GOOS {
	case "darwin":
		return "", fmt.Errorf("yt-dlp not found in PATH; install it with: brew install yt-dlp")
	case "windows":
		return "", fmt.Errorf("yt-dlp not found in PATH; install yt-dlp.exe and add it to PATH (see https://github.com/yt-dlp/yt-dlp#installation)")
	default:
		return "", fmt.Errorf("yt-dlp not found in PATH; install it with your package manager, e.g.: sudo apt install yt-dlp")
	}
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
		setRunningCmd(cmd)

		// Read stdout and stderr concurrently: reading them sequentially via
		// io.MultiReader would block on stdout until the process exits before
		// ever draining stderr, which can also deadlock the child if its
		// stderr pipe buffer fills up while nothing is reading it.
		var wg sync.WaitGroup
		streamLines := func(r io.Reader) {
			defer wg.Done()
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				p.Send(logMsg(scanner.Text()))
			}
		}
		wg.Add(2)
		go streamLines(stdout)
		go streamLines(stderr)
		wg.Wait()

		waitErr := cmd.Wait()
		setRunningCmd(nil)
		return doneMsg{err: waitErr}
	}
}

var p *tea.Program

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("yt-dlp-gui " + version)
		return
	}

	m := initialModel()
	p = tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
