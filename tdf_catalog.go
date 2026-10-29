package main

import (
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

//go:embed assets/fonts/catalog.json assets/fonts/*.tdf.gz
var bundledFontFiles embed.FS

type fontCatalogEntry struct {
	Name   string `json:"name"`
	Pack   string `json:"pack"`
	Record int    `json:"record"`
}

func loadBundledFontCatalog() ([]fontCatalogEntry, error) {
	data, err := bundledFontFiles.ReadFile("assets/fonts/catalog.json")
	if err != nil {
		return nil, fmt.Errorf("read bundled font catalog: %w", err)
	}
	var entries []fontCatalogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse bundled font catalog: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("bundled font catalog is empty")
	}
	for _, entry := range entries {
		if entry.Name == "" || entry.Record < 0 || filepath.Base(entry.Pack) != entry.Pack || filepath.Ext(entry.Pack) != ".tdf" {
			return nil, fmt.Errorf("invalid bundled font catalog entry %q", entry.Name)
		}
	}
	return entries, nil
}

func loadBundledFontPack(name string) ([]Font, error) {
	if filepath.Base(name) != name || filepath.Ext(name) != ".tdf" {
		return nil, fmt.Errorf("invalid bundled font pack %q", name)
	}
	compressed, err := bundledFontFiles.ReadFile("assets/fonts/" + name + ".gz")
	if err != nil {
		return nil, fmt.Errorf("read bundled font pack %q: %w", name, err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("open bundled font pack %q: %w", name, err)
	}
	data, readErr := io.ReadAll(io.LimitReader(reader, maxTDFBytes+1))
	closeErr := reader.Close()
	if readErr != nil {
		return nil, fmt.Errorf("decompress bundled font pack %q: %w", name, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close bundled font pack %q: %w", name, closeErr)
	}
	if len(data) > maxTDFBytes {
		return nil, fmt.Errorf("bundled font pack %q exceeds the %d MiB size limit", name, maxTDFBytes>>20)
	}
	fonts, err := ParseTDF(data)
	if err != nil {
		return nil, fmt.Errorf("parse bundled font pack %q: %w", name, err)
	}
	return fonts, nil
}

func loadBundledFontByName(name string) (Font, error) {
	entries, err := loadBundledFontCatalog()
	if err != nil {
		return Font{}, err
	}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name, name) {
			continue
		}
		return loadBundledFontEntry(entry)
	}
	return Font{}, fmt.Errorf("bundled font %q not found", name)
}

func loadBundledFontEntry(entry fontCatalogEntry) (Font, error) {
	fonts, err := loadBundledFontPack(entry.Pack)
	if err != nil {
		return Font{}, err
	}
	return bundledFontFromPack(fonts, entry)
}

func bundledFontFromPack(fonts []Font, entry fontCatalogEntry) (Font, error) {
	if entry.Record >= len(fonts) || fonts[entry.Record].Name != entry.Name {
		return Font{}, fmt.Errorf("bundled font catalog entry for %q does not match pack %q", entry.Name, entry.Pack)
	}
	return fonts[entry.Record], nil
}
