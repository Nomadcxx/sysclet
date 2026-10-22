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
	if format == exportTXT {
		return "Plain text only; ANSI colors are removed. Choose ANSI to keep them."
	}
	if strings.TrimSpace(path) == "" {
		path = "header"
	}
	file := shellQuoteArg(appendExportExtension(strings.TrimSpace(path), format))
	switch format {
	case exportANSI:
		return "Display in a terminal: cat " + file + " or less -R " + file
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
	if format == exportTXT {
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
