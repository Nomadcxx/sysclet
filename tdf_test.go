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
