package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type exportFormat string

const (
	exportSyscGreet exportFormat = "sysc-greet"
	exportANSI      exportFormat = "ANSI"
	exportTXT       exportFormat = "TXT"
	exportSyscWalls exportFormat = "sysc-walls"
	exportSyscTerm  exportFormat = "sysc-terminal"
	exportGO        exportFormat = "GO"
	exportJS        exportFormat = "JS"
	exportPY        exportFormat = "PY"
	exportRS        exportFormat = "RS"
	exportSH        exportFormat = "SH"
)

type exportOption struct {
	format    exportFormat
	extension string
}

var exportOptions = [...]exportOption{
	{format: exportSyscGreet},
	{format: exportANSI, extension: ".ans"},
	{format: exportTXT, extension: ".txt"},
	{format: exportGO, extension: ".go"},
	{format: exportJS, extension: ".js"},
	{format: exportPY, extension: ".py"},
	{format: exportRS, extension: ".rs"},
	{format: exportSH, extension: ".sh"},
	{format: exportSyscWalls, extension: ".txt"},
	{format: exportSyscTerm, extension: ".txt"},
}

func cycleExportFormat(current exportFormat, step int) exportFormat {
	index := 0
	for i, option := range exportOptions {
		if option.format == current {
			index = i
			break
		}
	}
	index = (index + step%len(exportOptions) + len(exportOptions)) % len(exportOptions)
	return exportOptions[index].format
}

func exportExtension(format exportFormat) string {
	for _, option := range exportOptions {
		if option.format == format {
			return option.extension
		}
	}
	return ""
}

func appendExportExtension(path string, format exportFormat) string {
	if extension := exportExtension(format); extension != "" && !strings.HasSuffix(strings.ToLower(path), extension) {
		return path + extension
	}
	return path
}

func shellQuoteArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func exportDisplayHint(format exportFormat, path string) string {
	if format == exportSyscGreet {
		return "Select the Beams effect in sysc-greet to animate this profile art."
	}
	if strings.TrimSpace(path) == "" {
		path = "header"
	}
	if format == exportSyscWalls || format == exportSyscTerm {
		resolved, err := resolveSyscArtPath(path)
		if err != nil {
			return err.Error()
		}
		if format == exportSyscWalls {
			return "Saves plain text to " + resolved + " and sets sysc-walls to beam-text (dracula) with it. Restart: systemctl --user restart sysc-walls.service"
		}
		return "Run: sysc-terminal --effect beam-text --file " + shellQuoteArg(resolved)
	}
	file := shellQuoteArg(appendExportExtension(strings.TrimSpace(path), format))
	switch format {
	case exportANSI:
		return "Display in a terminal: cat " + file + " or less -R " + file
	case exportTXT:
		return "View in a terminal: cat " + file + " or less " + file + " · plain text; ANSI colors are removed."
	case exportGO:
		return "Run in a terminal: go run " + file
	case exportJS:
		return "Run in a terminal: node " + file
	case exportPY:
		return "Run in a terminal: python3 " + file
	case exportRS:
		return "Run in a terminal: rustc " + file + " -o sysclet-header && ./sysclet-header"
	case exportSH:
		return "Run in a terminal: sh " + file
	default:
		return ""
	}
}

func formatExportError(err error) string {
	if os.IsPermission(err) {
		return "export: permission denied; run sysclet with sudo for protected system paths, or choose a writable path"
	}
	return "export: " + err.Error()
}

func generateExportContent(format exportFormat, slot, art string) (string, error) {
	if art == "" {
		return "", fmt.Errorf("art must not be empty")
	}
	if format == exportSyscGreet {
		return ProfileSnippet(slot, art)
	}
	if format == exportANSI {
		return art, nil
	}
	if format == exportTXT || format == exportSyscWalls || format == exportSyscTerm {
		// sysc-walls and sysc-terminal draw each rune as a cell in their own theme, so escapes must go.
		return ansi.Strip(art), nil
	}
	switch format {
	case exportGO, exportJS, exportPY, exportRS, exportSH:
	default:
		return "", fmt.Errorf("unsupported export format %q", format)
	}

	lines := strings.Split(art, "\n")
	var output strings.Builder
	switch format {
	case exportGO:
		output.WriteString("package main\n\nimport \"fmt\"\n\nfunc main() {\n")
		for _, line := range lines {
			fmt.Fprintf(&output, "\tfmt.Println(%s)\n", strconv.Quote(line))
		}
		output.WriteString("}\n")
	case exportJS:
		output.WriteString("const ansiArtLines = [\n")
		for _, line := range lines {
			fmt.Fprintf(&output, "  %s,\n", strconv.Quote(line))
		}
		output.WriteString("];\n\nansiArtLines.forEach(line => console.log(line));\n")
	case exportPY:
		output.WriteString("ansi_art_lines = [\n")
		for _, line := range lines {
			fmt.Fprintf(&output, "    %s,\n", strconv.Quote(line))
		}
		output.WriteString("]\n\nfor line in ansi_art_lines:\n    print(line)\n")
	case exportRS:
		output.WriteString("fn main() {\n    let ansi_art_lines = [\n")
		for _, line := range lines {
			fmt.Fprintf(&output, "        %s,\n", strconv.Quote(line))
		}
		output.WriteString("    ];\n    for line in ansi_art_lines {\n        println!(\"{}\", line);\n    }\n}\n")
	case exportSH:
		output.WriteString("#!/bin/sh\n")
		for _, line := range lines {
			fmt.Fprintf(&output, "printf '%%b\\n' %s\n", shellPrintfQuote(line))
		}
	}
	return output.String(), nil
}

func shellPrintfQuote(line string) string {
	var escaped strings.Builder
	for _, char := range line {
		switch char {
		case '\x1b':
			escaped.WriteString(`\033`)
		case '\\':
			escaped.WriteString(`\\`)
		default:
			escaped.WriteRune(char)
		}
	}
	return "'" + strings.ReplaceAll(escaped.String(), "'", "'\\''") + "'"
}

func writeExportFile(path string, format exportFormat, content string) (string, string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "", fmt.Errorf("enter an output path")
	}
	extension := exportExtension(format)
	if extension == "" {
		return "", "", fmt.Errorf("%q is not a file export format", format)
	}
	path = appendExportExtension(path, format)
	path, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("resolve output path: %w", err)
	}

	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		file, createErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if createErr != nil {
			return "", "", fmt.Errorf("create export %q: %w", path, createErr)
		}
		_, writeErr := io.WriteString(file, content)
		if writeErr == nil {
			writeErr = file.Sync()
		}
		closeErr := file.Close()
		if writeErr != nil {
			_ = os.Remove(path)
			return "", "", fmt.Errorf("write export %q: %w", path, writeErr)
		}
		if closeErr != nil {
			_ = os.Remove(path)
			return "", "", fmt.Errorf("close export %q: %w", path, closeErr)
		}
		return path, "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("inspect export %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", "", fmt.Errorf("refusing to replace symlink %q", path)
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("export path %q is not a regular file", path)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read existing export %q: %w", path, err)
	}
	backup, err := createBackup(path, original, info.Mode().Perm())
	if err != nil {
		return "", "", fmt.Errorf("back up existing export %q: %w", path, err)
	}
	if err := replaceFile(path, original, []byte(content), info.Mode().Perm()); err != nil {
		return path, backup, fmt.Errorf("write export %q (original saved at %s): %w", path, backup, err)
	}
	return path, backup, nil
}

// resolveSyscArtPath places a bare name in sysc-Go's walls folder and keeps other paths
// inside the only roots sysc-walls and sysc-terminal will read artwork from.
func resolveSyscArtPath(path string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("enter an output path")
	}
	if !strings.ContainsRune(path, filepath.Separator) {
		path = filepath.Join(home, ".local", "share", "syscgo", "walls", path)
	}
	path, err = filepath.Abs(appendExportExtension(path, exportSyscWalls))
	if err != nil {
		return "", fmt.Errorf("resolve output path: %w", err)
	}
	for _, root := range []string{filepath.Join(home, ".local", "share"), filepath.Join(home, ".config")} {
		if strings.HasPrefix(path, root+string(filepath.Separator)) {
			return path, nil
		}
	}
	return "", fmt.Errorf("sysc-walls and sysc-terminal only read art under ~/.local/share or ~/.config")
}

// saveSyscArt writes plain-text art for sysc-walls or sysc-terminal; for sysc-walls it
// also points daemon.conf at the file.
func saveSyscArt(input string, format exportFormat, art string) (path, backup, configBackup string, err error) {
	if path, err = resolveSyscArtPath(input); err != nil {
		return "", "", "", err
	}
	content, err := generateExportContent(format, "", art)
	if err != nil {
		return "", "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", "", "", err
	}
	if path, backup, err = writeExportFile(path, format, content); err != nil || format != exportSyscWalls {
		return path, backup, "", err
	}
	home, _ := os.UserHomeDir()
	configBackup, err = pointSyscWallsAt(filepath.Join(home, ".config", "sysc-walls", "daemon.conf"), path)
	if err != nil {
		err = fmt.Errorf("saved %s, but could not update the sysc-walls config: %w", path, err)
	}
	return path, backup, configBackup, err
}

// pointSyscWallsAt sets the [animation] values sysc-Go's walls export sets, keeping
// every other line of daemon.conf, and backs the file up first.
func pointSyscWallsAt(configPath, artPath string) (string, error) {
	values := [][2]string{{"effect", "beam-text"}, {"theme", "dracula"}, {"file", artPath}, {"cycle", "false"}}
	original, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
			return "", err
		}
		defaults := "[idle]\ntimeout = 300s\nmin_duration = 30s\n\n[daemon]\ndebug = false\n\n[terminal]\nfullscreen = true\nkitty = true\n"
		return "", createProfileFile(configPath, []byte(setINIValues(defaults, "animation", values)))
	}
	if err != nil {
		return "", err
	}
	// Edit the real file so a dotfile symlink survives the atomic replace.
	realPath, err := filepath.EvalSymlinks(configPath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(realPath)
	if err != nil {
		return "", err
	}
	updated := setINIValues(string(original), "animation", values)
	if updated == string(original) {
		return "", nil
	}
	backup, err := createBackup(realPath, original, info.Mode().Perm())
	if err != nil {
		return "", err
	}
	return backup, replaceFile(realPath, original, []byte(updated), info.Mode().Perm())
}

// setINIValues rewrites or appends key = value lines inside one INI section,
// adding the section at the end when it is missing.
func setINIValues(text, section string, values [][2]string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	start, end := -1, len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
			continue
		}
		if start >= 0 {
			end = i
			break
		}
		if strings.TrimSpace(trimmed[1:len(trimmed)-1]) == section {
			start = i
		}
	}
	if start < 0 {
		lines = append(lines, "", "["+section+"]")
		start, end = len(lines)-1, len(lines)
	}
	set := make([]bool, len(values))
	for i := start + 1; i < end; i++ {
		key, _, ok := strings.Cut(lines[i], "=")
		for j, value := range values {
			if ok && strings.TrimSpace(key) == value[0] {
				lines[i] = value[0] + " = " + value[1]
				set[j] = true
			}
		}
	}
	var missing []string
	for j, value := range values {
		if !set[j] {
			missing = append(missing, value[0]+" = "+value[1])
		}
	}
	insert := end
	for insert > start+1 && strings.TrimSpace(lines[insert-1]) == "" {
		insert--
	}
	lines = append(lines[:insert], append(missing, lines[insert:]...)...)
	return strings.Join(lines, "\n") + "\n"
}
