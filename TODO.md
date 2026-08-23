# TODO

## Scanner: support loose book files (no .inpx required)

**Status:** open
**Area:** `scanner/scanner.go`

### Problem

`ScanLibrary` imports books **only** from `.inpx` indexes. The directory walk
(`scanner/scanner.go`, top of `ScanLibrary`) also collects loose `.fb2` /
`.zip` / `.7z` files into a `files` variable (via the `exts` map) — but that
variable is never used. The collection is dead code, a leftover of an
unimplemented idea.

Intended behavior: loose files not covered by an `.inpx` should be parsed and
added to the database too, most likely as a second producer feeding the same
`entries` channel that `checkInpxFiles` feeds today.

### Consequence

- A library **without** `.inpx` indexes imports nothing, silently (the scan
  succeeds, zero books). The deadlock on that path was fixed, but the empty
  result stands.
- Loose files placed next to an indexed library are ignored.
- The dead `files` / `exts` collection misleads readers into thinking
  loose-file scanning works.

### Scope

When implementing:

- Decide how to extract metadata for loose files (an `.inpx` carries the
  fields; a bare `.fb2` must be parsed itself — see the `scanner` package's
  FB2 parsing and the `converter` package for archive handling).
- Add the second producer feeding `entries`; keep the unconditional
  `close(entries)` ownership rules established in the current code.
- Either way, remove the dead `files` / `exts` collection (or finally use it).
- Cover with a test: a directory with loose files and no `.inpx` imports > 0
  books (today it imports 0).

If loose-file support stays out of scope, at minimum delete the dead
`files` / `exts` walk so the code matches actual behavior.

### Where to look

- `scanner/scanner.go` — `ScanLibrary` walk (dead `files`/`exts` collection),
  `checkInpxFiles` (the `.inp`-driven import path that actually runs),
  `parseInpEntry` (the field layout `.inp` lines provide).
- `scanner/scanner_test.go` — existing fake `Storager` and the timeout-guard
  pattern for hang-prone paths.
