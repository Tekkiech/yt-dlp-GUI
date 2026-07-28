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
	state  sessionState
	form   *huh.Form
	config downloadConfig
	status string

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
	defaultMode         = "video"
	defaultVideoQuality = "1080"
	defaultVideoFormat  = "mp4"
	defaultAudioFormat  = "best"
	defaultDir, _       = os.Getwd()
)

// Preflight tool checks, populated once in main() before the program starts.
var (
	ytdlpCheck  toolCheck
	ffmpegCheck toolCheck
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
	videoFormatOptions := []huh.Option[string]{
		huh.NewOption("Original (no merge)", "none"),
	}
	videoFormatDescription := "ffmpeg not found: only a single pre-muxed file is available (no merge/convert)."
	if ffmpegCheck.available {
		videoFormatOptions = []huh.Option[string]{
			huh.NewOption("MP4 (merge, fast)", "mp4"),
			huh.NewOption("MKV (convert)", "mkv"),
			huh.NewOption("WebM (convert)", "webm"),
			huh.NewOption("MOV (convert)", "mov"),
			huh.NewOption("AVI (convert)", "avi"),
			huh.NewOption("Original (no merge)", "none"),
		}
		videoFormatDescription = "Anything other than MP4 is re-encoded with ffmpeg."
	}

	audioFormatOptions := []huh.Option[string]{
		huh.NewOption("Best (original)", "best"),
	}
	audioFormatDescription := "ffmpeg not found: only the original audio codec is available (no conversion)."
	if ffmpegCheck.available {
		audioFormatOptions = []huh.Option[string]{
			huh.NewOption("Best (original)", "best"),
			huh.NewOption("MP3", "mp3"),
			huh.NewOption("FLAC", "flac"),
			huh.NewOption("WAV", "wav"),
			huh.NewOption("OGG (Vorbis)", "vorbis"),
			huh.NewOption("M4A (AAC)", "m4a"),
			huh.NewOption("Opus", "opus"),
			huh.NewOption("ALAC", "alac"),
		}
		audioFormatDescription = "Converted with ffmpeg via yt-dlp's audio extractor."
	}

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
				Key("mode").
				Title("Download").
				Options(
					huh.NewOption("Video", "video"),
					huh.NewOption("Audio only", "audio"),
				).
				Value(&defaultMode),
			huh.NewFilePicker().
				Key("dir").
				Title("Select Download Directory").
				CurrentDirectory(defaultDir).
				DirAllowed(true).
				FileAllowed(false).
				Value(&defaultDir),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Key("videoQuality").
				Title("Video Quality").
				Options(
					huh.NewOption("4K (2160p)", "2160"),
					huh.NewOption("1080p", "1080"),
					huh.NewOption("720p", "720"),
					huh.NewOption("Best available", "best"),
				).
				Value(&defaultVideoQuality),
			huh.NewSelect[string]().
				Key("videoFormat").
				Title("Output Format").
				Description(videoFormatDescription).
				Options(videoFormatOptions...).
				Value(&defaultVideoFormat),
		).WithHideFunc(func() bool { return defaultMode != "video" }),
		huh.NewGroup(
			huh.NewSelect[string]().
				Key("audioFormat").
				Title("Audio Format").
				Description(audioFormatDescription).
				Options(audioFormatOptions...).
				Value(&defaultAudioFormat),
		).WithHideFunc(func() bool { return defaultMode != "audio" }),
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
			m.config = downloadConfig{
				url:             m.form.GetString("url"),
				dir:             m.form.GetString("dir"),
				mode:            m.form.GetString("mode"),
				videoQuality:    defaultVideoQuality,
				videoFormat:     defaultVideoFormat,
				audioFormat:     defaultAudioFormat,
				ffmpegAvailable: ffmpegCheck.available,
			}
			m.state = stateDownloading
			m.status = "Downloading..."
			m.phase = phasePreparing
			m.percent = 0
			m.speed, m.eta, m.size = "", "", ""
			m.sawDownloadPercent = false
			m.logLines = nil
			return m, tea.Batch(
				runYtDlpCmd(m.config),
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
		statusLines := toolStatusLines(ytdlpCheck, ffmpegCheck)
		header := lipgloss.JoinVertical(lipgloss.Left,
			append([]string{logo, subtitleStyle.Render("Download videos, beautifully."), ""}, statusLines...)...,
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
	var formatLine string
	if m.config.mode == "audio" {
		formatLine = kv("Format", audioFormatLabel(m.config.audioFormat))
	} else {
		formatLine = kv("Quality", videoQualityLabel(m.config.videoQuality)) +
			"   " + kv("Format", videoFormatLabel(m.config.videoFormat))
	}
	infoPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorMuted).
		Padding(0, 2).
		Width(infoContentWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			kv("URL", truncate(m.config.url, valueWidth)),
			formatLine,
			kv("Output", truncate(m.config.dir, valueWidth)),
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

func runYtDlpCmd(cfg downloadConfig) tea.Cmd {
	return func() tea.Msg {
		exe, err := findExecutable()
		if err != nil {
			return doneMsg{err: err}
		}

		cmd := exec.Command(exe, buildYtDlpArgs(cfg)...)
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

	ytdlpCheck = checkYtDlp()
	ffmpegCheck = checkFFmpeg()
	if !ffmpegCheck.available {
		// mp4/mkv/etc. and every non-"best" audio format require ffmpeg;
		// fall back to the one option that doesn't so the pre-selected
		// default always matches an option actually offered in the form.
		defaultVideoFormat = "none"
		defaultAudioFormat = "best"
	}

	m := initialModel()
	p = tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
