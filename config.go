package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func ProfileSnippet(slot, art string) (string, error) {
	if err := validateSlot(slot); err != nil {
		return "", err
	}
	if art == "" {
		return "", fmt.Errorf("art must not be empty")
	}
	lines := slotLines(slot, art)
	return strings.Join(append([]string{"ansi_style=true"}, lines...), "\n") + "\n", nil
}

func UpdateProfile(path, slot, art string) (string, error) {
	if err := validateSlot(slot); err != nil {
		return "", err
	}
	if art == "" {
		return "", fmt.Errorf("art must not be empty")
	}

	linkInfo, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("inspect profile %q: %w", path, err)
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("refusing to replace symlink %q; pass its target path", path)
	}
	if !linkInfo.Mode().IsRegular() {
		return "", fmt.Errorf("profile %q is not a regular file", path)
	}

	original, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read profile %q: %w", path, err)
	}
	updated, err := replaceProfile(string(original), slot, art)
	if err != nil {
		return "", err
	}
	backupPath, err := createBackup(path, original, linkInfo.Mode().Perm())
	if err != nil {
		return "", fmt.Errorf("back up profile %q: %w", path, err)
	}
	if err := replaceFile(path, original, []byte(updated), linkInfo.Mode().Perm()); err != nil {
		return backupPath, fmt.Errorf("write profile %q (original saved at %s): %w", path, backupPath, err)
	}
	return backupPath, nil
}

func validateSlot(slot string) error {
	if !strings.HasPrefix(slot, "ascii_") {
		return fmt.Errorf("invalid slot %q; expected ascii_N", slot)
	}
	digits := strings.TrimPrefix(slot, "ascii_")
	if digits == "" {
		return fmt.Errorf("invalid slot %q; expected ascii_N with N greater than zero", slot)
	}
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return fmt.Errorf("invalid slot %q; expected ascii_N with N greater than zero", slot)
		}
	}
	number, err := strconv.Atoi(digits)
	if err != nil || number < 1 {
		return fmt.Errorf("invalid slot %q; expected ascii_N with N greater than zero", slot)
	}
	return nil
}

func slotLines(slot, art string) []string {
	escaped := strings.ReplaceAll(art, "\x1b", `\033`)
	escaped = strings.TrimSuffix(escaped, "\n")
	lines := strings.Split(escaped, "\n")
	result := make([]string, 0, len(lines)+2)
	result = append(result, slot+`="""`)
	result = append(result, lines...)
	return append(result, `"""`)
}

func replaceProfile(profile, slot, art string) (string, error) {
	lineEnding := "\n"
	if strings.Contains(profile, "\r\n") {
		lineEnding = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(profile, "\r\n", "\n"), "\n")

	start, end := -1, -1
	for i := 0; i < len(lines); {
		key, value, ok := configAssignment(lines[i])
		if !ok {
			i++
			continue
		}
		if key == slot {
			start = i
			var blockErr error
			end, blockErr = profileEntryEnd(lines, i, value)
			if blockErr != nil {
				return "", fmt.Errorf("profile slot %q: %w", slot, blockErr)
			}
			break
		}
		next, scanErr := skipProfileEntry(lines, i, key, value)
		if scanErr != nil {
			return "", scanErr
		}
		i = next
	}

	block := slotLines(slot, art)
	if start >= 0 {
		lines = append(lines[:start], append(block, lines[end:]...)...)
	} else {
		insert := len(lines)
		if insert > 0 && lines[insert-1] == "" {
			insert--
		}
		lines = append(lines[:insert], append(block, lines[insert:]...)...)
	}

	styleFound := false
	firstSlot := -1
	for i := 0; i < len(lines); {
		key, value, ok := configAssignment(lines[i])
		if !ok {
			i++
			continue
		}
		if key == "ansi_style" {
			indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))]
			lines[i] = indent + "ansi_style=true"
			styleFound = true
			i++
			continue
		}
		if isASCIISlot(key) && firstSlot < 0 {
			firstSlot = i
		}
		next, scanErr := skipProfileEntry(lines, i, key, value)
		if scanErr != nil {
			return "", scanErr
		}
		i = next
	}
	if !styleFound {
		insert := firstSlot
		if insert < 0 {
			insert = len(lines)
			if insert > 0 && lines[insert-1] == "" {
				insert--
			}
		}
		lines = append(lines[:insert], append([]string{"ansi_style=true"}, lines[insert:]...)...)
	}

	return replaceLineEnding(strings.Join(lines, "\n"), lineEnding), nil
}

func replaceLineEnding(value, ending string) string {
	if ending == "\n" {
		return value
	}
	return strings.ReplaceAll(value, "\n", ending)
}

func configAssignment(line string) (key, value string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	separator := strings.IndexByte(trimmed, '=')
	if separator < 1 {
		return "", "", false
	}
	key = strings.TrimSpace(trimmed[:separator])
	for i, char := range key {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || char == '_' || (i > 0 && char >= '0' && char <= '9')) {
			return "", "", false
		}
	}
	return key, strings.TrimSpace(trimmed[separator+1:]), true
}

func isASCIISlot(key string) bool {
	return validateSlot(key) == nil
}

func skipProfileEntry(lines []string, start int, key, value string) (int, error) {
	if !isASCIISlot(key) && key != "ascii" {
		return start + 1, nil
	}
	end, err := profileEntryEnd(lines, start, value)
	return end, err
}

func profileEntryEnd(lines []string, start int, value string) (int, error) {
	if strings.Contains(value, `"""`) {
		if strings.Count(value, `"""`) > 1 {
			return start + 1, nil
		}
		for i := start + 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == `"""` {
				return i + 1, nil
			}
		}
		return -1, errors.New("unterminated triple-quoted ASCII block")
	}
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == `"""` {
			return i + 1, nil
		}
		if _, _, ok := configAssignment(lines[i]); ok {
			return i, nil
		}
	}
	return len(lines), nil
}

func createBackup(path string, contents []byte, mode os.FileMode) (string, error) {
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	for attempt := 0; attempt < 10; attempt++ {
		suffix := stamp
		if attempt > 0 {
			suffix += "-" + strconv.Itoa(attempt)
		}
		backupPath := path + ".sysclet-" + suffix + ".bak"
		file, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, writeErr := file.Write(contents)
		if writeErr == nil {
			writeErr = file.Sync()
		}
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			_ = os.Remove(backupPath)
			return "", errors.Join(writeErr, closeErr)
		}
		return backupPath, nil
	}
	return "", fmt.Errorf("could not choose a unique backup filename")
}

func replaceFile(path string, original, updated []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".sysclet-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(updated); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}

	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, original) {
		return fmt.Errorf("profile changed during update")
	}
	return os.Rename(temporaryPath, path)
}
