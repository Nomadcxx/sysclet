package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const maxTDFBytes = 64 << 20

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printUsage(stdout)
		return 0
	}
	switch args[0] {
	case "render":
		return runRender(args[1:], stdout, stderr)
	case "export":
		return runExport(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runRender(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("render", flag.ContinueOnError)
	flags.SetOutput(stderr)
	fontPath := flags.String("font", "", "path to a TheDraw TDF font file")
	fontName := flags.String("font-name", "", "font name when the file contains multiple fonts")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: sysclet render --font FONT.tdf [--font-name NAME] TEXT")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *fontPath == "" {
		fmt.Fprintln(stderr, "render requires --font")
		return 2
	}
	text, err := commandText(flags.Args())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	font, err := loadFont(*fontPath, *fontName)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	art, err := font.Render(text)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if _, err := fmt.Fprintln(stdout, art); err != nil {
		fmt.Fprintf(stderr, "write art: %v\n", err)
		return 1
	}
	return 0
}

func runExport(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	flags.SetOutput(stderr)
	fontPath := flags.String("font", "", "path to a TheDraw TDF font file")
	fontName := flags.String("font-name", "", "font name when the file contains multiple fonts")
	slot := flags.String("slot", "", "sysc-greet ASCII slot, such as ascii_1")
	configPath := flags.String("config", "", "update this profile and save a backup")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: sysclet export --font FONT.tdf --slot ascii_N [--font-name NAME] [--config PROFILE] TEXT")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *fontPath == "" {
		fmt.Fprintln(stderr, "export requires --font")
		return 2
	}
	text, err := commandText(flags.Args())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	font, err := loadFont(*fontPath, *fontName)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	art, err := font.Render(text)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *configPath != "" {
		backupPath, err := UpdateProfile(*configPath, *slot, art)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stderr, "updated %s\nbackup: %s\n", *configPath, backupPath)
		return 0
	}
	snippet, err := ProfileSnippet(*slot, art)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if _, err := io.WriteString(stdout, snippet); err != nil {
		fmt.Fprintf(stderr, "write profile snippet: %v\n", err)
		return 1
	}
	return 0
}

func commandText(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("text is required")
	}
	return strings.Join(args, " "), nil
}

func loadFont(path, name string) (Font, error) {
	file, err := os.Open(path)
	if err != nil {
		return Font{}, fmt.Errorf("inspect font %q: %w", path, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Font{}, fmt.Errorf("inspect font %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return Font{}, fmt.Errorf("font %q is not a regular file", path)
	}
	if info.Size() > maxTDFBytes {
		return Font{}, fmt.Errorf("font %q exceeds the %d MiB size limit", path, maxTDFBytes>>20)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxTDFBytes+1))
	if err != nil {
		return Font{}, fmt.Errorf("read font %q: %w", path, err)
	}
	if len(data) > maxTDFBytes {
		return Font{}, fmt.Errorf("font %q exceeds the %d MiB size limit", path, maxTDFBytes>>20)
	}
	fonts, err := ParseTDF(data)
	if err != nil {
		return Font{}, fmt.Errorf("parse font %q: %w", path, err)
	}
	if name == "" {
		return fonts[0], nil
	}
	for _, font := range fonts {
		if strings.EqualFold(font.Name, name) {
			return font, nil
		}
	}
	names := make([]string, len(fonts))
	for i, font := range fonts {
		names[i] = font.Name
	}
	return Font{}, fmt.Errorf("font %q not found; available fonts: %s", name, strings.Join(names, ", "))
}

func printUsage(output io.Writer) {
	fmt.Fprintln(output, "sysclet renders TheDraw TDF fonts as ANSI art and exports sysc-greet profile slots.")
	fmt.Fprintln(output, "")
	fmt.Fprintln(output, "Commands:")
	fmt.Fprintln(output, "  render  Print ANSI art to the terminal")
	fmt.Fprintln(output, "  export  Print a sysc-greet profile snippet or update a profile")
	fmt.Fprintln(output, "")
	fmt.Fprintln(output, "Examples:")
	fmt.Fprintln(output, "  sysclet render --font tdf-bleach.tdf SYSC")
	fmt.Fprintln(output, "  sysclet export --font tdf-bleach.tdf --slot ascii_1 SYSC")
}
