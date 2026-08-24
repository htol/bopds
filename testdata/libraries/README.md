# Sample libraries

A checked-in fixture of two library roots used by `app/fixture_test.go`
(`TestFixture_LibrariesScan`) and by `make scan-fixture` for manual UI checks.

```text
testdata/libraries/
├── r1/                 # root 1 (LIBRARY_PATH entry 1)
│   └── libA/           # library "libA", loose .inp source
│       ├── libA.inp    # 6 books, index lines
│       └── libA.zip    # companion archive next to the .inp
└── r2/                 # root 2 (LIBRARY_PATH entry 2)
    └── libB/           # library "libB", .inpx source
        ├── libB.inpx   # zip containing libB.inp
        └── sub/
            └── libB.zip # companion archive nested in a subdirectory
```

What the fixture demonstrates:

- Both index sources: loose `.inp` (libA) and `.inpx` with a nested companion
  archive (libB).
- Cross-library duplicates: "Shadow Protocol" and "Quiet Harbors" exist in
  both libraries with different file sizes — the REST API groups them into
  one card with two `copies` (library, display name, size, download link).
- Display-name seeding: `make scan-fixture` passes
  `LIBRARY_NAMES="libA:Fiction Library,libB:Tech Library"` so cards and OPDS
  entries show human names instead of directory names.
- Valid FB2 content inside every archive, so FB2/EPUB/MOBI downloads work
  from the UI out of the box.

Manual check:

```bash
make scan-fixture   # scans into ./books.db
make serve          # http://localhost:3001 — search "Shadow" or "Quiet"
```
