package main

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestParseTDFAndRenderPreservesGlyphColors(t *testing.T) {
	fonts, err := ParseTDF(testTDF())
	if err != nil {
		t.Fatal(err)
	}
	if len(fonts) != 1 || fonts[0].Name != "Tiny" {
		t.Fatalf("fonts = %#v, want one Tiny font", fonts)
	}

	got, err := fonts[0].Render("A")
	if err != nil {
		t.Fatal(err)
	}
	want := "\x1b[0;31;44m─\x1b[0m\n\x1b[0;94m│\x1b[0m"
	if got != want {
		t.Fatalf("Render(\"A\") = %q, want %q", got, want)
	}
}

func TestRenderStacksMultipleTextRows(t *testing.T) {
	fonts, err := ParseTDF(testTDF())
	if err != nil {
		t.Fatal(err)
	}
	row, err := fonts[0].Render("A")
	if err != nil {
		t.Fatal(err)
	}

	for _, input := range []struct {
		text string
		want string
	}{
		{text: "A\nA", want: row + "\n" + row},
		{text: "A\n\nA", want: row + "\n\n" + row},
		{text: "A\n", want: row + "\n"},
	} {
		got, err := fonts[0].Render(input.text)
		if err != nil {
			t.Fatalf("Render(%q) error = %v", input.text, err)
		}
		if got != input.want {
			t.Errorf("Render(%q) = %q, want %q", input.text, got, input.want)
		}
	}

	if _, err := fonts[0].Render("\n  \n"); err == nil || !strings.Contains(err.Error(), "at least one non-space") {
		t.Fatalf("Render(blank rows) error = %v, want missing glyph error", err)
	}
	if _, err := fonts[0].Render("A\rA"); err == nil || !strings.Contains(err.Error(), "unsupported character") {
		t.Fatalf("Render(CR) error = %v, want unsupported character error", err)
	}
}

func TestParseTDFAcceptsGlyphBoundEndingAfterZeroColorAttribute(t *testing.T) {
	data := testTDF()
	const glyphOffset = tdfHeaderSize + tdfRecordSize
	// The final cell's color is 00, which is valid data. The bounded glyph
	// record ends there without a separate NUL character.
	data = data[:glyphOffset+7]
	binary.LittleEndian.PutUint16(data[tdfHeaderSize+23:tdfHeaderSize+25], 7)
	data[glyphOffset+6] = 0

	fonts, err := ParseTDF(data)
	if err != nil {
		t.Fatal(err)
	}
	rows := fonts[0].Glyphs[int('A'-33)]
	if len(rows) != 2 || len(rows[1]) != 1 || rows[1][0] != (Cell{Rune: '│', Color: 0}) {
		t.Fatalf("glyph rows = %#v, want final cell │ with color 0", rows)
	}
}

func TestParseTDFRejectsGlyphOffsetOutsideBlock(t *testing.T) {
	data := testTDF()
	const recordStart, glyphIndex = 20, int('A' - 33)
	blockSize := binary.LittleEndian.Uint16(data[recordStart+23 : recordStart+25])
	binary.LittleEndian.PutUint16(data[recordStart+25+glyphIndex*2:], blockSize)

	_, err := ParseTDF(data)
	if err == nil || !strings.Contains(err.Error(), "glyph") {
		t.Fatalf("ParseTDF error = %v, want malformed glyph error", err)
	}
}

func TestParseTDFAcceptsExtendedUpstreamFontName(t *testing.T) {
	data := testTDF()
	name := "FinalDestiny2"
	data[tdfHeaderSize+4] = byte(len(name))
	copy(data[tdfHeaderSize+5:], name)

	fonts, err := ParseTDF(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(fonts) != 1 || fonts[0].Name != name {
		t.Fatalf("parsed font name = %#v, want %q", fonts, name)
	}
}

func TestParseTDFRejectsNameBeyondHeaderSlot(t *testing.T) {
	data := testTDF()
	data[tdfHeaderSize+4] = 17

	if _, err := ParseTDF(data); err == nil || !strings.Contains(err.Error(), "font name length") {
		t.Fatalf("ParseTDF error = %v, want overlong font name error", err)
	}
}

func TestParseTDFRejectsControlCharactersInFontName(t *testing.T) {
	data := testTDF()
	data[tdfHeaderSize+5] = '\x1b'

	if _, err := ParseTDF(data); err == nil || !strings.Contains(err.Error(), "non-printable name") {
		t.Fatalf("ParseTDF error = %v, want invalid font name error", err)
	}
}

func TestParseTDFBoundsExpandedGlyphCells(t *testing.T) {
	const blockSize = 65535
	glyphData := make([]byte, blockSize)
	for i := 2; i < len(glyphData); i++ {
		glyphData[i] = 'A'
	}
	data := testTDFWithRepeatedFontRecords(32, glyphData, 0)

	if _, err := ParseTDF(data); err == nil || !strings.Contains(err.Error(), "expanded glyph data") {
		t.Fatalf("ParseTDF error = %v, want expanded glyph data limit error", err)
	}
}

func TestParseTDFBoundsExpandedGlyphRows(t *testing.T) {
	glyphData := make([]byte, 65535)
	for i := 2; i < len(glyphData); i++ {
		glyphData[i] = 13
	}
	data := testTDFWithRepeatedFontRecords(32, glyphData, 0)

	if _, err := ParseTDF(data); err == nil || !strings.Contains(err.Error(), "expanded glyph data") {
		t.Fatalf("ParseTDF error = %v, want expanded glyph data limit error", err)
	}
}

func TestParseTDFBoundsFontCount(t *testing.T) {
	data := testTDFWithRepeatedFontRecords(4097, nil, 0)

	if _, err := ParseTDF(data); err == nil || !strings.Contains(err.Error(), "font record limit") {
		t.Fatalf("ParseTDF error = %v, want font record limit error", err)
	}
}

func TestParseTDFReusesGlyphDataForDuplicateOffsets(t *testing.T) {
	const blockSize = 12_000
	glyphData := make([]byte, blockSize)
	for i := 2; i < len(glyphData); i++ {
		glyphData[i] = 'A'
	}
	data := testTDFWithRepeatedFontRecords(1, glyphData, 0)
	for i := 0; i < tdfGlyphCount; i++ {
		binary.LittleEndian.PutUint16(data[tdfHeaderSize+25+i*2:], 0)
	}

	fonts, err := ParseTDF(data)
	if err != nil {
		t.Fatal(err)
	}
	first := &fonts[0].Glyphs[0][0][0]
	for i := 1; i < tdfGlyphCount; i++ {
		if got := &fonts[0].Glyphs[i][0][0]; got != first {
			t.Fatalf("glyph %d did not reuse the shared data", i)
		}
	}
}

func TestRenderRejectsTextWithoutFontGlyph(t *testing.T) {
	fonts, err := ParseTDF(testTDF())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fonts[0].Render("Z"); err == nil {
		t.Fatal("Render accepted a missing glyph")
	}
}

func testTDF() []byte {
	data := append([]byte{0x13}, []byte("TheDraw FONTS file")...)
	data = append(data, 0x1a)

	const recordSize = 213
	glyph := []byte{0, 0, 196, 0x14, 13, 179, 0x09, 0}
	record := make([]byte, recordSize+len(glyph))
	copy(record[:4], []byte{0x55, 0xaa, 0, 0xff})
	record[4] = 4
	copy(record[5:9], "Tiny")
	record[21] = 2
	record[22] = 1
	binary.LittleEndian.PutUint16(record[23:25], uint16(len(glyph)))
	for i := 0; i < 94; i++ {
		binary.LittleEndian.PutUint16(record[25+i*2:], 0xffff)
	}
	binary.LittleEndian.PutUint16(record[25+int('A'-33)*2:], 0)
	copy(record[recordSize:], glyph)
	return append(data, record...)
}

func testTDFWithRepeatedFontRecords(count int, glyphData []byte, fontType byte) []byte {
	data := append([]byte(nil), testTDF()[:tdfHeaderSize]...)
	for recordIndex := 0; recordIndex < count; recordIndex++ {
		record := make([]byte, tdfRecordSize+len(glyphData))
		copy(record[:4], tdfRecordTag)
		record[4] = 4
		copy(record[5:9], "Tiny")
		record[21] = fontType
		binary.LittleEndian.PutUint16(record[23:25], uint16(len(glyphData)))
		for i := 0; i < tdfGlyphCount; i++ {
			binary.LittleEndian.PutUint16(record[25+i*2:], 0xffff)
		}
		if len(glyphData) > 0 {
			binary.LittleEndian.PutUint16(record[25+int('A'-33)*2:], 0)
		}
		copy(record[tdfRecordSize:], glyphData)
		data = append(data, record...)
	}
	return data
}
