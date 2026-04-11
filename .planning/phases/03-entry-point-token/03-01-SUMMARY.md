---
phase: 03-entry-point-token
plan: 01
subsystem: entry-point
tags: [token, godotenv, security, layout]
dependency_graph:
  requires: [02-01-SUMMARY.md]
  provides: [cmd/mxlrcsvc-go/main.go, token-precedence-chain]
  affects: [go.mod, go.sum, .gitignore]
tech_stack:
  added: [github.com/joho/godotenv v1.5.1]
  patterns: [token-precedence-chain, dotenv-optional-load]
key_files:
  created: [cmd/mxlrcsvc-go/main.go]
  modified: [go.mod, go.sum, .gitignore]
  deleted: [main.go]
decisions:
  - godotenv.Load() called before signal context setup so env vars are available for all subsequent logic
  - Token error uses os.Exit(1) with slog.Error (not log.Fatal) for consistent structured logging
  - /mxlrcsvc-go (with leading slash) in .gitignore to target root binary only, not cmd/ directory
metrics:
  duration: 2min
  completed: "2026-04-11T00:07:03Z"
  tasks_completed: 2
  files_changed: 5
---

# Phase 3 Plan 1: Entry Point & Token Externalization Summary

**One-liner:** godotenv-powered entry point at cmd/mxlrcsvc-go/main.go with CLI flag > MUSIXMATCH_TOKEN env var > .env file token precedence, hardcoded token removed.

## What Was Built

Created the final entry point at `cmd/mxlrcsvc-go/main.go` and externalized the Musixmatch API token with a proper precedence chain. The old root `main.go` containing the hardcoded default token was deleted.

**Token precedence chain (CLI flag > env var > .env file):**
1. `--token` CLI flag (checked first via `args.Token`)
2. `MUSIXMATCH_TOKEN` environment variable (may have been loaded from `.env` by godotenv)
3. If neither: structured error + `os.Exit(1)`

The `godotenv.Load()` call is intentionally placed before any token logic. Because godotenv only sets env vars that don't already exist, a real `MUSIXMATCH_TOKEN` in the environment automatically takes precedence over the `.env` file — no explicit ordering code needed.

## Commits

| Task | Commit | Message |
|------|--------|---------|
| Task 1 | `034ca79` | feat(03-01): create cmd/mxlrcsvc-go/main.go with godotenv token precedence |
| Task 2 | `85c3286` | feat(03-01): delete root main.go, sole entry point is now cmd/mxlrcsvc-go/main.go |

## Success Criteria Verification

- [x] `cmd/mxlrcsvc-go/main.go` is the sole entry point (LAYOUT-01)
- [x] Token precedence: CLI flag > env var > .env file, with error on missing (API-02)
- [x] Zero hardcoded tokens in source — hardcoded token `2203269256ff7abcb649269df00e14c833dbf4ddfb5b36a1aae8b0` removed (API-03)
- [x] godotenv v1.5.1 in go.mod as direct dependency (BUILD-07)
- [x] All existing tests pass (`go test ./...`)
- [x] All lints clean (`go vet ./...`, golangci-lint via pre-commit)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing] Fixed .gitignore missing new binary name**
- **Found during:** Task 1 commit staging
- **Issue:** `.gitignore` only excluded `mxlrc-go` (old binary name) but not `mxlrcsvc-go` (new binary name). The compiled binary `mxlrcsvc-go` would appear as untracked after every `go build`.
- **Fix:** Added `mxlrcsvc-go` to `.gitignore`, then refined to `/mxlrcsvc-go` (with leading slash) to prevent the pattern from accidentally matching the `cmd/mxlrcsvc-go/` source directory.
- **Files modified:** `.gitignore`
- **Commit:** `034ca79`

## Known Stubs

None — token loading is fully wired. No placeholder data flows to any output.

## Threat Surface Scan

No new network endpoints, auth paths, or file access patterns beyond what the plan's threat model documents. The `.env` file loading via `godotenv.Load()` is contained in the entry point and documented in T-03-02.

## Self-Check

- `cmd/mxlrcsvc-go/main.go` exists: FOUND
- `main.go` deleted: CONFIRMED (test ! -f main.go passes)
- Commit `034ca79` exists: FOUND
- Commit `85c3286` exists: FOUND
- godotenv in go.mod as direct dep: CONFIRMED
- No hardcoded token in any .go file: CONFIRMED (grep returned empty)
