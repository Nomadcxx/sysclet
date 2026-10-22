package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ponytail: cap local fonts at 64 MiB, 4,096 records, and 1 Mi total decoded cells/rows;
// raise these limits only if a real TDF pack needs more.
const (
	maxTDFBytes              = 64 << 20
	maxTDFFonts              = 4096
	maxTDFExpandedGlyphUnits = 1 << 20
	tdfHeaderSize            = 20
	tdfRecordSize            = 213
	tdfGlyphCount            = 94
	tdfColorType             = 2
)

var (
	tdfSignature = []byte("TheDraw FONTS file")
	tdfRecordTag = []byte{0x55, 0xaa, 0x00, 0xff}
)

type Cell struct {
	Rune  rune
	Color byte
}

type Font struct {
	Name          string
	Type          byte
	LetterSpacing byte
	Glyphs        [tdfGlyphCount][][]Cell
}

func ParseTDF(data []byte) ([]Font, error) {
	if len(data) > maxTDFBytes {
		return nil, fmt.Errorf("TDF file exceeds the %d MiB size limit", maxTDFBytes>>20)
	}
	if len(data) < tdfHeaderSize {
		return nil, fmt.Errorf("TDF file is shorter than its header")
	}
	if data[0] != 0x13 || !bytes.Equal(data[1:19], tdfSignature) || data[19] != 0x1a {
		return nil, fmt.Errorf("invalid TheDraw TDF header")
	}

	var fonts []Font
	expandedGlyphUnits := 0
	for offset := tdfHeaderSize; offset < len(data); {
		if len(fonts) >= maxTDFFonts {
			return nil, fmt.Errorf("TDF file exceeds the %d font record limit", maxTDFFonts)
		}
		if len(data)-offset < tdfRecordSize || !bytes.Equal(data[offset:offset+4], tdfRecordTag) {
			return nil, fmt.Errorf("invalid TDF font record at byte %d", offset)
		}

		nameLen := int(data[offset+4])
		// The format guide says 12; TAAG's FinalDestiny2 record uses 13
		// bytes in the 16-byte name slot before the type field.
		if nameLen > 16 {
			return nil, fmt.Errorf("invalid font name length %d at byte %d", nameLen, offset+4)
		}
		name := strings.TrimRight(string(data[offset+5:offset+5+nameLen]), "\x00")
		if name == "" || !utf8.ValidString(name) {
			return nil, fmt.Errorf("font record at byte %d has an invalid name", offset)
		}
		for _, char := range name {
			if !unicode.IsPrint(char) {
				return nil, fmt.Errorf("font record at byte %d has a non-printable name", offset)
			}
		}

		blockSize := int(binary.LittleEndian.Uint16(data[offset+23 : offset+25]))
		glyphStart := offset + tdfRecordSize
		glyphEnd := glyphStart + blockSize
		if glyphEnd > len(data) {
			return nil, fmt.Errorf("font %q declares glyph data beyond the end of the TDF file", name)
		}

		font := Font{
			Name:          name,
			Type:          data[offset+21],
			LetterSpacing: data[offset+22],
		}
		glyphOffsets := make([]uint16, tdfGlyphCount)
		var parsedGlyphs map[uint16][][]Cell
		for i := range glyphOffsets {
			glyphOffsets[i] = binary.LittleEndian.Uint16(data[offset+25+i*2 : offset+27+i*2])
		}

		for i, glyphOffset := range glyphOffsets {
			if glyphOffset == 0xffff {
				continue
			}
			if rows, ok := parsedGlyphs[glyphOffset]; ok {
				font.Glyphs[i] = rows
				continue
			}
			if int(glyphOffset) >= blockSize {
				return nil, fmt.Errorf("font %q glyph %q has an offset outside its data block", name, rune(i+33))
			}

			glyphLimit := blockSize
			for _, next := range glyphOffsets {
				if next != 0xffff && next > glyphOffset && int(next) < glyphLimit {
					glyphLimit = int(next)
				}
			}
			glyphData := data[glyphStart+int(glyphOffset) : glyphStart+glyphLimit]
			rows, err := parseGlyph(glyphData, font.Type)
			if err != nil {
				return nil, fmt.Errorf("font %q glyph %q: %w", name, rune(i+33), err)
			}
			units := len(rows)
			for _, row := range rows {
				units += len(row)
			}
			if units > maxTDFExpandedGlyphUnits-expandedGlyphUnits {
				return nil, fmt.Errorf("TDF file exceeds the expanded glyph data limit")
			}
			expandedGlyphUnits += units
			if parsedGlyphs == nil {
				parsedGlyphs = make(map[uint16][][]Cell)
			}
			parsedGlyphs[glyphOffset] = rows
			font.Glyphs[i] = rows
		}

		fonts = append(fonts, font)
		offset = glyphEnd
	}
	if len(fonts) == 0 {
		return nil, fmt.Errorf("TDF file contains no fonts")
	}
	return fonts, nil
}

func parseGlyph(data []byte, fontType byte) ([][]Cell, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("glyph data is too short")
	}

	position := 2 // TheDraw stores two metadata bytes before each glyph's row data.
	var rows [][]Cell
	var row []Cell
	for position < len(data) {
		char := data[position]
		if char == 0 {
			if len(row) > 0 {
				rows = append(rows, row)
			}
			return rows, nil
		}
		if char == 13 {
			rows = append(rows, row)
			row = nil
			position++
			continue
		}

		color := byte(7)
		if fontType == tdfColorType {
			if position+1 >= len(data) {
				return nil, fmt.Errorf("colored glyph ends before its color attribute")
			}
			color = data[position+1]
			position += 2
		} else {
			position++
		}

		if fontType == 0 {
			if mapped, ok := outlineCharacters[char]; ok {
				char = mapped
			}
		}
		row = append(row, Cell{Rune: cp437Rune(char), Color: color})
	}
	// Glyph offsets bound the data, and some bundled records end at that
	// boundary without a separate NUL character after the final color byte.
	if len(row) > 0 {
		rows = append(rows, row)
	}
	return rows, nil
}

var outlineCharacters = map[byte]byte{
	'A': 205, 'B': 196, 'C': 179, 'D': 186, 'E': 213,
	'F': 187, 'G': 214, 'H': 191, 'I': 200, 'J': 190,
	'K': 192, 'L': 189, 'M': 181, 'N': 199, 'O': 32,
	'@': 32, '&': 32,
}

func (font Font) Render(text string) (string, error) {
	return font.RenderWithColors(text, -1, -1)
}

// RenderWithColors applies optional ANSI palette overrides while retaining TDF colors by default.
func (font Font) RenderWithColors(text string, foregroundOverride, backgroundOverride int) (string, error) {
	if text == "" {
		return "", fmt.Errorf("text must not be empty")
	}
	if foregroundOverride < -1 || foregroundOverride > 15 {
		return "", fmt.Errorf("foreground color must be -1 (TDF) or between 0 and 15")
	}
	if backgroundOverride < -1 || backgroundOverride > 15 {
		return "", fmt.Errorf("background color must be -1 (TDF) or between 0 and 15")
	}

	type token struct {
		space bool
		rows  [][]Cell
	}
	tokens := make([]token, 0, utf8.RuneCountInString(text))
	height := 0
	hasGlyph := false
	for _, char := range text {
		if char == ' ' {
			tokens = append(tokens, token{space: true})
			continue
		}
		if char < 33 || char > 126 {
			return "", fmt.Errorf("text contains unsupported character %q; use printable ASCII", char)
		}
		rows := font.Glyphs[int(char)-33]
		if len(rows) == 0 {
			return "", fmt.Errorf("font %q has no glyph for %q", font.Name, char)
		}
		if len(rows) > height {
			height = len(rows)
		}
		hasGlyph = true
		tokens = append(tokens, token{rows: rows})
	}
	if !hasGlyph {
		return "", fmt.Errorf("text must contain at least one non-space character")
	}

	spacing := int(font.LetterSpacing) - 1
	if spacing < 0 {
		spacing = 0
	}
	var output strings.Builder
	for line := 0; line < height; line++ {
		var cells []Cell
		for i, item := range tokens {
			if i > 0 {
				cells = appendDefaultSpaces(cells, spacing)
			}
			if item.space {
				cells = appendDefaultSpaces(cells, 4)
				continue
			}

			width := 0
			for _, row := range item.rows {
				if len(row) > width {
					width = len(row)
				}
			}
			rowWidth := 0
			if line < len(item.rows) {
				rowWidth = len(item.rows[line])
				cells = append(cells, item.rows[line]...)
			}
			cells = appendDefaultSpaces(cells, width-rowWidth)
		}
		writeANSIline(&output, cells, foregroundOverride, backgroundOverride)
		if line+1 < height {
			output.WriteByte('\n')
		}
	}
	return output.String(), nil
}

func appendDefaultSpaces(cells []Cell, count int) []Cell {
	for i := 0; i < count; i++ {
		cells = append(cells, Cell{Rune: ' ', Color: 7})
	}
	return cells
}

func writeANSIline(output *strings.Builder, cells []Cell, foregroundOverride, backgroundOverride int) {
	previousForeground, previousBackground := 0, 0
	haveColor := false
	for _, cell := range cells {
		foreground, background := ansiColors(cell.Color, foregroundOverride, backgroundOverride)
		if !haveColor || foreground != previousForeground || background != previousBackground {
			writeColorSGR(output, foreground, background)
			previousForeground, previousBackground = foreground, background
			haveColor = true
		}
		output.WriteRune(cell.Rune)
	}
	output.WriteString("\x1b[0m")
}

func ansiColors(attribute byte, foregroundOverride, backgroundOverride int) (int, int) {
	ansiOrder := [...]int{0, 4, 2, 6, 1, 5, 3, 7}
	foreground := int(attribute & 0x0f)
	foregroundCode := ansiOrder[foreground&0x07]
	if foreground >= 8 {
		foregroundCode += 8
	}
	if foregroundOverride >= 0 {
		foregroundCode = foregroundOverride
	}

	background := -1
	if value := int(attribute>>4) & 0x07; value != 0 {
		background = ansiOrder[value]
	}
	if backgroundOverride >= 0 {
		background = backgroundOverride
	}
	return foregroundCode, background
}

func writeColorSGR(output *strings.Builder, foreground, background int) {
	foregroundCode := 30 + (foreground & 7)
	if foreground >= 8 {
		foregroundCode = 90 + foreground - 8
	}
	if background < 0 {
		fmt.Fprintf(output, "\x1b[0;%dm", foregroundCode)
		return
	}
	backgroundCode := 40 + background
	if background >= 8 {
		backgroundCode = 100 + background - 8
	}
	fmt.Fprintf(output, "\x1b[0;%d;%dm", foregroundCode, backgroundCode)
}
