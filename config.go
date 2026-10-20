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
		if errors.Is(err, os.ErrNotExist) {
			snippet, snippetErr := ProfileSnippet(slot, art)
			if snippetErr != nil {
				return "", snippetErr
			}
			if createErr := createProfileFile(path, []byte(snippet)); createErr != nil {
				return "", fmt.Errorf("create profile %q: %w", path, createErr)
			}
			return "", nil
		}
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

func createProfileFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(contents)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return errors.Join(writeErr, closeErr)
	}
	return nil
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

type profileLine struct {
	text   string
	ending string
}

func splitProfileLines(profile string) []profileLine {
	lines := make([]profileLine, 0, strings.Count(profile, "\n")+1)
	for {
		newline := strings.IndexByte(profile, '\n')
		if newline < 0 {
			return append(lines, profileLine{text: profile})
		}
		text, ending := profile[:newline], "\n"
		if strings.HasSuffix(text, "\r") {
			text, ending = strings.TrimSuffix(text, "\r"), "\r\n"
		}
		lines = append(lines, profileLine{text: text, ending: ending})
		profile = profile[newline+1:]
	}
}

func profileTextLines(lines []profileLine) []string {
	text := make([]string, len(lines))
	for i := range lines {
		text[i] = lines[i].text
	}
	return text
}

func joinProfileLines(lines []profileLine) string {
	var result strings.Builder
	for _, line := range lines {
		result.WriteString(line.text)
		result.WriteString(line.ending)
	}
	return result.String()
}

func spliceProfileLines(lines []profileLine, start, end int, replacement []profileLine) []profileLine {
	result := make([]profileLine, 0, len(lines)-(end-start)+len(replacement))
	result = append(result, lines[:start]...)
	result = append(result, replacement...)
	return append(result, lines[end:]...)
}

func profileLineEnding(lines []profileLine, index int) string {
	if index > 0 && lines[index-1].ending != "" {
		return lines[index-1].ending
	}
	if index >= 0 && index < len(lines) && lines[index].ending != "" {
		return lines[index].ending
	}
	for _, line := range lines {
		if line.ending != "" {
			return line.ending
		}
	}
	return "\n"
}

func newProfileLines(text []string, ending string) []profileLine {
	lines := make([]profileLine, len(text))
	for i := range text {
		lines[i] = profileLine{text: text[i], ending: ending}
	}
	return lines
}

func replaceProfile(profile, slot, art string) (string, error) {
	profileLines := splitProfileLines(profile)
	lines := profileTextLines(profileLines)

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
		ending := profileLineEnding(profileLines, start)
		replacement := newProfileLines(block, ending)
		replacement[len(replacement)-1].ending = profileLines[end-1].ending
		profileLines = spliceProfileLines(profileLines, start, end, replacement)
	} else {
		insert := len(lines)
		if insert > 0 && lines[insert-1] == "" {
			insert--
		}
		ending := profileLineEnding(profileLines, insert)
		replacement := newProfileLines(block, ending)
		if insert == len(profileLines) {
			if len(profileLines) > 0 {
				profileLines[len(profileLines)-1].ending = ending
			}
			replacement[len(replacement)-1].ending = ""
		}
		profileLines = spliceProfileLines(profileLines, insert, insert, replacement)
	}
	lines = profileTextLines(profileLines)

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
			profileLines[i].text = lines[i]
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
		ending := profileLineEnding(profileLines, insert)
		profileLines = spliceProfileLines(profileLines, insert, insert, []profileLine{{text: "ansi_style=true", ending: ending}})
	}

	return joinProfileLines(profileLines), nil
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
