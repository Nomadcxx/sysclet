package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
)

// version is stamped by release builds: -ldflags "-X main.version=v0.3.0".
var version string

// versionString falls back to the module version Go records, so
// `go install github.com/Nomadcxx/sysclet@v0.3.0` reports v0.3.0 too.
func versionString() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "devel"
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return runTUI()
	}
	if args[0] == "--help" || args[0] == "-h" {
		printUsage(stdout)
		return 0
	}
	if args[0] == "--version" || args[0] == "-V" || args[0] == "version" {
		fmt.Fprintln(stdout, "sysclet "+versionString())
		return 0
	}
	switch args[0] {
	case "tui":
		return runTUI()
	case "render":
		return runRender(args[1:], stdout, stderr)
	case "export":
		return runExport(args[1:], stdout, stderr)
	case "fonts":
		return runFonts(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runRender(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("render", flag.ContinueOnError)
	flags.SetOutput(stderr)
	fontPath := flags.String("font", "", "path to a local TheDraw TDF font file")
	fontName := flags.String("font-name", "", "font name in the file, or bundled font when --font is omitted")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: sysclet render (--font FONT.tdf [--font-name NAME] | --font-name BUNDLED_FONT) TEXT")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *fontPath == "" && *fontName == "" {
		fmt.Fprintln(stderr, "render requires --font or --font-name")
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
	fontPath := flags.String("font", "", "path to a local TheDraw TDF font file")
	fontName := flags.String("font-name", "", "font name in the file, or bundled font when --font is omitted")
	slot := flags.String("slot", "", "sysc-greet ASCII slot, such as ascii_1")
	configPath := flags.String("config", "", "create or update this sysc-greet profile")
	formatName := flags.String("format", string(exportSyscGreet), "sysc-greet, ansi, txt, go, js, py, rs, sh, sysc-walls or sysc-terminal")
	output := flags.String("output", "", "file to write; ansi, txt and source formats print to stdout without it; required for sysc-walls and sysc-terminal")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: sysclet export (--font FONT.tdf [--font-name NAME] | --font-name BUNDLED_FONT) --slot ascii_N [--config PROFILE] TEXT")
		fmt.Fprintln(stderr, "       sysclet export (--font FONT.tdf [--font-name NAME] | --font-name BUNDLED_FONT) --format FORMAT [--output PATH] TEXT")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *fontPath == "" && *fontName == "" {
		fmt.Fprintln(stderr, "export requires --font or --font-name")
		return 2
	}
	format, ok := parseExportFormat(*formatName)
	if !ok {
		fmt.Fprintf(stderr, "unknown export format %q\n", *formatName)
		return 2
	}
	if format != exportSyscGreet && (*slot != "" || *configPath != "") {
		fmt.Fprintln(stderr, "--slot and --config apply only to --format sysc-greet")
		return 2
	}
	if format == exportSyscGreet && *output != "" {
		fmt.Fprintln(stderr, "--output does not apply to sysc-greet; use --config PROFILE")
		return 2
	}
	if (format == exportSyscWalls || format == exportSyscTerm) && *output == "" {
		fmt.Fprintf(stderr, "--format %s requires --output NAME_OR_PATH\n", format)
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
	if format != exportSyscGreet {
		return writeCLIExport(format, *output, art, stdout, stderr)
	}
	if *configPath != "" {
		backupPath, err := UpdateProfile(*configPath, *slot, art)
		if err != nil {
			fmt.Fprintln(stderr, formatExportError(err))
			return 1
		}
		if backupPath == "" {
			fmt.Fprintf(stderr, "created %s\n", *configPath)
			return 0
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

func writeCLIExport(format exportFormat, output, art string, stdout, stderr io.Writer) int {
	var path, backup, configBackup string
	var err error
	switch {
	case format == exportSyscWalls || format == exportSyscTerm:
		path, backup, configBackup, err = saveSyscArt(output, format, art)
	case output == "":
		content, err := generateExportContent(format, "", art)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if _, err := io.WriteString(stdout, content); err != nil {
			fmt.Fprintf(stderr, "write export: %v\n", err)
			return 1
		}
		return 0
	default:
		var content string
		if content, err = generateExportContent(format, "", art); err == nil {
			path, backup, err = writeExportFile(output, format, content)
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, formatExportError(err))
		return 1
	}
	fmt.Fprintf(stderr, "wrote %s\n", path)
	if backup != "" {
		fmt.Fprintf(stderr, "backup: %s\n", backup)
	}
	switch format {
	case exportSyscWalls:
		fmt.Fprintln(stderr, "updated sysc-walls config; restart sysc-walls.service")
		if configBackup != "" {
			fmt.Fprintf(stderr, "config backup: %s\n", configBackup)
		}
	case exportSyscTerm:
		fmt.Fprintln(stderr, exportDisplayHint(format, path))
	}
	return 0
}

func runFonts(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("fonts", flag.ContinueOnError)
	flags.SetOutput(stderr)
	fontPath := flags.String("font", "", "list the records in this local TDF file instead of the bundled catalog")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: sysclet fonts [--font FONT.tdf] [FILTER]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	var names []string
	if *fontPath != "" {
		fonts, err := loadFonts(*fontPath)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		for _, font := range fonts {
			names = append(names, font.Name)
		}
	} else {
		catalog, err := loadBundledFontCatalog()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		for _, entry := range catalog {
			names = append(names, entry.Name)
		}
	}
	filter := strings.ToLower(strings.Join(flags.Args(), " "))
	matched := 0
	for _, name := range names {
		if strings.Contains(strings.ToLower(name), filter) {
			fmt.Fprintln(stdout, name)
			matched++
		}
	}
	if matched == 0 {
		fmt.Fprintf(stderr, "no fonts match %q\n", filter)
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
	if path == "" {
		if name == "" {
			return Font{}, fmt.Errorf("font path or bundled font name is required")
		}
		return loadBundledFontByName(name)
	}
	fonts, err := loadFonts(path)
	if err != nil {
		return Font{}, err
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

func loadFonts(path string) ([]Font, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("inspect font %q: %w", path, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect font %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("font %q is not a regular file", path)
	}
	if info.Size() > maxTDFBytes {
		return nil, fmt.Errorf("font %q exceeds the %d MiB size limit", path, maxTDFBytes>>20)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxTDFBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read font %q: %w", path, err)
	}
	if len(data) > maxTDFBytes {
		return nil, fmt.Errorf("font %q exceeds the %d MiB size limit", path, maxTDFBytes>>20)
	}
	fonts, err := ParseTDF(data)
	if err != nil {
		return nil, fmt.Errorf("parse font %q: %w", path, err)
	}
	return fonts, nil
}

func printUsage(output io.Writer) {
	fmt.Fprintln(output, "sysclet renders TheDraw TDF fonts as ANSI art and exports it to sysc-greet, sysc-walls, sysc-terminal and files.")
	fmt.Fprintln(output, "")
	fmt.Fprintln(output, "Commands:")
	fmt.Fprintln(output, "  tui     Open the interactive ANSI header editor")
	fmt.Fprintln(output, "  render  Print ANSI art to the terminal")
	fmt.Fprintln(output, "  export  Print a sysc-greet profile snippet or update a profile; --format picks another target")
	fmt.Fprintln(output, "  fonts   List bundled font names, or the records in a local .tdf file")
	fmt.Fprintln(output, "  version Print the sysclet version (also --version, -V)")
	fmt.Fprintln(output, "")
	fmt.Fprintln(output, "Examples:")
	fmt.Fprintln(output, "  sysclet")
	fmt.Fprintln(output, "  sysclet render --font-name Bleach SYSC")
	fmt.Fprintln(output, "  sysclet render --font tdf-bleach.tdf SYSC")
	fmt.Fprintln(output, "  sysclet export --font tdf-bleach.tdf --slot ascii_1 SYSC")
	fmt.Fprintln(output, "  sysclet export --font-name Bleach --format sysc-walls --output header SYSC")
	fmt.Fprintln(output, "  sysclet fonts rivendell")
}
