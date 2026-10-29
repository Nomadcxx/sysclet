# sysclet rules

The rules of the organisation are in the shell repository's `AGENTS.md`; this file adds what is
specific to sysclet. CONTRIBUTING.md has the commit gate, the tracker and the release order.

- Go only. Builds are `CGO_ENABLED=0`; do not add CGO.
- Stop at the first working rung: existing sysclet code, the Go standard library, a pinned
  dependency, then new code.
- The TDF parser is a trust boundary: keep its size, record and expanded-glyph limits in `tdf.go`.
- Bundled fonts change only through `scripts/import_fonts.py` or a new pack plus catalog entries;
  `TestBundledFontCatalogMatchesEveryEmbeddedRecord` pins the catalog count and pack count.
- Writes to user files (sysc-greet profiles, sysc-walls `daemon.conf`, export targets) back up the
  original first and replace it atomically; keep that in any new writer.
- Change the `go` directive and module pins on purpose; check `go.mod` after `go get` or
  `go mod tidy`, which can rewrite the directive.
- Plans live in `docs/plans/`, gitignored. Status lives in bd, run from `/home/nomadx/sysc-shell`.
