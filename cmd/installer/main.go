package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const syscletModule = "github.com/Nomadcxx/sysclet@latest"
const releaseBaseURL = "https://github.com/Nomadcxx/sysclet/releases/latest/download"

const maxChecksumManifestSize = 1 << 20

// ponytail: cap release downloads at 256 MiB to bound disk use; raise this if the bundled app grows beyond it.
const maxReleaseBinarySize = 256 << 20

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

type installDoneMsg struct {
	err error
}

type model struct {
	ctx        context.Context
	cancel     context.CancelFunc
	installDir string
	spinner    spinner.Model
	installing bool
	done       bool
	err        error
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
			return m, tea.Batch(m.spinner.Tick, installCmd(m.ctx, m.installDir))
		}
	case spinner.TickMsg:
		if m.installing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case installDoneMsg:
		m.installing = false
		m.done = true
		m.err = msg.err
	}
	return m, nil
}

func (m model) View() string {
	installPath := filepath.Join(m.installDir, "sysclet")
	content := []string{
		headerStyle.Render(strings.Join(asciiHeaderLines, "\n")),
		titleStyle.Render("SYSCLET INSTALLER"),
		mutedStyle.Render("Install or update sysclet for this user"),
		accentStyle.Render(wrapInstallerPathLine("Binary: ", installPath, m.width)),
		mutedStyle.Render("TDF fonts are bundled in the binary"),
		"",
	}
	switch {
	case m.installing:
		content = append(content, accentStyle.Render(m.spinner.View()+" Installing sysclet release"))
		content = append(content, mutedStyle.Render("Downloading and verifying sysclet release"))
		content = append(content, mutedStyle.Render(wrapInstallerPathLine("Destination: ", installPath, m.width)))
		content = append(content, mutedStyle.Render("Ctrl+C cancels the download"))
	case m.done && m.err == nil:
		content = append(content, successStyle.Render(wrapInstallerPathLine("Installed successfully to ", installPath, m.width)))
		content = append(content, wrapInstallerPathLine("Run: ", installPath, m.width))
		content = append(content, mutedStyle.Render("Enter or q to close"))
	case m.done:
		content = append(content, errorStyle.Render("Install failed: "+m.err.Error()))
		content = append(content, mutedStyle.Render("Enter or q to close"))
	default:
		content = append(content, mutedStyle.Render("Install: "+syscletModule))
		content = append(content, accentStyle.Render("Enter  Install or update sysclet"))
		content = append(content, mutedStyle.Render("q  Quit"))
		content = append(content, "")
		content = append(content, mutedStyle.Render("Requires an internet connection."))
	}

	view := lipgloss.JoinVertical(lipgloss.Center, content...)
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, view)
	}
	return view
}

func wrapInstallerPathLine(label, path string, width int) string {
	line := label + path
	if width > 0 {
		return ansi.Hardwrap(line, width, true)
	}
	return line
}

func installCmd(ctx context.Context, installDir string) tea.Cmd {
	return func() tea.Msg {
		return installDoneMsg{err: installRelease(ctx, releaseHTTPClient, releaseBaseURL, installDir, runtime.GOOS, runtime.GOARCH)}
	}
}

func installRelease(ctx context.Context, client *http.Client, baseURL, installDir, goos, goarch string) error {
	if goos != "linux" || (goarch != "amd64" && goarch != "arm64") {
		return fmt.Errorf("release installation is supported on Linux amd64 and arm64, not %s/%s", goos, goarch)
	}
	if client == nil {
		return fmt.Errorf("release HTTP client is nil")
	}

	asset := "sysclet-linux-" + goarch
	manifestResponse, err := getReleaseAsset(ctx, client, baseURL, "SHA256SUMS")
	if err != nil {
		return err
	}
	manifest, err := io.ReadAll(io.LimitReader(manifestResponse.Body, maxChecksumManifestSize+1))
	_ = manifestResponse.Body.Close()
	if err != nil {
		return fmt.Errorf("read SHA256SUMS: %w", err)
	}
	if len(manifest) > maxChecksumManifestSize {
		return fmt.Errorf("SHA256SUMS exceeds %d bytes", maxChecksumManifestSize)
	}
	checksum, err := releaseChecksum(manifest, asset)
	if err != nil {
		return err
	}

	assetResponse, err := getReleaseAsset(ctx, client, baseURL, asset)
	if err != nil {
		return err
	}
	defer assetResponse.Body.Close()
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", installDir, err)
	}
	tmp, err := os.CreateTemp(installDir, ".sysclet-*")
	if err != nil {
		return fmt.Errorf("create temporary binary in %s: %w", installDir, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(assetResponse.Body, maxReleaseBinarySize+1))
	if err != nil {
		_ = tmp.Close()
		return fmt.Errorf("download %s: %w", asset, err)
	}
	if size == 0 {
		_ = tmp.Close()
		return fmt.Errorf("release asset %s is empty", asset)
	}
	if size > maxReleaseBinarySize {
		_ = tmp.Close()
		return fmt.Errorf("release asset %s exceeds %d bytes", asset, maxReleaseBinarySize)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != checksum {
		_ = tmp.Close()
		return fmt.Errorf("SHA-256 mismatch for %s", asset)
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("make downloaded binary executable: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync downloaded binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close downloaded binary: %w", err)
	}
	if err := os.Rename(tmpPath, filepath.Join(installDir, "sysclet")); err != nil {
		return fmt.Errorf("replace %s: %w", filepath.Join(installDir, "sysclet"), err)
	}
	return nil
}

var releaseHTTPClient = &http.Client{Timeout: 5 * time.Minute}

func getReleaseAsset(ctx context.Context, client *http.Client, baseURL, name string) (*http.Response, error) {
	url := strings.TrimRight(baseURL, "/") + "/" + name
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create release request for %s: %w", name, err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", name, err)
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return nil, fmt.Errorf("download %s: HTTP %s", name, response.Status)
	}
	return response, nil
}

func releaseChecksum(manifest []byte, asset string) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(manifest))
	checksum := ""
	for line := 1; scanner.Scan(); line++ {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 {
			return "", fmt.Errorf("invalid SHA256SUMS entry on line %d", line)
		}
		decoded, err := hex.DecodeString(fields[0])
		if err != nil || len(decoded) != sha256.Size {
			return "", fmt.Errorf("invalid SHA-256 digest on line %d", line)
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name == asset {
			if checksum != "" {
				return "", fmt.Errorf("duplicate SHA256SUMS entry for %s", asset)
			}
			checksum = strings.ToLower(fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read SHA256SUMS: %w", err)
	}
	if checksum == "" {
		return "", fmt.Errorf("SHA256SUMS has no entry for %s", asset)
	}
	return checksum, nil
}
