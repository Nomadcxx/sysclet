package main

import (
	"errors"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestDefaultFontDirUsesXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/sysclet-data")
	t.Setenv("HOME", "/tmp/home")

	got, err := defaultFontDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/tmp/sysclet-data", "sysclet", "fonts"); got != want {
		t.Fatalf("defaultFontDir() = %q, want %q", got, want)
	}
}

func TestDefaultFontDirFallsBackToLocalShare(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "relative-data")
	t.Setenv("HOME", "/tmp/home")

	got, err := defaultFontDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/tmp/home", ".local", "share", "sysclet", "fonts"); got != want {
		t.Fatalf("defaultFontDir() = %q, want %q", got, want)
	}
}

func TestNewEditorStartsWithBundledFont(t *testing.T) {
	model := newEditorModel()
	if model.font.Name != "Bleach" {
		t.Fatalf("initial font = %q, want bundled Bleach", model.font.Name)
	}
	if model.preview == "" {
		t.Fatal("initial bundled font did not render the default SYSC header")
	}
}

func TestFontControlSearchesBundledCatalogAndLoadsChoice(t *testing.T) {
	model := newEditorModel()
	model.openFontPicker()
	if model.mode != modeFontChoice || !model.fontChoiceFromCatalog || len(model.fontChoices) != 3481 {
		t.Fatalf("font chooser state = mode %v, catalog=%v, choices=%d; want all bundled fonts", model.mode, model.fontChoiceFromCatalog, len(model.fontChoices))
	}

	model.fontFilter.SetValue("FinalDestiny")
	model.filterFontChoices()
	if len(model.fontChoices) != 1 {
		t.Fatalf("FinalDestiny search returned %d results, want actual pack record FinalDestiny2", len(model.fontChoices))
	}
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	if model.font.Name != "FinalDestiny2" {
		t.Fatalf("selected font = %q, want actual bundled record FinalDestiny2", model.font.Name)
	}
}

func TestBundledFontChooserCanOpenLocalTDFPicker(t *testing.T) {
	model := newEditorModel()
	model.openFontPicker()
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlO}))
	if model.mode != modeFontPicker {
		t.Fatalf("mode after Ctrl+O = %v, want local file picker", model.mode)
	}
}

func TestBundledFontChoiceViewRendersWithoutLoadedLocalFonts(t *testing.T) {
	model := newEditorModel()
	model.width, model.height = 100, 32
	model.openFontPicker()
	view := model.View()
	if !strings.Contains(view, "Choose a bundled TDF font") {
		t.Fatalf("font chooser view = %q, missing bundled catalog title", view)
	}
}

func TestLocalFontPickerEscapeReturnsToBundledCatalog(t *testing.T) {
	model := newEditorModel()
	model.openFontPicker()
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlO}))
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEsc}))
	if model.mode != modeFontChoice || !model.fontChoiceFromCatalog {
		t.Fatalf("state after escaping local picker = mode %v, catalog=%v; want bundled catalog", model.mode, model.fontChoiceFromCatalog)
	}
}

func TestSetTextRendersANSIImmediately(t *testing.T) {
	font := Font{Name: "Tiny"}
	font.Glyphs[int('A'-33)] = [][]Cell{{{Rune: 'A', Color: 0x14}}}
	model := newEditorModel()
	model.font = font

	if err := model.setText("A"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(model.preview, "\x1b[0;31;44mA\x1b[0m") {
		t.Fatalf("preview = %q, want TDF foreground/background SGR", model.preview)
	}
}

func TestTypingRerendersPreviewOnEachKey(t *testing.T) {
	model := newEditorModel()
	font := Font{Name: "Tiny"}
	font.Glyphs[int('A'-33)] = [][]Cell{{{Rune: 'A', Color: 0x14}}}
	model.font = font
	model.textInput.SetValue("")
	model.textInput.Focus()

	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'A'}}))
	if model.text != "A" || !strings.Contains(model.preview, "\x1b[0;31;44mA\x1b[0m") {
		t.Fatalf("after one key, text=%q preview=%q", model.text, model.preview)
	}
}

func TestViewPreservesTDFAnsiColors(t *testing.T) {
	font := Font{Name: "Tiny"}
	font.Glyphs[int('A'-33)] = [][]Cell{{{Rune: 'A', Color: 0x14}}}
	model := newEditorModel()
	model.width, model.height = 100, 32
	model.font = font
	if err := model.setText("A"); err != nil {
		t.Fatal(err)
	}
	view := model.View()
	if !strings.Contains(view, "\x1b[0;31;44mA\x1b[0m") {
		t.Fatalf("TUI view lost TDF color sequences: %q", view)
	}
}

func TestAlignmentControlChangesPreviewWidth(t *testing.T) {
	model := newEditorModel()
	font := Font{Name: "Tiny"}
	font.Glyphs[int('A'-33)] = [][]Cell{{{Rune: 'A', Color: 0x14}}}
	model.font = font
	if err := model.setText("A"); err != nil {
		t.Fatal(err)
	}
	if width := ansi.StringWidth(model.preview); width != 80 {
		t.Fatalf("default centered preview width = %d, want 80", width)
	}

	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRight}))
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRight}))
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyUp}))
	if width := ansi.StringWidth(model.preview); width != 1 {
		t.Fatalf("left aligned preview width = %d, want 1", width)
	}

	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyDown}))
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyDown}))
	if width := ansi.StringWidth(model.preview); width != 80 || !strings.HasPrefix(model.preview, " ") {
		t.Fatalf("right aligned preview = width %d, value %q; want 80 columns with leading padding", width, model.preview)
	}
}

func TestCenterAlignmentKeepsRaggedRowsOnSharedBlockOrigin(t *testing.T) {
	art := "\x1b[0;31mABCDEFGH\x1b[0m\n\x1b[0;32mWXYZ\x1b[0m\n\x1b[0;34mAB\x1b[0m"
	got := alignANSI(art, alignmentCenter, 12)
	lines := strings.Split(ansi.Strip(got), "\n")
	if len(lines) != 3 {
		t.Fatalf("aligned rows = %d, want 3", len(lines))
	}
	for i, line := range lines {
		leading := len(line) - len(strings.TrimLeft(line, " "))
		if leading != 2 {
			t.Errorf("row %d starts at column %d, want shared block origin column 2: %q", i, leading, line)
		}
		if width := ansi.StringWidth(line); width != 12 {
			t.Errorf("row %d width = %d, want 12", i, width)
		}
	}
}

func TestPreviewViewKeepsCenteredRowsOnASharedOrigin(t *testing.T) {
	model := newEditorModel()
	model.width, model.height = 120, 32
	model.previewRaw = "\x1b[0;31mABCDEFGH\x1b[0m\n\x1b[0;32mWXYZ\x1b[0m"
	view := ansi.Strip(model.View())
	positions := map[string]int{}
	for _, line := range strings.Split(view, "\n") {
		for _, art := range []string{"ABCDEFGH", "WXYZ"} {
			if position := strings.Index(line, art); position >= 0 {
				positions[art] = position
			}
		}
	}
	first, firstOK := positions["ABCDEFGH"]
	second, secondOK := positions["WXYZ"]
	if !firstOK || !secondOK {
		t.Fatalf("preview rows not found in view: %#v", positions)
	}
	if first != second {
		t.Fatalf("centered row origins differ: %#v\n%s", positions, view)
	}
}

func TestFontControlCyclesWithVerticalArrows(t *testing.T) {
	model := newEditorModel()
	model.selectedControl = controlFont
	original := model.font.Name
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyDown}))
	if model.font.Name == original {
		t.Fatalf("font after Down = %q, want a different bundled font", model.font.Name)
	}
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyUp}))
	if model.font.Name != original {
		t.Fatalf("font after Down then Up = %q, want %q", model.font.Name, original)
	}
}

func TestTabMovesThroughControlsAndShiftTabMovesBack(t *testing.T) {
	model := newEditorModel()
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	if model.selectedControl != controlFont {
		t.Fatalf("selected control after Tab = %d, want TDF FONT (%d)", model.selectedControl, controlFont)
	}
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	if model.selectedControl != controlAlignment {
		t.Fatalf("selected control after second Tab = %d, want ALIGN (%d)", model.selectedControl, controlAlignment)
	}
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyShiftTab}))
	if model.selectedControl != controlFont {
		t.Fatalf("selected control after Shift+Tab = %d, want TDF FONT (%d)", model.selectedControl, controlFont)
	}

	model.selectedControl = controlText
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyShiftTab}))
	if model.selectedControl != controlExport {
		t.Fatalf("selected control after Shift+Tab at first control = %d, want EXPORT (%d)", model.selectedControl, controlExport)
	}
}

func TestViewIdentifiesTheSelectedControl(t *testing.T) {
	model := newEditorModel()
	model.width, model.height = 100, 32
	model.selectedControl = controlBackground
	view := ansi.Strip(model.View())
	if !strings.Contains(view, "SELECTED: BG") {
		t.Fatalf("view does not identify selected BG control:\n%s", view)
	}
}

func TestFocusedControlHasADistinctRing(t *testing.T) {
	model := newEditorModel()
	model.selectedControl = controlAlignment
	focused := ansi.Strip(model.controlView(controlAlignment, "ALIGN", "Center"))
	unfocused := ansi.Strip(model.controlView(controlText, "TEXT", "SYSC"))
	if !strings.Contains(focused, "╔") || !strings.Contains(focused, "║") {
		t.Fatalf("focused control has no distinct ring:\n%s", focused)
	}
	if strings.Contains(unfocused, "╔") || strings.Contains(unfocused, "║") {
		t.Fatalf("unfocused control unexpectedly has focused ring:\n%s", unfocused)
	}
}

func TestViewShowsTheFullFontNameAboveControls(t *testing.T) {
	model := newEditorModel()
	model.width, model.height = 100, 32
	model.font.Name = "A Font Name Longer Than The Control"
	view := ansi.Strip(model.View())
	if !strings.Contains(view, model.font.Name) {
		t.Fatalf("view does not show full font name %q", model.font.Name)
	}
}

func TestControlStripUsesTheAvailableScreenWidth(t *testing.T) {
	model := newEditorModel()
	model.width, model.height = 120, 32
	for _, line := range strings.Split(ansi.Strip(model.View()), "\n") {
		if strings.Contains(line, "TEXT") && strings.Contains(line, "TDF FONT") && strings.Contains(line, "EXPORT") {
			if position := strings.Index(line, "EXPORT"); position < model.width-40 {
				t.Fatalf("EXPORT control starts at column %d, want near the right edge", position)
			}
			return
		}
	}
	t.Fatal("control strip row not found")
}

func TestExportViewListsFormatsWithSyscGreetFirst(t *testing.T) {
	model := newEditorModel()
	model.width, model.height = 120, 36
	model.openExport()
	view := ansi.Strip(model.View())
	for _, format := range []string{"sysc-greet", "ANSI", "TXT", "GO", "JS", "PY", "RS", "SH"} {
		if !strings.Contains(view, format) {
			t.Errorf("export view is missing %q", format)
		}
	}
	if !strings.Contains(view, "Snippet preview") {
		t.Fatal("sysc-greet export lost its snippet preview")
	}
}

func TestExportTabsThroughFormatsBeforeDestination(t *testing.T) {
	model := newEditorModel()
	model.openExport()
	want := []exportFormat{exportSyscGreet, exportANSI, exportTXT, exportGO, exportJS, exportPY, exportRS, exportSH}
	for i, format := range want {
		if model.exportFormat != format {
			t.Fatalf("format tab %d = %q, want %q", i, model.exportFormat, format)
		}
		model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	}
	if model.exportFormat != exportSyscGreet {
		t.Fatalf("format after cycling all tabs = %q, want sysc-greet", model.exportFormat)
	}
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyShiftTab}))
	if model.exportFormat != exportSH {
		t.Fatalf("format after Shift+Tab = %q, want SH", model.exportFormat)
	}
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	if model.exportFormat != exportSyscGreet {
		t.Fatalf("format after Tab = %q, want sysc-greet", model.exportFormat)
	}
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	if model.exportFocus != exportFocusPath {
		t.Fatalf("focus after opening destination = %d, want output path", model.exportFocus)
	}
}

func TestOpenExportPreservesExplicitProfilePath(t *testing.T) {
	model := newEditorModel()
	path := filepath.Join(t.TempDir(), "custom-compositor.conf")
	model.profilePathInput.SetValue(path)

	model.openExport()

	if got := model.profilePathInput.Value(); got != path {
		t.Fatalf("profile path after opening export = %q, want explicit path %q", got, path)
	}
}

func TestOpenExportShowsProfileSlotReadErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "malformed.conf")
	if err := os.WriteFile(path, []byte("ascii_1=\"\"\"\nunterminated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	model := newEditorModel()
	model.profilePathInput.SetValue(path)

	model.openExport()

	if !strings.Contains(model.status, "Cannot choose a new profile slot") {
		t.Fatalf("export status = %q, want profile parsing guidance", model.status)
	}
}

func TestExportDetectsProfileWhenSwitchingToSyscGreet(t *testing.T) {
	profileName := "sysclet_test_compositor"
	configHome := t.TempDir()
	configDir := filepath.Join(configHome, "sysc-greet", "ascii_configs")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(configDir, profileName+".conf")
	if err := os.WriteFile(profilePath, []byte("ascii_2=\"\"\"\nexisting\n\"\"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("SYSC_COMPOSITOR", profileName)
	t.Setenv("NIRI_SOCKET", "")
	t.Setenv("SWAYSOCK", "")
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	t.Setenv("XDG_SESSION_DESKTOP", "")
	t.Setenv("DESKTOP_SESSION", "")

	model := newEditorModel()
	model.exportFormat = exportANSI
	model.openExport()
	for i := 0; i < len(exportOptions)-1; i++ {
		model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	}

	if model.exportFormat != exportSyscGreet {
		t.Fatalf("selected format = %q, want sysc-greet", model.exportFormat)
	}
	if got := model.profilePathInput.Value(); got != profilePath {
		t.Fatalf("detected profile path = %q, want %q", got, profilePath)
	}
	if got := model.slotInput.Value(); got != "ascii_3" {
		t.Fatalf("detected profile slot = %q, want ascii_3", got)
	}
}

func TestExportF5UsesNextSlotForTypedProfilePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "greeter.conf")
	profile := "ascii_1=\"\"\"\nfirst\n\"\"\"\nascii_3=\"\"\"\nthird\n\"\"\"\n"
	if err := os.WriteFile(path, []byte(profile), 0o600); err != nil {
		t.Fatal(err)
	}

	model := newEditorModel()
	model.mode = modeExport
	model.exportFormat = exportSyscGreet
	model.slotAuto = true
	model.profilePathInput.SetValue(path)
	model.preview = "A"
	var copied string
	model.writeClipboard = func(value string) error {
		copied = value
		return nil
	}

	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyF5}))

	if !strings.Contains(copied, "ascii_4=\"\"\"\n") || strings.Contains(copied, "ascii_1=\"\"\"\n") {
		t.Fatalf("copied snippet = %q, want the next profile slot ascii_4", copied)
	}
}

func TestExportF5CopiesPlainSyscGreetSnippet(t *testing.T) {
	model := newEditorModel()
	model.openExport()
	model.slotInput.SetValue("ascii_3")
	var copied string
	model.writeClipboard = func(value string) error {
		copied = value
		return nil
	}

	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyF5}))
	want, err := ProfileSnippet("ascii_3", model.preview)
	if err != nil {
		t.Fatal(err)
	}
	if copied != want {
		t.Fatalf("copied snippet = %q, want %q", copied, want)
	}
	if model.status != "Copied sysc-greet snippet to clipboard" {
		t.Fatalf("copy status = %q, want success message", model.status)
	}
}

func TestExportF5ReportsClipboardErrors(t *testing.T) {
	model := newEditorModel()
	model.openExport()
	model.writeClipboard = func(string) error { return errors.New("clipboard unavailable") }

	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyF5}))
	if model.status != "Clipboard copy failed: clipboard unavailable" {
		t.Fatalf("copy status = %q, want clipboard error", model.status)
	}
}

func TestExportCanWriteBashOutputAlongsideSyscGreet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "header.sh")
	model := newEditorModel()
	font := Font{Name: "Tiny"}
	font.Glyphs[int('A'-33)] = [][]Cell{{{Rune: 'A', Color: 0x14}}}
	model.font = font
	if err := model.setText("A"); err != nil {
		t.Fatal(err)
	}
	model.openExport()
	model.filePathInput.SetValue(path)
	for i := 0; i < 7; i++ {
		model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	}
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlS}))

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated Bash file: %v (status: %s)", err, model.status)
	}
	for _, want := range []string{"#!/bin/sh", "printf '%b\\n'", `\033[0;31;44mA\033[0m`} {
		if !strings.Contains(string(content), want) {
			t.Errorf("Bash export is missing %q: %s", want, content)
		}
	}
	rendered, err := exec.Command("sh", path).Output()
	if err != nil {
		t.Fatalf("run generated Bash script: %v", err)
	}
	if !strings.Contains(string(rendered), "\x1b[0;31;44mA\x1b[0m") {
		t.Fatalf("Bash output = %q, want the colored glyph", rendered)
	}
}

func TestBITExportFormatsWriteTheirExpectedContent(t *testing.T) {
	tests := []struct {
		name       string
		tabs       int
		extension  string
		want       string
		wantAbsent string
	}{
		{name: "ANSI", tabs: 1, extension: ".ans", want: "\x1b[0;31;44mA\x1b[0m"},
		{name: "TXT", tabs: 2, extension: ".txt", want: "A", wantAbsent: "\x1b["},
		{name: "GO", tabs: 3, extension: ".go", want: "package main", wantAbsent: ""},
		{name: "JS", tabs: 4, extension: ".js", want: "console.log", wantAbsent: ""},
		{name: "PY", tabs: 5, extension: ".py", want: "print(line)", wantAbsent: ""},
		{name: "RS", tabs: 6, extension: ".rs", want: "fn main()", wantAbsent: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "header")
			model := newEditorModel()
			font := Font{Name: "Tiny"}
			font.Glyphs[int('A'-33)] = [][]Cell{{{Rune: 'A', Color: 0x14}}}
			model.font = font
			if err := model.setText("A"); err != nil {
				t.Fatal(err)
			}
			model.openExport()
			model.filePathInput.SetValue(path)
			for i := 0; i < test.tabs; i++ {
				model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
			}
			model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
			model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlS}))

			content, err := os.ReadFile(path + test.extension)
			if err != nil {
				t.Fatalf("read %s output: %v (status: %s)", test.name, err, model.status)
			}
			if !strings.Contains(string(content), test.want) {
				t.Errorf("%s output is missing %q: %s", test.name, test.want, content)
			}
			if test.wantAbsent != "" && strings.Contains(string(content), test.wantAbsent) {
				t.Errorf("%s output unexpectedly contains %q: %s", test.name, test.wantAbsent, content)
			}
			if test.name == "GO" {
				if _, err := format.Source(content); err != nil {
					t.Errorf("GO output is not valid Go source: %v", err)
				}
			}
		})
	}
}

func TestWriteExportFileKeepsBackupOfExistingOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "header.txt")
	original := "old header"
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}

	gotPath, backupPath, err := writeExportFile(path, exportTXT, "new header")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != path {
		t.Fatalf("export path = %q, want %q", gotPath, path)
	}
	if backupPath == "" {
		t.Fatal("replacing existing output did not create a backup")
	}
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != original {
		t.Fatalf("backup = %q, want %q", backup, original)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(updated) != "new header" {
		t.Fatalf("updated export = %q, want new header", updated)
	}
}

func TestWriteExportFileRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "header.txt")
	if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("create symlink: %v", err)
	}
	if _, _, err := writeExportFile(link, exportTXT, "replace"); err == nil {
		t.Fatal("writeExportFile followed a symlink")
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "keep" {
		t.Fatalf("symlink target changed to %q", contents)
	}
}

func TestPreviewCentersWithinAn80ColumnPanel(t *testing.T) {
	model := newEditorModel()
	model.width, model.height = 80, 24
	font := Font{Name: "Tiny"}
	font.Glyphs[int('A'-33)] = [][]Cell{{{Rune: 'A', Color: 0x14}}}
	model.font = font
	if err := model.setText("A"); err != nil {
		t.Fatal(err)
	}
	var artRow string
	for _, line := range strings.Split(model.View(), "\n") {
		if strings.Contains(line, "\x1b[0;31;44mA") {
			artRow = ansi.Strip(line)
			break
		}
	}
	if artRow == "" {
		t.Fatal("preview omitted the test glyph")
	}
	position := strings.Index(artRow, "A")
	center := (ansi.StringWidth(artRow) - 1) / 2
	if delta := position - center; delta < -2 || delta > 2 {
		t.Fatalf("centered glyph is at column %d in a %d-column preview row, want near %d", position, ansi.StringWidth(artRow), center)
	}
}

func TestPaletteControlsOverrideForegroundAndBackground(t *testing.T) {
	model := newEditorModel()
	font := Font{Name: "Tiny"}
	font.Glyphs[int('A'-33)] = [][]Cell{{{Rune: 'A', Color: 0x14}}}
	model.font = font
	if err := model.setText("A"); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRight}))
	}
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyDown}))
	if !strings.Contains(model.preview, "\x1b[0;30;44mA\x1b[0m") {
		t.Fatalf("foreground override preview = %q, want black foreground and original blue background", model.preview)
	}

	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRight}))
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyDown}))
	if !strings.Contains(model.preview, "\x1b[0;30;40mA\x1b[0m") {
		t.Fatalf("background override preview = %q, want black foreground and background", model.preview)
	}

	for i := 0; i < 15; i++ {
		model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyDown}))
	}
	if !strings.Contains(model.preview, "\x1b[0;30;107mA\x1b[0m") {
		t.Fatalf("bright background override preview = %q, want bright white background", model.preview)
	}

	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyLeft}))
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyUp}))
	if !strings.Contains(model.preview, "\x1b[0;31;107mA\x1b[0m") {
		t.Fatalf("TDF foreground reset preview = %q, want original red foreground", model.preview)
	}
}

func TestLoadFontsReturnsAllNamedRecords(t *testing.T) {
	path := writeMultiTDF(t, t.TempDir())
	fonts, err := loadFonts(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(fonts) != 2 || fonts[0].Name != "Tiny" || fonts[1].Name != "Wide" {
		t.Fatalf("loadFonts() names = %#v, want Tiny and Wide", fonts)
	}
}

func TestFontPickerOffersNamedFontsFromTDF(t *testing.T) {
	dir := t.TempDir()
	writeMultiTDF(t, dir)
	model := newEditorModel()
	model.mode = modeFontPicker
	model.filePicker = newFilePicker(dir, ".tdf", 10)
	model.Update(model.filePicker.Init()())
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	if model.mode != modeFontChoice || len(model.fonts) != 2 {
		t.Fatalf("font selection state = %v, fonts = %d; want named font choices", model.mode, len(model.fonts))
	}
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyDown}))
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	if model.font.Name != "Wide" {
		t.Fatalf("selected font = %q, want Wide", model.font.Name)
	}
}

func TestFontChoiceSearchSelectsNamedRecord(t *testing.T) {
	dir := t.TempDir()
	writeMultiTDF(t, dir)
	model := newEditorModel()
	model.mode = modeFontPicker
	model.filePicker = newFilePicker(dir, ".tdf", 10)
	model.Update(model.filePicker.Init()())
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'W'}}))
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	if model.font.Name != "Wide" {
		t.Fatalf("filtered font = %q, want Wide", model.font.Name)
	}
}

func TestExportPreviewKeepsProfileEscapeEncoding(t *testing.T) {
	model := newEditorModel()
	font := Font{Name: "Tiny"}
	font.Glyphs[int('A'-33)] = [][]Cell{{{Rune: 'A', Color: 0x14}}}
	model.font = font
	if err := model.setText("A"); err != nil {
		t.Fatal(err)
	}

	snippet := model.exportSnippet()
	if !strings.Contains(snippet, `ascii_1="""`+"\n") || !strings.Contains(snippet, `\033[0;31;44mA\033[0m`) {
		t.Fatalf("export snippet = %q, want escaped ANSI profile content", snippet)
	}
	if strings.ContainsRune(snippet, '\x1b') {
		t.Fatal("export snippet contains a raw escape byte")
	}
}

func TestExportPathCanContainP(t *testing.T) {
	model := newEditorModel()
	model.mode = modeExport
	model.exportFormat = exportSyscGreet
	model.setExportFocus(exportFocusPath)
	path := "/tmp/profile.conf"
	for _, r := range path {
		model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{r}}))
	}
	if model.mode != modeExport {
		t.Fatalf("mode after entering %q = %v, want export form", path, model.mode)
	}
	if got := model.profilePathInput.Value(); got != path {
		t.Fatalf("profile path = %q, want %q", got, path)
	}
}

func TestExportCtrlOBrowsesProfiles(t *testing.T) {
	model := newEditorModel()
	model.mode = modeExport
	model.exportFocus = 0
	model.profilePathInput.Focus()
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlO}))
	if model.mode != modeProfilePicker {
		t.Fatalf("mode after Ctrl+O = %v, want profile picker", model.mode)
	}
}

func TestExportBrowserWorksFromAnyFieldAndForCurrentFormat(t *testing.T) {
	tests := []struct {
		name   string
		format exportFormat
		focus  int
		key    tea.KeyType
		ext    string
	}{
		{name: "profile path", format: exportSyscGreet, focus: exportFocusPath, key: tea.KeyCtrlO, ext: ".conf"},
		{name: "profile slot", format: exportSyscGreet, focus: exportFocusSlot, key: tea.KeyCtrlO, ext: ".conf"},
		{name: "profile format", format: exportSyscGreet, focus: exportFocusFormat, key: tea.KeyCtrlO, ext: ".conf"},
		{name: "file output", format: exportTXT, focus: exportFocusFormat, key: tea.KeyCtrlB, ext: ".txt"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			model := newEditorModel()
			model.width, model.height = 100, 32
			model.mode = modeExport
			model.exportFormat = test.format
			model.exportFocus = test.focus
			model.activeExportPathInput().SetValue(filepath.Join(dir, "new"+test.ext))
			model.Update(tea.KeyMsg(tea.Key{Type: test.key}))
			if model.mode != modeProfilePicker {
				t.Fatalf("mode after browse key = %v, want file picker", model.mode)
			}
			if model.filePicker.CurrentDirectory != dir {
				t.Fatalf("picker directory = %q, want %q", model.filePicker.CurrentDirectory, dir)
			}
			if len(model.filePicker.AllowedTypes) != 1 || model.filePicker.AllowedTypes[0] != test.ext {
				t.Fatalf("picker allowed types = %v, want %q", model.filePicker.AllowedTypes, test.ext)
			}
		})
	}
}

func TestPickerStartDirUsesExistingParentForMissingPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing", "new.conf")
	if got := pickerStartDir(path); got != dir {
		t.Fatalf("pickerStartDir(%q) = %q, want nearest existing directory %q", path, got, dir)
	}
}

func TestExportCreatesMissingSyscGreetProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new-profile.conf")
	model := newEditorModel()
	model.openExport()
	model.profilePathInput.SetValue(path)
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlS}))
	if !strings.HasPrefix(model.status, "Created profile ") {
		t.Fatalf("export status = %q, want created-profile confirmation", model.status)
	}
	if strings.Contains(model.status, "backup:") {
		t.Fatalf("new profile status claims a backup exists: %q", model.status)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("new profile was not created: %v", err)
	}
}

func TestPermissionExportErrorExplainsHowToWriteSystemProfiles(t *testing.T) {
	message := formatExportError(os.ErrPermission)
	if !strings.Contains(message, "sudo") || !strings.Contains(message, "writable path") {
		t.Fatalf("permission error message = %q, want explicit elevation and writable-path guidance", message)
	}
}

func TestExportViewFitsFullScreenTerminal(t *testing.T) {
	model := newEditorModel()
	model.width, model.height = 80, 24
	model.openExport()
	model.profilePathInput.SetValue("/tmp/sysclet-showcase.conf")
	model.refreshAutoSlot()
	view := model.View()
	for _, want := range []string{"OUTPUT FORMAT", "Profile path", "/tmp/sysclet-showcase.conf", "New slot", "Snippet preview", "Ctrl+S"} {
		if !strings.Contains(view, want) {
			t.Errorf("export view does not show %q", want)
		}
	}
	if lines := strings.Count(view, "\n") + 1; lines > model.height {
		t.Fatalf("export view uses %d rows, terminal has %d", lines, model.height)
	}
	for i, line := range strings.Split(view, "\n") {
		if got := ansi.StringWidth(line); got > model.width {
			t.Errorf("export view row %d uses %d columns, terminal has %d: %q", i+1, got, model.width, line)
		}
	}
}

func TestExportViewFitsAtLayoutThreshold(t *testing.T) {
	model := newEditorModel()
	model.width, model.height = 58, 20
	model.openExport()
	model.profilePathInput.SetValue("/tmp/profile.conf")
	view := model.View()
	if lines := strings.Split(view, "\n"); len(lines) > model.height {
		t.Fatalf("export view uses %d rows, terminal has %d", len(lines), model.height)
	}
	for i, line := range strings.Split(view, "\n") {
		if got := ansi.StringWidth(line); got > model.width {
			t.Errorf("export view row %d uses %d columns, terminal has %d", i+1, got, model.width)
		}
	}
	if !strings.Contains(view, "Ctrl+S") {
		t.Fatal("export view hid its save key at the layout threshold")
	}
}

func TestExportViewFitsCompactTerminal(t *testing.T) {
	for _, format := range []exportFormat{exportANSI, exportSyscGreet} {
		t.Run(string(format), func(t *testing.T) {
			model := newEditorModel()
			model.width, model.height = 42, 14
			model.openExport()
			model.exportFormat = format
			model.exportFocus = exportFocusFormat
			model.filePathInput.SetValue("/tmp/header")
			model.profilePathInput.SetValue("/tmp/profile.conf")
			model.previewRaw = strings.Repeat("\x1b[0;97mSYSCLET\x1b[0m\n", 12)
			if format == exportSyscGreet {
				model.exportFocus = exportFocusSlot
				_ = model.View()
				model.slotInput.SetValue("ascii_123456789012345678901234")
				model.slotInput.Blur()
				model.exportFocus = exportFocusFormat
			}

			view := model.View()
			lines := strings.Split(view, "\n")
			if len(lines) > model.height {
				t.Fatalf("export view uses %d rows, terminal has %d:\n%s", len(lines), model.height, view)
			}
			for _, want := range []string{"OUTPUT FORMAT", "Ctrl+S", "Tab formats"} {
				if !strings.Contains(view, want) {
					t.Errorf("compact export view does not show %q:\n%s", want, view)
				}
			}
			if format == exportSyscGreet {
				for _, want := range []string{"Profile", "New slot", "Beams"} {
					if !strings.Contains(view, want) {
						t.Errorf("compact sysc-greet export view does not show %q:\n%s", want, view)
					}
				}
			}
			for i, line := range lines {
				if got := ansi.StringWidth(line); got > model.width {
					t.Errorf("export view row %d uses %d columns, terminal has %d: %q", i+1, got, model.width, line)
				}
			}
		})
	}
}

func TestCompactExportKeepsPermissionRecoveryGuidance(t *testing.T) {
	model := newEditorModel()
	model.width, model.height = 42, 14
	model.mode = modeExport
	model.exportFormat = exportANSI
	model.exportFocus = exportFocusFormat
	model.status = formatExportError(os.ErrPermission)

	view := model.View()
	if !strings.Contains(view, "sudo") || !strings.Contains(view, "writable path") {
		t.Fatalf("compact export hid permission recovery guidance:\n%s", view)
	}
	if !strings.Contains(view, "Ctrl+S") {
		t.Fatalf("compact export hid its save key:\n%s", view)
	}
	if lines := strings.Split(view, "\n"); len(lines) > model.height {
		t.Fatalf("compact export uses %d rows, terminal has %d", len(lines), model.height)
	}
}

func TestCompactExportKeepsControlsVisibleWithLongStatus(t *testing.T) {
	model := newEditorModel()
	model.width, model.height = 42, 14
	model.mode = modeExport
	model.exportFormat = exportANSI
	model.exportFocus = exportFocusFormat
	model.status = "Exported " + strings.Repeat("./very-long-header-name-", 30)

	view := model.View()
	if !strings.Contains(view, "Ctrl+S") {
		t.Fatalf("compact export hid controls behind a long status:\n%s", view)
	}
	if lines := strings.Split(view, "\n"); len(lines) > model.height {
		t.Fatalf("compact export uses %d rows, terminal has %d", len(lines), model.height)
	}
}

func TestExportUpdatesProfileOnlyOnCtrlSAndKeepsBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "greeter.conf")
	original := "name=demo\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	model := newEditorModel()
	font := Font{Name: "Tiny"}
	font.Glyphs[int('A'-33)] = [][]Cell{{{Rune: 'A', Color: 0x14}}}
	model.font = font
	if err := model.setText("A"); err != nil {
		t.Fatal(err)
	}
	model.openExport()
	model.profilePathInput.SetValue(path)

	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	unchanged, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(unchanged) != original {
		t.Fatal("Enter changed the profile before explicit save confirmation")
	}

	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlS}))
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), `ascii_1="""`) {
		t.Fatalf("profile update = %q, missing ascii_1", updated)
	}
	if model.status == "" || !strings.Contains(model.status, "backup:") {
		t.Fatalf("export status = %q, want backup path", model.status)
	}
	statusParts := strings.SplitN(model.status, "backup: ", 2)
	if len(statusParts) != 2 {
		t.Fatalf("export status = %q, missing backup path", model.status)
	}
	backupPath := statusParts[1]
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup %q: %v", backupPath, err)
	}
	if string(backup) != original {
		t.Fatalf("backup = %q, want original profile", backup)
	}
}

func TestExportAddsNextProfileSlotAndReusesItOnSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "greeter.conf")
	original := "ansi_style=true\nascii_1=\"\"\"\nkeep this header\n\"\"\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	model := newEditorModel()
	font := Font{Name: "Tiny"}
	font.Glyphs[int('A'-33)] = [][]Cell{{{Rune: 'A', Color: 0x14}}}
	model.font = font
	if err := model.setText("A"); err != nil {
		t.Fatal(err)
	}
	model.openExport()
	model.profilePathInput.SetValue(path)

	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlS}))
	if got := model.slotInput.Value(); got != "ascii_2" {
		t.Fatalf("saved slot = %q, want next slot ascii_2", got)
	}
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlS}))

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), original) {
		t.Fatalf("existing slot was changed: %q", updated)
	}
	if count := strings.Count(string(updated), "ascii_2=\"\"\""); count != 1 {
		t.Fatalf("ascii_2 block count = %d, want one after two saves", count)
	}
}

func TestEditingProfileSlotReplacesThatSlot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "greeter.conf")
	original := "ansi_style=true\nascii_1=\"\"\"\nreplace this header\n\"\"\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	model := newEditorModel()
	font := Font{Name: "Tiny"}
	font.Glyphs[int('A'-33)] = [][]Cell{{{Rune: 'A', Color: 0x14}}}
	model.font = font
	if err := model.setText("A"); err != nil {
		t.Fatal(err)
	}
	model.openExport()
	model.profilePathInput.SetValue(path)
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	model.slotInput.SetValue("")
	for _, char := range "ascii_1" {
		model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{char}}))
	}
	model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlS}))

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(updated), "replace this header") || !strings.Contains(string(updated), "\\033[") {
		t.Fatalf("selected profile slot was not replaced: %q", updated)
	}
}

func TestDetectSyscGreetProfile(t *testing.T) {
	tests := []struct {
		name       string
		env        map[string]string
		profiles   []string
		wantName   string
		wantChoice string
	}{
		{
			name:       "explicit compositor wins",
			env:        map[string]string{"SYSC_COMPOSITOR": "Plasma", "NIRI_SOCKET": "niri.sock"},
			profiles:   []string{"kde"},
			wantName:   "kde",
			wantChoice: "kde.conf",
		},
		{
			name:       "compositor socket",
			env:        map[string]string{"NIRI_SOCKET": "niri.sock"},
			profiles:   []string{"niri"},
			wantName:   "niri",
			wantChoice: "niri.conf",
		},
		{
			name:       "desktop aliases",
			env:        map[string]string{"XDG_CURRENT_DESKTOP": "GNOME:Ubuntu"},
			profiles:   []string{"gnome_desktop"},
			wantName:   "gnome_desktop",
			wantChoice: "gnome_desktop.conf",
		},
		{
			name:     "unknown desktop",
			env:      map[string]string{"XDG_CURRENT_DESKTOP": "unknown"},
			profiles: []string{"niri"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, profile := range test.profiles {
				path := filepath.Join(dir, profile+".conf")
				if err := os.WriteFile(path, []byte("name=test\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			path, name := detectSyscGreetProfile(test.env, []string{dir})
			if name != test.wantName {
				t.Fatalf("detected compositor = %q, want %q", name, test.wantName)
			}
			if test.wantChoice != "" && filepath.Base(path) != test.wantChoice {
				t.Fatalf("detected profile = %q, want %q", path, test.wantChoice)
			}
		})
	}
}

func writeMultiTDF(t *testing.T, dir string) string {
	t.Helper()
	first := testTDF()
	second := testTDF()
	copy(second[tdfHeaderSize+5:tdfHeaderSize+9], "Wide")
	data := append(append([]byte(nil), first...), second[tdfHeaderSize:]...)
	path := filepath.Join(dir, "multi.tdf")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
