package main

import (
	"os"
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
	model.exportFocus = 0
	model.profilePathInput.Focus()
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

func TestExportViewFitsFullScreenTerminal(t *testing.T) {
	model := newEditorModel()
	model.width, model.height = 153, 43
	model.mode = modeExport
	model.profilePathInput.SetValue("/tmp/sysclet-showcase.conf")
	view := model.View()
	for _, want := range []string{"Profile path", "/tmp/sysclet-showcase.conf", "ASCII slot", "Snippet preview"} {
		if !strings.Contains(view, want) {
			t.Errorf("export view does not show %q", want)
		}
	}
	if lines := strings.Count(view, "\n") + 1; lines > model.height {
		t.Fatalf("export view uses %d rows, terminal has %d", lines, model.height)
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
	model.profilePathInput.SetValue(path)
	model.mode = modeExport

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
