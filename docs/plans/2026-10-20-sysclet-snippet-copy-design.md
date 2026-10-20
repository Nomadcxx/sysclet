# Snippet Copy Design

The sysc-greet export preview is rendered inside the styled TUI, so terminal selection also captures surrounding interface text. On the export screen, F5 will copy the generated sysc-greet profile snippet directly to the system clipboard. The copied text will come from `ProfileSnippet` and retain its literal `\\033` escapes; it will not include preview colors or TUI styling.

Use the existing `github.com/atotto/clipboard` package, which selects `wl-copy` on Wayland and clipboard helpers such as `xclip` or `xsel` on X11. Show a success message only after the clipboard write succeeds, and show the returned error otherwise. Add the F5 hint to the export screen. Clipboard copy is available only for the sysc-greet format.
