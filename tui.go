package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func defaultFontDir() (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" || !filepath.IsAbs(dataHome) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "sysclet", "fonts"), nil
}

type editorMode uint8

const (
	modeNormal editorMode = iota
	modeFontPicker
	modeFontChoice
	modeExport
	modeProfilePicker
)

const (
	controlText = iota
	controlFont
	controlAlignment
	controlForeground
	controlBackground
	controlExport
	controlCount
)

const (
	alignmentLeft = iota
	alignmentCenter
	alignmentRight
)

const (
	exportFocusFormat = iota
	exportFocusPath
	exportFocusSlot
)

const alignedWidth = 80

// ponytail: Retain at most eight parsed packs to bound memory; raise this cap only if profiling shows more reuse is needed.
const fontPackCacheLimit = 8

var ansiPaletteNames = [...]string{
	"Black", "Red", "Green", "Yellow", "Blue", "Magenta", "Cyan", "White",
	"Bright Black", "Bright Red", "Bright Green", "Bright Yellow",
	"Bright Blue", "Bright Magenta", "Bright Cyan", "Bright White",
}

type editorModel struct {
	textInput             textinput.Model
	fontFilter            textinput.Model
	profilePathInput      textinput.Model
	filePathInput         textinput.Model
	slotInput             textinput.Model
	filePicker            filepicker.Model
	mode                  editorMode
	text                  string
	previewRaw            string
	preview               string
	font                  Font
	fontCatalog           []fontCatalogEntry
	fontPackCache         fontPackCache
	fonts                 []Font
	fontChoices           []int
	fontPath              string
	fontIndex             int
	fontPreview           string
	fontChoiceFromCatalog bool
	fontPickerReturnMode  editorMode
	selectedControl       int
	alignment             int
	foreground            int
	background            int
	exportFocus           int
	exportFormat          exportFormat
	slotAuto              bool
	autoSlotPath          string
	detectedCompositor    string
	writeClipboard        func(string) error
	width                 int
	height                int
	status                string
}

type fontPackCache struct {
	packs map[string][]Font
	order []string
}

func (c *fontPackCache) load(entry fontCatalogEntry) (Font, error) {
	if fonts, ok := c.packs[entry.Pack]; ok {
		c.touch(entry.Pack)
		return bundledFontFromPack(fonts, entry)
	}

	fonts, err := loadBundledFontPack(entry.Pack)
	if err != nil {
		return Font{}, err
	}
	if c.packs == nil {
		c.packs = make(map[string][]Font)
	}
	if len(c.order) == fontPackCacheLimit {
		delete(c.packs, c.order[0])
		c.order = c.order[1:]
	}
	c.packs[entry.Pack] = fonts
	c.order = append(c.order, entry.Pack)
	return bundledFontFromPack(fonts, entry)
}

func (c *fontPackCache) touch(pack string) {
	for i, cached := range c.order {
		if cached == pack {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, pack)
			return
		}
	}
}

func (c *fontPackCache) clear() {
	c.packs = nil
	c.order = nil
}

func newEditorModel() *editorModel {
	textInput := textinput.New()
	textInput.Prompt = "> "
	textInput.Placeholder = "Type a header"
	textInput.CharLimit = 128
	textInput.SetValue("SYSC")
	textInput.Blur()

	fontFilter := textinput.New()
	fontFilter.Prompt = "> "
	fontFilter.Placeholder = "Filter font names"
	fontFilter.CharLimit = 64
	fontFilter.Width = 64
	fontFilter.Blur()

	profilePathInput := textinput.New()
	profilePathInput.Prompt = "> "
	profilePathInput.Placeholder = "/path/to/ascii_configs/compositor.conf"
	profilePathInput.CharLimit = 4096
	profilePathInput.Width = 64
	profilePathInput.Blur()

	filePathInput := textinput.New()
	filePathInput.Prompt = "> "
	filePathInput.Placeholder = "./header"
	filePathInput.CharLimit = 4096
	filePathInput.Width = 64
	filePathInput.Blur()

	slotInput := textinput.New()
	slotInput.Prompt = "> "
	slotInput.Placeholder = "ascii_1"
	slotInput.CharLimit = 32
	slotInput.SetValue("ascii_1")
	slotInput.Blur()

	model := &editorModel{
		textInput:        textInput,
		fontFilter:       fontFilter,
		profilePathInput: profilePathInput,
		filePathInput:    filePathInput,
		slotInput:        slotInput,
		selectedControl:  controlText,
		alignment:        alignmentCenter,
		foreground:       -1,
		background:       -1,
		exportFormat:     exportSyscGreet,
		slotAuto:         true,
		writeClipboard:   clipboard.WriteAll,
	}
	if font, err := loadBundledFontByName("Bleach"); err != nil {
		model.status = "font: " + err.Error()
	} else {
		model.font = font
		model.fontPath = "bundled TheDraw fonts"
	}
	_ = model.setText("SYSC")
	return model
}

func runTUI() int {
	program := tea.NewProgram(newEditorModel(), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "sysclet: %v\n", err)
		return 1
	}
	return 0
}

func (m *editorModel) setText(text string) error {
	m.text = text
	if text == "" || m.font.Name == "" {
		m.previewRaw = ""
		m.preview = ""
		if text == "" {
			m.status = ""
		}
		return nil
	}
	preview, err := m.font.RenderWithColors(text, m.foreground, m.background)
	if err != nil {
		m.previewRaw = ""
		m.preview = ""
		m.status = "text: " + err.Error()
		return err
	}
	m.previewRaw = preview
	m.preview = alignANSI(preview, m.alignment, alignedWidth)
	m.status = ""
	return nil
}

func (m *editorModel) Init() tea.Cmd {
	return nil
}

func (m *editorModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.mode == modeFontPicker || m.mode == modeProfilePicker {
			m.filePicker.SetHeight(minInt(24, maxInt(6, msg.Height-12)))
		}
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.mode {
		case modeFontPicker:
			return m.updateFontPicker(msg)
		case modeFontChoice:
			return m.updateFontChoice(msg)
		case modeExport:
			return m.updateExport(msg)
		case modeProfilePicker:
			return m.updateProfilePicker(msg)
		default:
			if m.editingText() {
				if msg.String() == "enter" || msg.String() == "esc" {
					m.textInput.Blur()
					return m, nil
				}
				var command tea.Cmd
				m.textInput, command = m.textInput.Update(msg)
				_ = m.setText(m.textInput.Value())
				return m, command
			}
			return m.updateControls(msg)
		}
	default:
		var command tea.Cmd
		switch m.mode {
		case modeFontPicker, modeProfilePicker:
			m.filePicker, command = m.filePicker.Update(message)
		case modeFontChoice:
			m.fontFilter, command = m.fontFilter.Update(message)
		case modeExport:
			if m.exportFocus == exportFocusPath {
				pathInput := m.activeExportPathInput()
				*pathInput, command = pathInput.Update(message)
			} else if m.exportFocus == exportFocusSlot {
				m.slotInput, command = m.slotInput.Update(message)
			}
		default:
			if m.editingText() {
				m.textInput, command = m.textInput.Update(message)
				_ = m.setText(m.textInput.Value())
			}
		}
		return m, command
	}
}

func (m *editorModel) editingText() bool {
	return m.textInput.Focused()
}

func (m *editorModel) updateControls(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		return m, tea.Quit
	case "tab":
		m.selectedControl = (m.selectedControl + 1) % controlCount
	case "shift+tab":
		m.selectedControl = (m.selectedControl + controlCount - 1) % controlCount
	case "left", "h":
		m.selectedControl = (m.selectedControl + controlCount - 1) % controlCount
	case "right", "l":
		m.selectedControl = (m.selectedControl + 1) % controlCount
	case "up", "k":
		m.adjustControl(-1)
	case "down", "j":
		m.adjustControl(1)
	case "e":
		return m, m.openExport()
	case "enter":
		switch m.selectedControl {
		case controlText:
			m.textInput.SetValue(m.text)
			return m, m.textInput.Focus()
		case controlFont:
			return m.openFontPicker()
		case controlExport:
			return m, m.openExport()
		default:
			m.adjustControl(1)
		}
	}
	return m, nil
}

func (m *editorModel) adjustControl(step int) {
	switch m.selectedControl {
	case controlFont:
		m.cycleFont(step)
		return
	case controlAlignment:
		m.alignment = (m.alignment + step + 3) % 3
	case controlForeground:
		m.foreground = cyclePalette(m.foreground, step)
	case controlBackground:
		m.background = cyclePalette(m.background, step)
	default:
		return
	}
	_ = m.setText(m.text)
}

func cyclePalette(current, step int) int {
	state := (current + 1 + step + 17) % 17
	return state - 1
}

func alignANSI(art string, alignment, width int) string {
	if art == "" || alignment == alignmentLeft || width <= 0 {
		return art
	}
	lines := strings.Split(art, "\n")
	if alignment == alignmentCenter {
		blockWidth := 0
		for _, line := range lines {
			if lineWidth := ansi.StringWidth(line); lineWidth > blockWidth {
				blockWidth = lineWidth
			}
		}
		rowWidth := maxInt(width, blockWidth)
		left := 0
		if blockWidth < width {
			left = (width - blockWidth) / 2
		}
		for i, line := range lines {
			lineWidth := ansi.StringWidth(line)
			lines[i] = strings.Repeat(" ", left) + line + strings.Repeat(" ", rowWidth-left-lineWidth)
		}
		return strings.Join(lines, "\n")
	}
	for i, line := range lines {
		lineWidth := ansi.StringWidth(line)
		if lineWidth >= width {
			continue
		}
		padding := width - lineWidth
		lines[i] = strings.Repeat(" ", padding) + line
	}
	return strings.Join(lines, "\n")
}

func (m *editorModel) cycleFont(step int) {
	if len(m.fonts) > 0 && !m.fontChoiceFromCatalog {
		index := (m.fontIndex + step%len(m.fonts) + len(m.fonts)) % len(m.fonts)
		m.selectFont(index)
		return
	}
	if len(m.fontCatalog) == 0 {
		catalog, err := loadBundledFontCatalog()
		if err != nil {
			m.status = "font catalog: " + err.Error()
			return
		}
		m.fontCatalog = catalog
	}
	index := -1
	if m.fontChoiceFromCatalog {
		index = m.fontIndex
	} else {
		for i, entry := range m.fontCatalog {
			if entry.Name == m.font.Name {
				index = i
				break
			}
		}
	}
	if index < 0 || index >= len(m.fontCatalog) {
		index = 0
	}
	index = (index + step%len(m.fontCatalog) + len(m.fontCatalog)) % len(m.fontCatalog)
	m.fontChoiceFromCatalog = true
	m.selectFont(index)
}

func (m *editorModel) openFontPicker() (tea.Model, tea.Cmd) {
	m.fontPackCache.clear()
	catalog, err := loadBundledFontCatalog()
	if err != nil {
		m.status = "font catalog: " + err.Error()
		return m, nil
	}
	m.fontCatalog = catalog
	m.fonts = nil
	m.fontChoiceFromCatalog = true
	m.fontPickerReturnMode = modeNormal
	m.fontChoices = make([]int, len(catalog))
	for i := range catalog {
		m.fontChoices[i] = i
	}
	m.fontIndex = 0
	m.fontFilter.SetValue("")
	m.mode = modeFontChoice
	m.status = ""
	m.refreshFontPreview()
	return m, m.fontFilter.Focus()
}

func (m *editorModel) openLocalFontPicker() (tea.Model, tea.Cmd) {
	dir, err := defaultFontDir()
	if err != nil {
		m.status = "font folder: " + err.Error()
		return m, nil
	}
	m.fontPickerReturnMode = modeNormal
	if m.mode == modeFontChoice {
		m.fontPickerReturnMode = modeFontChoice
	}
	m.filePicker = newFilePicker(pickerStartDir(dir), ".tdf", m.height)
	m.mode = modeFontPicker
	m.status = "Choose a local .tdf font file"
	return m, m.filePicker.Init()
}

func (m *editorModel) openExport() tea.Cmd {
	m.mode = modeExport
	m.exportFocus = exportFocusFormat
	m.status = ""
	if m.exportFormat == exportSyscGreet {
		m.slotAuto = true
		m.autoSlotPath = ""
		m.detectedCompositor = ""
		m.detectDefaultSyscGreetProfile()
		m.refreshAutoSlot()
	}
	m.slotInput.Blur()
	return m.setExportFocus(m.exportFocus)
}

func (m *editorModel) openExportPicker() (tea.Model, tea.Cmd) {
	start := m.activeExportPathInput().Value()
	if start == "" {
		start = "."
		if m.exportFormat == exportSyscGreet {
			start = existingProfileDir()
		}
	}
	extension := exportExtension(m.exportFormat)
	if m.exportFormat == exportSyscGreet {
		extension = ".conf"
	}
	m.filePicker = newFilePicker(pickerStartDir(start), extension, m.height)
	m.mode = modeProfilePicker
	return m, m.filePicker.Init()
}

func newFilePicker(dir, extension string, height int) filepicker.Model {
	picker := filepicker.New()
	picker.CurrentDirectory = dir
	picker.AllowedTypes = []string{extension}
	picker.ShowPermissions = false
	picker.ShowSize = false
	if height > 0 {
		picker.SetHeight(minInt(24, maxInt(6, height-12)))
	}
	return picker
}

func pickerStartDir(path string) string {
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return path
		}
		return filepath.Dir(path)
	}
	path = filepath.Dir(path)
	for {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func profileConfigDirs() []string {
	candidates := []string{"/usr/share/sysc-greet/ascii_configs"}
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" && filepath.IsAbs(configHome) {
		candidates = append(candidates, filepath.Join(configHome, "sysc-greet", "ascii_configs"))
	} else if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "sysc-greet", "ascii_configs"))
	}
	return candidates
}

func existingProfileDir() string {
	for _, candidate := range profileConfigDirs() {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func currentSyscGreetProfile() (string, string) {
	env := make(map[string]string)
	for _, key := range []string{
		"SYSC_COMPOSITOR", "NIRI_SOCKET", "SWAYSOCK", "HYPRLAND_INSTANCE_SIGNATURE",
		"XDG_CURRENT_DESKTOP", "XDG_SESSION_DESKTOP", "DESKTOP_SESSION",
	} {
		if value := os.Getenv(key); value != "" {
			env[key] = value
		}
	}
	return detectSyscGreetProfile(env, profileConfigDirs())
}

func detectSyscGreetProfile(env map[string]string, dirs []string) (string, string) {
	var names []string
	seen := make(map[string]bool)
	addNames := func(value string) {
		for _, part := range strings.FieldsFunc(value, func(char rune) bool {
			return char == ':' || char == ';' || char == ',' || char == '/' || char == ' '
		}) {
			name := normalizeCompositorProfileName(part)
			if name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	addNames(env["SYSC_COMPOSITOR"])
	for _, key := range []string{"NIRI_SOCKET", "SWAYSOCK", "HYPRLAND_INSTANCE_SIGNATURE"} {
		if env[key] != "" {
			name := ""
			switch key {
			case "NIRI_SOCKET":
				name = "niri"
			case "SWAYSOCK":
				name = "sway"
			case "HYPRLAND_INSTANCE_SIGNATURE":
				name = "hyprland"
			}
			addNames(name)
		}
	}
	for _, key := range []string{"XDG_CURRENT_DESKTOP", "XDG_SESSION_DESKTOP", "DESKTOP_SESSION"} {
		addNames(env[key])
	}
	for _, name := range names {
		for _, dir := range dirs {
			path := filepath.Join(dir, name+".conf")
			info, err := os.Lstat(path)
			if err == nil && info.Mode().IsRegular() {
				return path, name
			}
		}
	}
	return "", ""
}

func normalizeCompositorProfileName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "gnome":
		return "gnome_desktop"
	case "kde", "plasma":
		return "kde"
	case "i3":
		return "i3wm"
	case "bspwm":
		return "bspwm_manager"
	}
	if name == "" {
		return ""
	}
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-') {
			return ""
		}
	}
	return name
}

func (m *editorModel) updateFontPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.mode = m.fontPickerReturnMode
		m.status = ""
		if m.mode == modeFontChoice {
			return m, m.fontFilter.Focus()
		}
		return m, nil
	}
	var command tea.Cmd
	m.filePicker, command = m.filePicker.Update(msg)
	if selected, path := m.filePicker.DidSelectFile(msg); selected {
		fonts, err := loadFonts(path)
		if err != nil {
			m.mode = modeNormal
			m.status = "font: " + err.Error()
			m.fontPackCache.clear()
			return m, command
		}
		m.fontPath = path
		m.fonts = fonts
		m.fontIndex = 0
		m.fontChoiceFromCatalog = false
		m.fontPickerReturnMode = modeNormal
		if len(fonts) == 1 {
			m.selectFont(0)
		} else {
			m.fontPackCache.clear()
			m.fontFilter.SetValue("")
			m.fontChoices = make([]int, len(fonts))
			for i := range fonts {
				m.fontChoices[i] = i
			}
			m.mode = modeFontChoice
			m.status = "This file contains multiple fonts"
			m.refreshFontPreview()
			return m, tea.Batch(command, m.fontFilter.Focus())
		}
	}
	return m, command
}

func (m *editorModel) updateFontChoice(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.fontFilter.Blur()
		m.mode = modeNormal
		m.status = ""
		m.fontPackCache.clear()
	case "ctrl+o":
		return m.openLocalFontPicker()
	case "up":
		if m.fontIndex > 0 {
			m.fontIndex--
			m.refreshFontPreview()
		}
	case "down":
		if m.fontIndex+1 < len(m.fontChoices) {
			m.fontIndex++
			m.refreshFontPreview()
		}
	case "enter":
		if len(m.fontChoices) > 0 {
			m.selectFont(m.fontChoices[m.fontIndex])
		}
	default:
		var command tea.Cmd
		m.fontFilter, command = m.fontFilter.Update(msg)
		m.filterFontChoices()
		return m, command
	}
	return m, nil
}

func (m *editorModel) filterFontChoices() {
	query := strings.ToLower(strings.TrimSpace(m.fontFilter.Value()))
	m.fontChoices = m.fontChoices[:0]
	if m.fontChoiceFromCatalog {
		for i, entry := range m.fontCatalog {
			if query == "" || strings.Contains(strings.ToLower(entry.Name), query) {
				m.fontChoices = append(m.fontChoices, i)
			}
		}
	} else {
		for i, font := range m.fonts {
			if query == "" || strings.Contains(strings.ToLower(font.Name), query) {
				m.fontChoices = append(m.fontChoices, i)
			}
		}
	}
	m.fontIndex = 0
	m.refreshFontPreview()
}

func (m *editorModel) selectFont(index int) {
	var font Font
	if m.fontChoiceFromCatalog {
		if index < 0 || index >= len(m.fontCatalog) {
			return
		}
		var err error
		font, err = m.fontPackCache.load(m.fontCatalog[index])
		if err != nil {
			m.status = "font: " + err.Error()
			return
		}
		m.fontPath = "bundled TDF catalog"
	} else {
		if index < 0 || index >= len(m.fonts) {
			return
		}
		font = m.fonts[index]
	}
	m.fontIndex = index
	m.font = font
	m.fontPackCache.clear()
	m.fontFilter.Blur()
	m.mode = modeNormal
	_ = m.setText(m.text)
	if m.status == "" {
		m.status = fmt.Sprintf("Loaded %s · %s", filepath.Base(m.fontPath), m.font.Name)
	}
}

func (m *editorModel) refreshFontPreview() {
	m.fontPreview = ""
	if m.fontIndex < 0 || m.fontIndex >= len(m.fontChoices) {
		return
	}

	choice := m.fontChoices[m.fontIndex]
	var font Font
	if m.fontChoiceFromCatalog {
		if choice < 0 || choice >= len(m.fontCatalog) {
			return
		}
		var err error
		font, err = m.fontPackCache.load(m.fontCatalog[choice])
		if err != nil {
			m.fontPreview = "Preview unavailable: " + err.Error()
			return
		}
	} else {
		if choice < 0 || choice >= len(m.fonts) {
			return
		}
		font = m.fonts[choice]
	}

	preview, err := font.RenderWithColors(m.text, m.foreground, m.background)
	if err != nil {
		m.fontPreview = "Preview unavailable: " + err.Error()
		return
	}
	m.fontPreview = preview
}

func (m *editorModel) updateExport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.profilePathInput.Blur()
		m.filePathInput.Blur()
		m.slotInput.Blur()
		m.status = ""
		return m, nil
	case "tab":
		if m.exportFocus == exportFocusFormat {
			m.cycleExportFormat(1)
			return m, nil
		}
		if m.exportFormat == exportSyscGreet && m.exportFocus == exportFocusPath {
			m.refreshAutoSlot()
		}
		return m, m.setExportFocus(m.nextExportFocus(1))
	case "shift+tab":
		if m.exportFocus == exportFocusFormat {
			m.cycleExportFormat(-1)
			return m, nil
		}
		if m.exportFormat == exportSyscGreet && m.exportFocus == exportFocusPath {
			m.refreshAutoSlot()
		}
		return m, m.setExportFocus(m.nextExportFocus(-1))
	case "ctrl+o":
		return m.openExportPicker()
	case "ctrl+b":
		return m.openExportPicker()
	case "ctrl+s":
		m.saveExport()
		return m, nil
	case "f5":
		if m.exportFormat != exportSyscGreet {
			return m, nil
		}
		if err := m.refreshAutoSlot(); err != nil {
			return m, nil
		}
		if m.preview == "" {
			m.status = "Choose a TDF font and enter text before copying"
			return m, nil
		}
		snippet, err := ProfileSnippet(strings.TrimSpace(m.slotInput.Value()), m.preview)
		if err != nil {
			m.status = "Cannot copy snippet: " + err.Error()
			return m, nil
		}
		if err := m.writeClipboard(snippet); err != nil {
			m.status = "Clipboard copy failed: " + err.Error()
			return m, nil
		}
		m.status = "Copied sysc-greet snippet to clipboard"
		return m, nil
	case "enter":
		if m.exportFocus == exportFocusFormat {
			return m, m.setExportFocus(exportFocusPath)
		}
		if m.exportFormat == exportSyscGreet && m.exportFocus == exportFocusPath {
			m.refreshAutoSlot()
		}
		return m, m.setExportFocus(m.nextExportFocus(1))
	case "up", "k", "left", "h":
		if m.exportFocus == exportFocusFormat {
			m.cycleExportFormat(-1)
			return m, nil
		}
	case "down", "j", "right", "l":
		if m.exportFocus == exportFocusFormat {
			m.cycleExportFormat(1)
			return m, nil
		}
	}
	var command tea.Cmd
	if m.exportFocus == exportFocusPath {
		pathInput := m.activeExportPathInput()
		oldPath := pathInput.Value()
		*pathInput, command = pathInput.Update(msg)
		if pathInput.Value() != oldPath && m.exportFormat == exportSyscGreet {
			m.slotAuto = true
			m.autoSlotPath = ""
			m.detectedCompositor = ""
		}
	} else if m.exportFocus == exportFocusSlot {
		oldSlot := m.slotInput.Value()
		m.slotInput, command = m.slotInput.Update(msg)
		if m.slotInput.Value() != oldSlot {
			m.slotAuto = false
		}
	}
	return m, command
}

func (m *editorModel) syncAutoSlot() error {
	if !m.slotAuto {
		return nil
	}
	path := strings.TrimSpace(m.profilePathInput.Value())
	if path == m.autoSlotPath {
		return nil
	}
	slot, err := nextProfileSlot(path)
	if err != nil {
		return err
	}
	m.slotInput.SetValue(slot)
	m.autoSlotPath = path
	return nil
}

func (m *editorModel) refreshAutoSlot() error {
	m.autoSlotPath = ""
	if err := m.syncAutoSlot(); err != nil {
		m.status = "Cannot choose a new profile slot: " + err.Error()
		return err
	}
	return nil
}

func (m *editorModel) setExportFocus(index int) tea.Cmd {
	if index == exportFocusSlot && m.exportFormat != exportSyscGreet {
		index = exportFocusPath
	}
	m.exportFocus = index
	m.profilePathInput.Blur()
	m.filePathInput.Blur()
	m.slotInput.Blur()
	switch index {
	case exportFocusPath:
		return m.activeExportPathInput().Focus()
	case exportFocusSlot:
		return m.slotInput.Focus()
	default:
		return nil
	}
}

func (m *editorModel) nextExportFocus(step int) int {
	switch m.exportFocus {
	case exportFocusPath:
		if step < 0 {
			return exportFocusFormat
		}
		if m.exportFormat == exportSyscGreet {
			return exportFocusSlot
		}
		return exportFocusFormat
	case exportFocusSlot:
		if step < 0 {
			return exportFocusPath
		}
		return exportFocusFormat
	default:
		return exportFocusPath
	}
}

func (m *editorModel) cycleExportFormat(step int) {
	previous := m.exportFormat
	m.exportFormat = cycleExportFormat(m.exportFormat, step)
	if previous != exportSyscGreet && m.exportFormat == exportSyscGreet {
		m.detectDefaultSyscGreetProfile()
		m.refreshAutoSlot()
	}
}

func (m *editorModel) detectDefaultSyscGreetProfile() {
	if strings.TrimSpace(m.profilePathInput.Value()) != "" {
		return
	}
	path, compositor := currentSyscGreetProfile()
	if path != "" {
		m.profilePathInput.SetValue(path)
		m.detectedCompositor = compositor
	}
}

func (m *editorModel) activeExportPathInput() *textinput.Model {
	if m.exportFormat == exportSyscGreet {
		return &m.profilePathInput
	}
	return &m.filePathInput
}

func (m *editorModel) updateProfilePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.mode = modeExport
		return m, m.setExportFocus(exportFocusPath)
	}
	var command tea.Cmd
	m.filePicker, command = m.filePicker.Update(msg)
	if selected, path := m.filePicker.DidSelectFile(msg); selected {
		m.activeExportPathInput().SetValue(path)
		if m.exportFormat == exportSyscGreet {
			m.slotAuto = true
			m.autoSlotPath = ""
			m.detectedCompositor = ""
		}
		m.mode = modeExport
		if m.exportFormat == exportSyscGreet {
			m.refreshAutoSlot()
		}
		return m, tea.Batch(command, m.setExportFocus(exportFocusPath))
	}
	return m, command
}

func (m *editorModel) saveExport() {
	if m.preview == "" {
		m.status = "Choose a TDF font and enter text before exporting"
		return
	}
	path := strings.TrimSpace(m.activeExportPathInput().Value())
	if path == "" {
		if m.exportFormat == exportSyscGreet {
			m.status = "Enter a sysc-greet profile path or press Ctrl+O/Ctrl+B to browse"
		} else {
			m.status = "Enter an output path"
		}
		return
	}
	if m.exportFormat == exportSyscGreet {
		if m.slotAuto {
			slot, backup, err := AppendProfile(path, m.preview)
			if err != nil {
				m.status = formatExportError(err)
				return
			}
			m.slotInput.SetValue(slot)
			if backup == "" {
				m.status = fmt.Sprintf("Created profile %s · added %s", path, slot)
			} else {
				m.status = fmt.Sprintf("Updated %s · added %s · backup: %s", path, slot, backup)
			}
			m.slotAuto = false
			m.autoSlotPath = path
			return
		}
		backup, err := UpdateProfile(path, strings.TrimSpace(m.slotInput.Value()), m.preview)
		if err != nil {
			m.status = formatExportError(err)
			return
		}
		if backup == "" {
			m.status = "Created profile " + path
			return
		}
		m.status = fmt.Sprintf("Updated %s · backup: %s", path, backup)
		return
	}
	if m.exportFormat == exportSyscWalls || m.exportFormat == exportSyscTerm {
		path, backup, configBackup, err := saveSyscArt(path, m.exportFormat, m.previewRaw)
		if err != nil {
			m.status = formatExportError(err)
			return
		}
		m.status = "Exported " + path
		if backup != "" {
			m.status += " · backup: " + backup
		}
		if m.exportFormat == exportSyscWalls {
			m.status += " · sysc-walls config updated; restart sysc-walls.service"
			if configBackup != "" {
				m.status += " · config backup: " + configBackup
			}
		}
		return
	}
	content, err := generateExportContent(m.exportFormat, "", m.previewRaw)
	if err != nil {
		m.status = formatExportError(err)
		return
	}
	path, backup, err := writeExportFile(path, m.exportFormat, content)
	if err != nil {
		m.status = formatExportError(err)
		return
	}
	if backup != "" {
		m.status = fmt.Sprintf("Exported %s · backup: %s", path, backup)
		return
	}
	m.status = "Exported " + path
}

func (m *editorModel) exportSnippet() string {
	if m.preview == "" {
		return "Choose a TDF font and enter supported text to preview the profile snippet."
	}
	snippet, err := ProfileSnippet(strings.TrimSpace(m.slotInput.Value()), m.preview)
	if err != nil {
		return err.Error()
	}
	return snippet
}

func (m *editorModel) View() string {
	width, height := m.width, m.height
	if m.mode == modeExport {
		if width < 1 {
			width = 80
		}
		if height < 1 {
			height = 24
		}
		return m.exportView(width, height)
	}
	if width < 48 {
		width = 80
	}
	if height < 18 {
		height = 24
	}
	if m.mode == modeFontPicker {
		return m.modalView("Select a TDF font", m.filePicker.View(), "↑/↓ navigate · Enter select · Esc close")
	}
	if m.mode == modeFontChoice {
		return m.fontChoiceView(width, height)
	}
	if m.mode == modeProfilePicker {
		title := "Choose an output file"
		if m.exportFormat == exportSyscGreet {
			title = "Choose a sysc-greet compositor profile"
		}
		return m.modalView(title, m.filePicker.View(), "↑/↓ navigate · Enter select · Esc back")
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("45")).Render("sysclet") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("  //  ANSI HEADER STUDIO")
	fontLabel := "Choose TDF font"
	if m.font.Name != "" {
		fontLabel = m.font.Name
	}
	fontInfo := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("TDF FONT  " + fontLabel)
	controlNames := [...]string{"TEXT", "TDF FONT", "ALIGN", "FG", "BG", "EXPORT"}
	controls := []string{
		m.controlView(controlText, controlNames[controlText], m.text),
		m.controlView(controlFont, controlNames[controlFont], "↑/↓"),
		m.controlView(controlAlignment, controlNames[controlAlignment], alignmentLabel(m.alignment)),
		m.controlView(controlForeground, controlNames[controlForeground], colorLabel(m.foreground)),
		m.controlView(controlBackground, controlNames[controlBackground], colorLabel(m.background)),
		m.controlView(controlExport, controlNames[controlExport], string(m.exportFormat)),
	}
	controlStrip := lipgloss.JoinHorizontal(lipgloss.Top, controls...)
	selectedInfo := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(lipgloss.Color("45")).Padding(0, 1).Render("SELECTED: " + controlNames[m.selectedControl])

	panelHeight := height - 12
	if panelHeight < 4 {
		panelHeight = 4
	}
	availableWidth := maxInt(0, width-12)
	art := alignANSI(m.previewRaw, m.alignment, minInt(alignedWidth, availableWidth))
	hasArt := art != ""
	if !hasArt {
		message := "Select a local .tdf font to begin\n\nTDF files in ~/.local/share/sysclet/fonts appear here"
		if fontDir, err := defaultFontDir(); err == nil {
			message = "Select a local .tdf font to begin\n\nFont folder: " + fontDir
		}
		art = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Align(lipgloss.Center).Render(message)
	} else {
		artWidth := 0
		for _, line := range strings.Split(art, "\n") {
			if lineWidth := ansi.StringWidth(line); lineWidth > artWidth {
				artWidth = lineWidth
			}
		}
		offset := 0
		if artWidth < availableWidth {
			switch m.alignment {
			case alignmentCenter:
				offset = (availableWidth - artWidth) / 2
			case alignmentRight:
				offset = availableWidth - artWidth
			}
		}
		art = indentANSI(art, offset)
	}
	if artHeight := lipgloss.Height(art); artHeight < panelHeight-2 {
		art = strings.Repeat("\n", (panelHeight-2-artHeight)/2) + art
	}
	previewStyle := lipgloss.NewStyle().
		Width(width-8).
		Height(panelHeight).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238"))
	if !hasArt {
		previewStyle = previewStyle.Align(lipgloss.Center)
	}
	preview := previewStyle.Render(art)

	inputLine := ""
	if m.editingText() {
		inputLine = lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Render("TEXT  ") + m.textInput.View()
	}
	status := ""
	if m.status != "" {
		status = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render(m.status)
	}
	help := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Tab/Shift+Tab or ←/→ select · ↑/↓ adjust · Enter edit/open · e export · q quit")
	if m.editingText() {
		help = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Type to rerender · Enter/Esc finish · Ctrl+C quit")
	}
	parts := []string{title, preview, fontInfo, selectedInfo, controlStrip}
	if inputLine != "" {
		parts = append(parts, inputLine)
	}
	if status != "" {
		parts = append(parts, status)
	}
	parts = append(parts, help)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func alignmentLabel(alignment int) string {
	switch alignment {
	case alignmentLeft:
		return "Left"
	case alignmentRight:
		return "Right"
	default:
		return "Center"
	}
}

func indentANSI(art string, spaces int) string {
	if art == "" || spaces <= 0 {
		return art
	}
	indent := strings.Repeat(" ", spaces)
	lines := strings.Split(art, "\n")
	for i := range lines {
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n")
}

func colorLabel(color int) string {
	if color < 0 || color >= len(ansiPaletteNames) {
		return "TDF"
	}
	return ansiPaletteNames[color]
}

func (m *editorModel) modalView(title, content, help string) string {
	width := m.width
	height := m.height
	if width < 48 {
		width = 80
	}
	if height < 18 {
		height = 24
	}
	boxWidth := minInt(96, width-8)
	box := lipgloss.NewStyle().
		Width(boxWidth).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("45"))
	modal := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("45")).Render("sysclet  //  "+title),
		box.Render(content),
		lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(help),
	)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, modal)
}

func (m *editorModel) fontChoiceView(width, height int) string {
	contentWidth := minInt(92, width-12)
	listWidth := (contentWidth - 2) / 2
	previewWidth := contentWidth - listWidth - 2
	rows := maxInt(1, height-14)
	start := 0
	if m.fontIndex >= rows {
		start = m.fontIndex - rows + 1
	}
	end := minInt(len(m.fontChoices), start+rows)
	var choices strings.Builder
	if start > 0 {
		choices.WriteString("  ↑ more fonts\n")
	}
	for i := start; i < end; i++ {
		marker := "  "
		if i == m.fontIndex {
			marker = "› "
		}
		var name string
		if m.fontChoiceFromCatalog {
			name = m.fontCatalog[m.fontChoices[i]].Name
		} else {
			name = m.fonts[m.fontChoices[i]].Name
		}
		name = ansi.Truncate(name, maxInt(1, listWidth-2), "…")
		fmt.Fprintf(&choices, "%s%s\n", marker, name)
	}
	if end < len(m.fontChoices) {
		choices.WriteString("  ↓ more fonts\n")
	}
	if len(m.fontChoices) == 0 {
		choices.WriteString("No font names match this filter.")
	}
	selectedName := ""
	if m.fontIndex >= 0 && m.fontIndex < len(m.fontChoices) {
		choice := m.fontChoices[m.fontIndex]
		if m.fontChoiceFromCatalog && choice >= 0 && choice < len(m.fontCatalog) {
			selectedName = m.fontCatalog[choice].Name
		} else if !m.fontChoiceFromCatalog && choice >= 0 && choice < len(m.fonts) {
			selectedName = m.fonts[choice].Name
		}
	}
	preview := m.fontPreview
	if preview == "" {
		preview = "No font selected"
	}
	preview = clipExportPreview(preview, previewWidth, maxInt(1, rows-1))
	previewView := lipgloss.NewStyle().Width(previewWidth).Render(lipgloss.JoinVertical(
		lipgloss.Left,
		"Preview: "+selectedName,
		preview,
	))
	choiceView := lipgloss.NewStyle().Width(listWidth).Render(choices.String())
	columns := lipgloss.JoinHorizontal(lipgloss.Top, choiceView, "  ", previewView)
	content := lipgloss.JoinVertical(lipgloss.Left, "Filter: "+m.fontFilter.View(), "", columns)
	title := "Choose a font in " + filepath.Base(m.fontPath)
	help := "Type to filter · ↑/↓ navigate · Enter load · Esc back"
	if m.fontChoiceFromCatalog {
		title = "Choose a bundled TDF font"
		help = "Type to filter · ↑/↓ navigate · Enter load · Ctrl+O browse local .tdf · Esc back"
	}
	return m.modalView(title, content, help)
}

func (m *editorModel) exportView(width, height int) string {
	if width < 58 || height < 20 {
		return m.compactExportView(width, height)
	}

	previewLines := maxInt(0, height-18)
	for {
		view := m.fullExportView(width, previewLines)
		if lipgloss.Height(view) <= height {
			return view
		}
		if previewLines == 0 {
			return m.compactExportView(width, height)
		}
		previewLines--
	}
}

func (m *editorModel) fullExportView(width, previewLines int) string {
	tabs := make([]string, 0, len(exportOptions))
	for _, option := range exportOptions {
		style := lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("244"))
		if option.format == m.exportFormat {
			style = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("16")).Background(lipgloss.Color("45")).Bold(true)
		}
		tabs = append(tabs, style.Render(string(option.format)))
	}
	contentParts := []string{
		exportFieldLabel("OUTPUT FORMAT", m.exportFocus == exportFocusFormat),
		lipgloss.NewStyle().Width(maxInt(1, width-14)).Render(strings.Join(tabs, " ")),
		"",
	}
	if m.exportFormat == exportSyscGreet {
		profileHint := "No compositor detected · Ctrl+O choose a sysc-greet profile"
		if m.detectedCompositor != "" {
			profileHint = "Detected compositor: " + m.detectedCompositor + " · Ctrl+O choose another"
		} else if strings.TrimSpace(m.profilePathInput.Value()) != "" {
			profileHint = "Sysc-greet profile · edit path or Ctrl+O choose another"
		}
		contentParts = append(contentParts, lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(profileHint))
	}
	pathLabel := "Output path"
	pathInput := m.activeExportPathInput()
	if m.exportFormat == exportSyscGreet {
		pathLabel = "Profile path"
	}
	contentWidth := maxInt(1, width-14)
	pathInput.Width = maxInt(1, minInt(64, contentWidth-lipgloss.Width(pathLabel)-4))
	contentParts = append(contentParts, exportFieldLabel(pathLabel, m.exportFocus == exportFocusPath)+"  "+pathInput.View())
	if m.exportFormat == exportSyscGreet {
		slotLabel := "New slot · auto"
		if !m.slotAuto {
			slotLabel = "Replace slot"
		}
		m.slotInput.Width = maxInt(1, minInt(32, contentWidth-lipgloss.Width(slotLabel)-4))
		contentParts = append(contentParts, exportFieldLabel(slotLabel, m.exportFocus == exportFocusSlot)+"  "+m.slotInput.View())
	}

	previewLabel := "Source preview · run the generated file in a terminal"
	preview := ""
	switch m.exportFormat {
	case exportSyscGreet:
		previewLabel = "Snippet preview · ANSI colors shown · profile stores \\033 escapes"
		preview = strings.ReplaceAll(m.exportSnippet(), `\033`, "\x1b")
	case exportANSI:
		previewLabel = "ANSI preview · escape sequences are preserved"
		preview = m.previewRaw
	case exportTXT, exportSyscWalls, exportSyscTerm:
		previewLabel = "Plain text preview · ANSI colors are removed"
		preview, _ = generateExportContent(m.exportFormat, "", m.previewRaw)
	default:
		var err error
		preview, err = generateExportContent(m.exportFormat, "", m.previewRaw)
		if err != nil {
			preview = err.Error()
		}
	}
	preview = clipExportPreview(preview, contentWidth, previewLines)
	contentParts = append(contentParts,
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(previewLabel),
		lipgloss.NewStyle().Width(contentWidth).Foreground(lipgloss.Color("252")).Render(preview),
	)
	hint := exportDisplayHint(m.exportFormat, pathInput.Value())
	if hint != "" {
		contentParts = append(contentParts, lipgloss.NewStyle().Width(contentWidth).Foreground(lipgloss.Color("45")).Render(hint))
	}
	if m.status != "" {
		contentParts = append(contentParts, lipgloss.NewStyle().Width(contentWidth).Foreground(lipgloss.Color("220")).Render(m.status))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, contentParts...)
	box := lipgloss.NewStyle().Width(width-14).Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("45"))
	help := "Tab/Shift+Tab format · Enter destination · Ctrl+O/B browse · Ctrl+S save · Esc back"
	if m.exportFocus == exportFocusPath {
		help = "Tab/Shift+Tab fields · Enter next · Ctrl+O/B browse · Ctrl+S save · Esc back"
	} else if m.exportFocus == exportFocusSlot {
		help = "Tab/Shift+Tab fields · Enter next · F5 copy · Ctrl+S save · Esc back"
	} else if m.exportFormat == exportSyscGreet {
		help += " · F5 copy"
	}
	title := "sysclet  //  Export · " + string(m.exportFormat)
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Width(width).Bold(true).Foreground(lipgloss.Color("45")).Render(title),
		box.Render(content),
		lipgloss.NewStyle().Width(width).Foreground(lipgloss.Color("244")).Render(help),
	)
}

func (m *editorModel) compactExportView(width, height int) string {
	if width < 1 {
		width = 1
	}
	showRecoveryStatus := m.status != "" &&
		!strings.HasPrefix(m.status, "Copied ") &&
		!strings.HasPrefix(m.status, "Created profile ") &&
		!strings.HasPrefix(m.status, "Updated ") &&
		!strings.HasPrefix(m.status, "Exported ")
	pathLabel := "Output"
	if m.exportFormat == exportSyscGreet {
		pathLabel = "Profile"
	}
	pathInput := m.activeExportPathInput()
	pathInput.Width = maxInt(1, width-lipgloss.Width(pathLabel)-5)

	tabs := make([]string, 0, len(exportOptions))
	selected := ""
	for i, option := range exportOptions {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		if option.format == m.exportFormat {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("45")).Bold(true)
			selected = style.Render(string(option.format)) + fmt.Sprintf(" %d/%d · Tab cycles", i+1, len(exportOptions))
		}
		tabs = append(tabs, style.Render(string(option.format)))
	}
	strip := strings.Join(tabs, " ")
	if lipgloss.Width(strip) > width {
		// Keep the format row to one line so narrow terminals still show recovery guidance.
		strip = selected
	}
	parts := []string{lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("45")).Render("sysclet · Export · " + string(m.exportFormat))}
	if !showRecoveryStatus || height >= 11 {
		parts = append(parts, exportFieldLabel("OUTPUT FORMAT", m.exportFocus == exportFocusFormat))
	}
	parts = append(parts, lipgloss.NewStyle().Width(width).Render(strip))
	if m.exportFormat == exportSyscGreet {
		profileHint := "No compositor detected · Ctrl+O choose profile"
		if m.detectedCompositor != "" {
			profileHint = "Detected compositor: " + m.detectedCompositor
		} else if strings.TrimSpace(m.profilePathInput.Value()) != "" {
			profileHint = "Sysc-greet profile · Ctrl+O choose another"
		}
		if !showRecoveryStatus {
			parts = append(parts, lipgloss.NewStyle().Width(width).Foreground(lipgloss.Color("244")).Render(profileHint))
		}
		parts = append(parts, exportFieldLabel(pathLabel, m.exportFocus == exportFocusPath)+"  "+pathInput.View())
		slotLabel := "New slot · auto"
		if !m.slotAuto {
			slotLabel = "Replace slot"
		}
		m.slotInput.Width = maxInt(1, minInt(32, width-lipgloss.Width(slotLabel)-5))
		parts = append(parts, exportFieldLabel(slotLabel, m.exportFocus == exportFocusSlot)+"  "+m.slotInput.View())
	} else {
		parts = append(parts, exportFieldLabel(pathLabel, m.exportFocus == exportFocusPath)+"  "+pathInput.View())
	}

	if showRecoveryStatus {
		parts = append(parts, lipgloss.NewStyle().Width(width).Foreground(lipgloss.Color("220")).Render(m.status))
	} else if hint := exportDisplayHint(m.exportFormat, pathInput.Value()); hint != "" {
		parts = append(parts, lipgloss.NewStyle().Width(width).Foreground(lipgloss.Color("45")).Render(hint))
	}
	help := "Tab formats · Enter destination · Ctrl+O/B browse · Ctrl+S save · Esc back"
	if showRecoveryStatus {
		help = "Ctrl+S retry · Esc back"
	} else if m.exportFocus == exportFocusPath {
		help = "Tab fields · Enter next · Ctrl+O/B browse · Ctrl+S save · Esc back"
	} else if m.exportFocus == exportFocusSlot {
		help = "Tab fields · Enter next · F5 copy · Ctrl+S save · Esc back"
	} else if m.exportFormat == exportSyscGreet {
		help += " · F5 copy"
	}
	helpView := lipgloss.NewStyle().Width(width).Foreground(lipgloss.Color("244")).Render(help)
	if m.status != "" && !showRecoveryStatus {
		base := append(append([]string(nil), parts...), helpView)
		statusLines := height - lipgloss.Height(lipgloss.JoinVertical(lipgloss.Left, base...))
		if statusLines > 0 {
			status := lipgloss.NewStyle().Width(width).Foreground(lipgloss.Color("220")).Render(m.status)
			parts = append(parts, clipExportPreview(status, width, statusLines))
		}
	}
	parts = append(parts, helpView)
	view := lipgloss.JoinVertical(lipgloss.Left, parts...)
	if height > 0 && lipgloss.Height(view) > height {
		return clipExportPreview(view, width, height)
	}
	return view
}

func exportFieldLabel(label string, focused bool) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Bold(true)
	if focused {
		style = style.Foreground(lipgloss.Color("45"))
	}
	return style.Render(label)
}

func clipExportPreview(preview string, width, maxLines int) string {
	if preview == "" || width < 1 || maxLines < 1 {
		return ""
	}
	lines := strings.Split(strings.TrimSuffix(preview, "\n"), "\n")
	if len(lines) > maxLines {
		if maxLines == 1 {
			lines = lines[:1]
		} else {
			omitted := len(lines) - maxLines + 1
			lines = append(lines[:maxLines-1], fmt.Sprintf("… %d more lines", omitted))
		}
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], width, "…")
	}
	return strings.Join(lines, "\n")
}

func (m *editorModel) controlView(control int, label, value string) string {
	style := lipgloss.NewStyle().Width(10).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238"))
	if m.selectedControl == control {
		style = style.Border(lipgloss.DoubleBorder()).Bold(true).Foreground(lipgloss.Color("16")).Background(lipgloss.Color("45")).BorderForeground(lipgloss.Color("45"))
	} else {
		style = style.Foreground(lipgloss.Color("252"))
	}
	if utf8.RuneCountInString(value) > 10 {
		value = string([]rune(value)[:9]) + "…"
	}
	return style.Render(label + "\n" + value)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
