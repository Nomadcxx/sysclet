# Sysclet Snippet Copy Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Let users copy the exact sysc-greet profile snippet from sysclet's export screen without selecting terminal UI text.

**Architecture:** Handle F5 in the existing export key handler. Generate the same plain profile snippet used by the preview and write it using the existing clipboard package; report success or failure in the export status. Show the key hint beside the existing export actions.

**Tech Stack:** Go, Bubble Tea, `github.com/atotto/clipboard`.

---

### Task 1: Copy a profile snippet from the export screen

**Files:**
- Modify: `tui.go`
- Modify: `go.mod` and `go.sum` to declare the existing clipboard package as a direct dependency

**Steps:**
1. Import `github.com/atotto/clipboard` in `tui.go`.
2. Handle F5 in `updateExport`; reject an empty preview, generate the profile snippet from the current slot and preview, and call `clipboard.WriteAll` with the plain snippet.
3. Set an accurate status for successful copy and clipboard errors.
4. Add `F5 copy snippet` to the export help when the selected format is sysc-greet.
5. Run `go build ./...` to confirm the package compiles.
