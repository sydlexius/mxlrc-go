---
reviewed: 2026-04-10T00:00:00Z
depth: standard
files_reviewed: 2
files_reviewed_list:
  - internal/app/app.go
  - internal/scanner/scanner.go
findings:
  critical: 1
  warning: 1
  info: 1
  total: 3
status: issues_found
verdict: BLOCK
---

# Code Review: CSV writer change in `handleFailed()`

**Reviewed:** 2026-04-10
**Depth:** standard
**Files Reviewed:** 2
**Verdict:** BLOCK

## Summary

The `csv.Writer` adoption in `handleFailed()` (`app.go`) is mechanically correct — writer
construction, `Write()`, `Flush()`, and `Error()` are all used properly. However the change
introduces a **critical correctness regression**: the downstream reader in `scanner.go`
(`GetSongText` → `AssertInput`) still parses the retry file using `strings.Split(song, ",")`
with no CSV decoding. The moment `csv.Writer` emits a quoted field (which it will for any
artist or track name that contains a comma), the retry file becomes silently unreadable.
Every affected item is dropped without error — the user sees no warning and loses their retry
list for those entries.

There is also a secondary warning about incorrect close-on-error ordering for the flush error
path, and a minor naming observation.

---

## Critical Issues

### CR-01: Retry file writer is CSV; reader is `strings.Split` — format mismatch silently drops items

**Files:**
- Writer: `internal/app/app.go:139` — `buffer.Write([]string{cur.Track.ArtistName, cur.Track.TrackName})`
- Reader: `internal/scanner/scanner.go:32-40` — `AssertInput()` calls `strings.Split(song, ",")`

**Issue:**
`csv.Writer` quotes any field that contains a comma, double-quote, or newline per RFC 4180.
`AssertInput` calls `strings.Split(line, ",")` and rejects any line where the split does not
yield exactly two parts (`len(s) != 2`).

Concrete failure cases (confirmed by running both paths):

| Artist / Track written | Line emitted by csv.Writer | AssertInput result |
|---|---|---|
| `"Artist, The"` / `"Song Title"` | `"Artist, The",Song Title` | **rejected** (3 parts after split) |
| `"Artist"` / `"Song, Title"` | `Artist,"Song, Title"` | **rejected** (3 parts after split) |
| `"Artist \"Live\""` / `"Song"` | `"Artist ""Live""",Song` | **rejected** (3 parts) |
| `"Artist"` / `"Song\nB-side"` | two-line CSV record | **rejected** (first line has 1 part) |

The failure is **silent**: `AssertInput` returns `nil`, `GetSongMulti` logs a `slog.Warn`
("invalid input") and continues. No error is returned to the caller; the retry list is silently
shorter than it should be.

Comma-containing names are genuinely common in music metadata ("The Beatles", "AC/DC" are
fine, but "Artist, The" style is widespread, and track subtitles like "Paranoid Android
(feat. Thom Yorke, live)" are routine).

**Fix — Option A (preferred): make the reader also use `csv.Reader`**

Replace `GetSongText`'s line-scan + `AssertInput` path for the retry file, OR change
`AssertInput` to use `csv.Reader` so it handles quoted fields:

```go
// internal/scanner/scanner.go — replace AssertInput body
func AssertInput(song string) *models.Track {
    r := csv.NewReader(strings.NewReader(song))
    fields, err := r.Read()
    if err != nil || len(fields) != 2 {
        return nil
    }
    return &models.Track{
        ArtistName: strings.TrimSpace(fields[0]),
        TrackName:  strings.TrimSpace(fields[1]),
    }
}
```

Add `"encoding/csv"` to scanner imports. This is backwards-compatible: unquoted `artist,title`
lines still parse correctly via `csv.Reader`.

**Fix — Option B (simpler, no reader change): revert to raw comma-join in the writer**

The original `bufio.Writer` approach with `artist + "," + title` at least fails consistently
for comma-containing names on both sides of the round-trip. If the intent of this PR was only
to fix the write side without touching the scanner, the change is incomplete and should be
reverted until both sides are updated together.

---

## Warnings

### WR-01: File closed before `buffer.Error()` check — unconsumed buffered data may be silently lost

**File:** `internal/app/app.go:144-151`

```go
buffer.Flush()
if err := buffer.Error(); err != nil {
    _ = f.Close()                                          // line 146
    return fmt.Errorf("flushing failed items: %w", err)
}
if err := f.Close(); err != nil {                          // line 149
    return fmt.Errorf("closing failed items file: %w", err)
}
```

**Issue:**
This ordering is correct for the happy path, but on the flush-error branch (line 146) the
code calls `_ = f.Close()` and discards the close error. This is acceptable — you are already
returning a write error — but note that `buffer.Error()` captures the *first* error that
occurred during any prior `Write()` or `Flush()` call, not just the final `Flush()`. If a
`Write()` inside the loop at line 139 returned an error that was caught and returned early, the
code already handles that. However, if `buffer.Write()` internally buffers the error (csv
buffers nothing — it writes directly to the underlying writer), there is no real double-error
risk here.

The more substantive concern: `Flush()` at line 144 is called **unconditionally** regardless
of whether the loop exited via error (lines 136-137 return early, so this is fine) or via
successful completion. The call order is correct. This is a **minor warning**, not a bug, but
worth noting for the next reader: the error from `f.Close()` on the flush-error path (line 146)
is silently discarded. Per project conventions (`AGENTS.md`) discard is acceptable only "when
already returning a real error" — this case qualifies, so the `_ =` discard is justified.

**Actual concern in this warning:** If `f.Sync()` or an OS-level flush is ever needed (e.g.
on network filesystems), `f.Close()` returning an error on the success path (line 149-151) is
the only place that would be surfaced. This is already handled correctly. No code change
needed, but reviewers should be aware of the intentional discard on line 146.

> Severity downgraded to Warning because no data is actually lost under the current code paths
> — the early returns on `buffer.Write` error (lines 139-142) prevent the flush-then-discard
> scenario from occurring in practice.

---

## Info

### IN-01: Variable named `buffer` holds a `*csv.Writer` — name is a leftover from the `bufio` era

**File:** `internal/app/app.go:132`

```go
buffer := csv.NewWriter(f)
```

**Issue:**
`buffer` was the natural name when the value was a `*bufio.Writer`. Now it holds a
`*csv.Writer`. The name is not wrong, but it is misleading — `csv.Writer` has no exposed
buffer concept; calling it `buffer` implies byte-buffering semantics that no longer apply.

**Fix:** Rename to `cw` (consistent with the project's abbreviated-name convention: `c` for
client, `w` for writer, `sc` for scanner) or simply `w`:

```go
w := csv.NewWriter(f)
```

---

## Verdict

**BLOCK** — CR-01 is a silent correctness regression that defeats the stated purpose of the
change. The fix for comma-in-names works on the write side only; the read side (`AssertInput`)
still uses `strings.Split` and will silently discard any retry entry whose artist or track name
contains a comma. The two sides must be updated together, or the writer change reverted, before
this lands.

---

_Reviewed: 2026-04-10_
_Reviewer: gsd-code-reviewer (claude-sonnet-4.6)_
_Depth: standard_
