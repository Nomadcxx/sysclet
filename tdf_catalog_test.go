package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestBundledFontCatalogMatchesEveryEmbeddedRecord(t *testing.T) {
	catalog, err := loadBundledFontCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 3481 {
		t.Fatalf("catalog contains %d fonts, want 3481", len(catalog))
	}

	byPack := make(map[string][]fontCatalogEntry)
	seenNames := make(map[string]bool, len(catalog))
	for _, entry := range catalog {
		if seenNames[entry.Name] {
			t.Errorf("duplicate font name %q", entry.Name)
		}
		seenNames[entry.Name] = true
		byPack[entry.Pack] = append(byPack[entry.Pack], entry)
	}
	if len(byPack) != 36 {
		t.Fatalf("catalog references %d packs, want 36", len(byPack))
	}
	for pack, entries := range byPack {
		fonts, err := loadBundledFontPack(pack)
		if err != nil {
			t.Fatalf("loadBundledFontPack(%q): %v", pack, err)
		}
		if len(fonts) != len(entries) {
			t.Fatalf("pack %q has %d parsed fonts, catalog has %d", pack, len(fonts), len(entries))
		}
		for _, entry := range entries {
			if entry.Record < 0 || entry.Record >= len(fonts) {
				t.Errorf("pack %q record index %d is outside %d parsed fonts", pack, entry.Record, len(fonts))
				continue
			}
			if fonts[entry.Record].Name != entry.Name {
				t.Errorf("pack %q record %d is %q, catalog says %q", pack, entry.Record, fonts[entry.Record].Name, entry.Name)
			}
		}
	}
}

func TestBundledBleachFontRendersSYSC(t *testing.T) {
	font, err := loadBundledFontByName("Bleach")
	if err != nil {
		t.Fatal(err)
	}
	art, err := font.Render("SYSC")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(art, "\x1b[") || !strings.Contains(art, "▄") {
		t.Fatalf("Bleach render = %q, want colored block art", art)
	}
}

func TestRenderCommandUsesBundledFontName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"render", "--font-name", "Bleach", "SYSC"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("render output = %q, want ANSI color sequences", stdout.String())
	}
}
