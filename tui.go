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
	exportFocusPath = iota
	exportFocusSlot
	exportFocusFormat
)

const alignedWidth = 80

var ansiPaletteNames = [...]string{
	"Black", "Red", "Green", "Yellow", "Blue", "Magenta", "Cyan", "White",
	"Bright Black", "Bright Red", "Bright Green", "Bright Yellow",
	"Bright Blue", "Bright Magenta", "Bright Cyan", "Bright White",
}

type editorModel struct {
	textInput             textinput.Model
	fontFilter            textinput.Model
	profilePathInput      textinput.Model
	slotInput             textinput.Model
	filePicker            filepicker.Model
	mode                  editorMode
	text                  string
	previewRaw            string
	preview               string
	font                  Font
	fontCatalog           []fontCatalogEntry
	fonts                 []Font
	fontChoices           []int
	fontPath              string
	fontIndex             int
	fontChoiceFromCatalog bool
	fontPickerReturnMode  editorMode
	selectedControl       int
	alignment             int
	foreground            int
	background            int
	exportFocus           int
	exportFormat          exportFormat
	writeClipboard        func(string) error
	width                 int
	height                int
	status                string
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
		slotInput:        slotInput,
		selectedControl:  controlText,
		alignment:        alignmentCenter,
		foreground:       -1,
		background:       -1,
		exportFormat:     exportSyscGreet,
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
			m.filePicker.Height = minInt(24, maxInt(6, msg.Height-12))
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
			if m.exportFocus == 0 {
				m.profilePathInput, command = m.profilePathInput.Update(message)
			} else {
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
	m.exportFocus = exportFocusPath
	m.slotInput.Blur()
	m.status = "Tab switches fields · ↑/↓ choose format · Ctrl+O/Ctrl+B browse · Ctrl+S export · Esc cancel"
	return m.profilePathInput.Focus()
}

func (m *editorModel) openExportPicker() (tea.Model, tea.Cmd) {
	start := m.profilePathInput.Value()
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
		picker.Height = minInt(24, maxInt(6, height-12))
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

func existingProfileDir() string {
	candidates := []string{"/usr/share/sysc-greet/ascii_configs"}
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" && filepath.IsAbs(configHome) {
		candidates = append(candidates, filepath.Join(configHome, "sysc-greet", "ascii_configs"))
	} else if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "sysc-greet", "ascii_configs"))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
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
			m.fontFilter.SetValue("")
			m.fontChoices = make([]int, len(fonts))
			for i := range fonts {
				m.fontChoices[i] = i
			}
			m.mode = modeFontChoice
			m.status = "This file contains multiple fonts"
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
	case "ctrl+o":
		return m.openLocalFontPicker()
	case "up", "k":
		if m.fontIndex > 0 {
			m.fontIndex--
		}
	case "down", "j":
		if m.fontIndex+1 < len(m.fontChoices) {
			m.fontIndex++
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
}

func (m *editorModel) selectFont(index int) {
	var font Font
	if m.fontChoiceFromCatalog {
		if index < 0 || index >= len(m.fontCatalog) {
			return
		}
		var err error
		font, err = loadBundledFontEntry(m.fontCatalog[index])
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
	m.fontFilter.Blur()
	m.mode = modeNormal
	_ = m.setText(m.text)
	if m.status == "" {
		m.status = fmt.Sprintf("Loaded %s · %s", filepath.Base(m.fontPath), m.font.Name)
	}
}

func (m *editorModel) updateExport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.profilePathInput.Blur()
		m.slotInput.Blur()
		m.status = ""
		return m, nil
	case "tab":
		return m, m.setExportFocus(m.nextExportFocus(1))
	case "shift+tab":
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
		m.profilePathInput, command = m.profilePathInput.Update(msg)
	} else if m.exportFocus == exportFocusSlot {
		m.slotInput, command = m.slotInput.Update(msg)
	}
	return m, command
}

func (m *editorModel) setExportFocus(index int) tea.Cmd {
	if index == exportFocusSlot && m.exportFormat != exportSyscGreet {
		index = exportFocusFormat
	}
	m.exportFocus = index
	m.profilePathInput.Blur()
	m.slotInput.Blur()
	switch index {
	case exportFocusPath:
		return m.profilePathInput.Focus()
	case exportFocusSlot:
		return m.slotInput.Focus()
	default:
		return nil
	}
}

func (m *editorModel) nextExportFocus(step int) int {
	if m.exportFormat != exportSyscGreet {
		if m.exportFocus == exportFocusFormat {
			return exportFocusPath
		}
		return exportFocusFormat
	}
	switch m.exportFocus {
	case exportFocusPath:
		if step < 0 {
			return exportFocusFormat
		}
		return exportFocusSlot
	case exportFocusSlot:
		if step < 0 {
			return exportFocusPath
		}
		return exportFocusFormat
	default:
		if step < 0 {
			return exportFocusSlot
		}
		return exportFocusPath
	}
}

func (m *editorModel) cycleExportFormat(step int) {
	m.exportFormat = cycleExportFormat(m.exportFormat, step)
	if m.exportFormat != exportSyscGreet && m.exportFocus == exportFocusSlot {
		m.setExportFocus(exportFocusFormat)
	}
}

func (m *editorModel) updateProfilePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.mode = modeExport
		return m, m.setExportFocus(exportFocusPath)
	}
	var command tea.Cmd
	m.filePicker, command = m.filePicker.Update(msg)
	if selected, path := m.filePicker.DidSelectFile(msg); selected {
		m.profilePathInput.SetValue(path)
		m.mode = modeExport
		return m, tea.Batch(command, m.setExportFocus(exportFocusPath))
	}
	return m, command
}

func (m *editorModel) saveExport() {
	if m.preview == "" {
		m.status = "Choose a TDF font and enter text before exporting"
		return
	}
	path := strings.TrimSpace(m.profilePathInput.Value())
	if path == "" {
		if m.exportFormat == exportSyscGreet {
			m.status = "Enter a sysc-greet profile path or press Ctrl+O/Ctrl+B to browse"
		} else {
			m.status = "Enter an output path"
		}
		return
	}
	if m.exportFormat == exportSyscGreet {
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
		return m.modalView("Select a sysc-greet profile", m.filePicker.View(), "↑/↓ navigate · Enter select · Esc back")
	}
	if m.mode == modeExport {
		return m.exportView(width)
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
		fmt.Fprintf(&choices, "%s%s\n", marker, name)
	}
	if end < len(m.fontChoices) {
		choices.WriteString("  ↓ more fonts\n")
	}
	if len(m.fontChoices) == 0 {
		choices.WriteString("No font names match this filter.")
	}
	content := lipgloss.JoinVertical(lipgloss.Left, "Filter: "+m.fontFilter.View(), "", choices.String())
	title := "Choose a font in " + filepath.Base(m.fontPath)
	help := "Type to filter · ↑/↓ navigate · Enter load · Esc back"
	if m.fontChoiceFromCatalog {
		title = "Choose a bundled TDF font"
		help = "Type to filter · ↑/↓ navigate · Enter load · Ctrl+O browse local .tdf · Esc back"
	}
	return m.modalView(title, content, help)
}

func (m *editorModel) exportView(width int) string {
	formatLabel := "Format"
	if m.exportFocus == exportFocusFormat {
		formatLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Bold(true).Render(formatLabel)
	}
	formatNames := make([]string, 0, len(exportOptions))
	for _, option := range exportOptions {
		if option.format == m.exportFormat {
			formatNames = append(formatNames, lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Bold(true).Render(string(option.format)))
		} else {
			formatNames = append(formatNames, lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(string(option.format)))
		}
	}
	pathLabel := "Output path"
	if m.exportFormat == exportSyscGreet {
		pathLabel = "Profile path"
	}
	if m.exportFocus == exportFocusPath {
		pathLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Bold(true).Render(pathLabel)
	}
	contentParts := []string{
		formatLabel + "  " + strings.Join(formatNames, "  "),
		pathLabel + "  " + m.profilePathInput.View(),
	}
	if m.exportFormat == exportSyscGreet {
		slotLabel := "ASCII slot"
		if m.exportFocus == exportFocusSlot {
			slotLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Bold(true).Render(slotLabel)
		}
		contentParts = append(contentParts, slotLabel+"  "+m.slotInput.View())
	}
	previewLabel := "File preview"
	preview := ""
	if m.exportFormat == exportSyscGreet {
		previewLabel = "Snippet preview · ANSI colors shown · profile stores \\033 escapes"
		preview = strings.ReplaceAll(m.exportSnippet(), `\033`, "\x1b")
	} else {
		var err error
		preview, err = generateExportContent(m.exportFormat, "", m.previewRaw)
		if err != nil {
			preview = err.Error()
		}
	}
	contentParts = append(contentParts,
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(previewLabel),
		lipgloss.NewStyle().Width(width-14).Foreground(lipgloss.Color("252")).Render(preview),
	)
	content := lipgloss.JoinVertical(lipgloss.Left, contentParts...)
	box := lipgloss.NewStyle().Width(width-8).Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("45"))
	help := "Tab/Enter next field · ↑/↓ choose format · Ctrl+O/Ctrl+B browse · Ctrl+S export · Esc cancel"
	if m.exportFormat == exportSyscGreet {
		help = "Tab/Enter next field · ↑/↓ choose format · F5 copy snippet · Ctrl+O/Ctrl+B browse · Ctrl+S export · Esc cancel"
	}
	if m.status != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, content, "", lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render(m.status))
	}
	title := "sysclet  //  Export to " + string(m.exportFormat)
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("45")).Render(title),
		box.Render(content),
		lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(help),
	)
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
