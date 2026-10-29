package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const syscletModule = "github.com/Nomadcxx/sysclet@latest"

var asciiHeaderLines = []string{
	"  ██████████████░████░       ████░   ██████████████░  ██████████████░",
	"████████████████░████░       ████░ ████████████████░████████████████░",
	"████░            ██████░   ██████░ ████░            ████░            ",
	"██████████████░    ████████████░   ██████████████░  ████░            ",
	"  ██████████████░    ████████░       ██████████████░████░            ",
	"            ████░      ████░                   ████░████░            ",
	"████████████████░      ████░       ████████████████░████████████████░",
	"██████████████░        ████░       ██████████████░    ██████████████░",
	"           //////////////SEE YOU IN SPACE COWBOY//////////",
}

var (
	headerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#3434ff")).Bold(true)
	titleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true)
	accentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00c9e8"))
	mutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#8b8b9a"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#37f499"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#f16c75"))
)

type installStartedMsg struct {
	stream *installStream
	err    error
}

type installOutputMsg struct {
	line string
}

type installDoneMsg struct {
	err     error
	version string
}

type installStream struct {
	pipe   *io.PipeReader
	reader *bufio.Reader
	done   <-chan error
	binary string
}

type model struct {
	ctx        context.Context
	cancel     context.CancelFunc
	installDir string
	spinner    spinner.Model
	installing bool
	done       bool
	err        error
	output     string
	stream     *installStream
	width      int
	height     int
}

func main() {
	os.Exit(run())
}

func run() int {
	if os.Geteuid() == 0 {
		fmt.Fprintln(os.Stderr, "sysclet installer: run this as your user, without sudo")
		return 1
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sysclet installer: resolve home directory: %v\n", err)
		return 1
	}
	installDir, err := userInstallDir(home)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sysclet installer: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	program := tea.NewProgram(newModel(ctx, cancel, installDir), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "sysclet installer: %v\n", err)
		return 1
	}
	return 0
}

func userInstallDir(home string) (string, error) {
	if home == "" || !filepath.IsAbs(home) {
		return "", fmt.Errorf("home directory %q is not an absolute path", home)
	}
	return filepath.Join(home, ".local", "bin"), nil
}

func newModel(ctx context.Context, cancel context.CancelFunc, installDir string) model {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = accentStyle
	return model{ctx: ctx, cancel: cancel, installDir: installDir, spinner: spin}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.installing {
				m.cancel()
				if m.stream != nil {
					_ = m.stream.pipe.Close()
				}
			}
			return m, tea.Quit
		case "q", "esc":
			if !m.installing {
				return m, tea.Quit
			}
		case "enter":
			if m.installing {
				return m, nil
			}
			if m.done {
				return m, tea.Quit
			}
			m.installing = true
			m.output = ""
			return m, tea.Batch(m.spinner.Tick, installCmd(m.ctx, m.installDir))
		}
	case spinner.TickMsg:
		if m.installing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case installStartedMsg:
		if msg.err != nil {
			m.installing = false
			m.done = true
			m.err = msg.err
			return m, nil
		}
		m.stream = msg.stream
		return m, readInstallOutputCmd(m.stream)
	case installOutputMsg:
		if msg.line != "" {
			m.output = appendInstallOutput(m.output, ansi.Strip(msg.line))
		}
		return m, readInstallOutputCmd(m.stream)
	case installDoneMsg:
		m.installing = false
		m.done = true
		m.err = msg.err
		if m.stream != nil {
			_ = m.stream.pipe.Close()
		}
		if msg.err == nil {
			m.output = appendInstallOutput(m.output, "Installed executable: "+filepath.Join(m.installDir, "sysclet"))
			if msg.version != "" {
				m.output = appendInstallOutput(m.output, "Installed version: "+msg.version)
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	installPath := filepath.Join(m.installDir, "sysclet")
	content := []string{
		headerStyle.Render(strings.Join(asciiHeaderLines, "\n")),
		titleStyle.Render("SYSCLET INSTALLER"),
		mutedStyle.Render("Install or update sysclet for this user"),
		accentStyle.Render("Binary: " + installPath),
		mutedStyle.Render("TDF fonts are bundled in the binary"),
		"",
	}
	switch {
	case m.installing:
		content = append(content, accentStyle.Render(m.spinner.View()+" Installing "+syscletModule))
		content = append(content, mutedStyle.Render("Destination: "+installPath))
		if m.output != "" {
			content = append(content, titleStyle.Render("Progress"), lipgloss.NewStyle().Width(78).Render(m.output))
		}
		content = append(content, mutedStyle.Render("Ctrl+C cancels the Go install"))
	case m.done && m.err == nil:
		content = append(content, successStyle.Render("Installed successfully to "+installPath))
		if m.output != "" {
			content = append(content, titleStyle.Render("Progress"), lipgloss.NewStyle().Width(78).Render(m.output))
		}
		content = append(content, "Run: "+installPath)
		content = append(content, mutedStyle.Render("Enter or q to close"))
	case m.done:
		content = append(content, errorStyle.Render("Install failed: "+m.err.Error()))
		if m.output != "" {
			content = append(content, lipgloss.NewStyle().Width(72).Render(m.output))
		}
		content = append(content, mutedStyle.Render("Enter or q to close"))
	default:
		content = append(content, mutedStyle.Render("Install: "+syscletModule))
		content = append(content, accentStyle.Render("Enter  Install or update sysclet"))
		content = append(content, mutedStyle.Render("q  Quit"))
		content = append(content, "")
		content = append(content, mutedStyle.Render("Requires Go 1.21+ and an internet connection."))
	}

	view := lipgloss.JoinVertical(lipgloss.Center, content...)
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, view)
	}
	return view
}

func installCmd(ctx context.Context, installDir string) tea.Cmd {
	return func() tea.Msg {
		goPath, err := exec.LookPath("go")
		if err != nil {
			return installStartedMsg{err: fmt.Errorf("find Go on PATH: %w", err)}
		}
		if err := os.MkdirAll(installDir, 0o755); err != nil {
			return installStartedMsg{err: fmt.Errorf("create %s: %w", installDir, err)}
		}

		cmd := exec.CommandContext(ctx, goPath, "install", syscletModule)
		cmd.Env = withEnv(os.Environ(), "GOBIN", installDir)
		reader, writer := io.Pipe()
		cmd.Stdout = writer
		cmd.Stderr = writer
		if err := cmd.Start(); err != nil {
			_ = reader.Close()
			_ = writer.Close()
			return installStartedMsg{err: fmt.Errorf("start go install: %w", err)}
		}
		done := make(chan error, 1)
		go func() {
			err := cmd.Wait()
			_ = writer.Close()
			if err != nil {
				err = fmt.Errorf("go install failed: %w", err)
			}
			done <- err
		}()
		return installStartedMsg{stream: &installStream{pipe: reader, reader: bufio.NewReader(reader), done: done, binary: filepath.Join(installDir, "sysclet")}}
	}
}

func readInstallOutputCmd(stream *installStream) tea.Cmd {
	return func() tea.Msg {
		line, err := stream.reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if line != "" {
			return installOutputMsg{line: line}
		}
		if err == io.EOF {
			done := installDoneMsg{err: <-stream.done}
			if done.err == nil {
				done.version = installedVersion(stream.binary)
			}
			return done
		}
		if err != nil {
			return installDoneMsg{err: err}
		}
		return installOutputMsg{}
	}
}

// installedVersion asks the new binary for its version; releases before
// v0.3.0 have no --version, so a failure just leaves the line out.
func installedVersion(binary string) string {
	output, err := exec.Command(binary, "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(string(output)), "sysclet ")
}

func appendInstallOutput(current, line string) string {
	lines := []string{}
	if current != "" {
		lines = strings.Split(current, "\n")
	}
	lines = append(lines, line)
	if len(lines) > 6 {
		lines = lines[len(lines)-6:]
	}
	return strings.Join(lines, "\n")
}

func withEnv(env []string, key, value string) []string {
	result := make([]string, 0, len(env)+1)
	for _, entry := range env {
		name, _, ok := strings.Cut(entry, "=")
		if !ok || name != key {
			result = append(result, entry)
		}
	}
	return append(result, key+"="+value)
}
