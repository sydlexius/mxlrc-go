# MxLRC-Go (sydlexius/mxlrcsvc-go)

## What This Is

A Go CLI tool that fetches synced lyrics from the Musixmatch API and saves them as `.lrc` files. This is a fork of fashni/mxlrc-go being restructured into a maintainable Go project layout under a new module path, with global state eliminated and the hardcoded API token externalized.

## Core Value

The tool fetches synced lyrics reliably and writes correct `.lrc` files. Everything else (project structure, config handling, CI) exists to support that.

## Requirements

### Validated

- Fetch synced lyrics from Musixmatch desktop API -- existing
- Write `.lrc` files with metadata tags (artist, title, album, length) -- existing
- Support three input modes: CLI pairs, text file, directory scan -- existing
- Read audio file metadata via ID3/MP4/FLAC tags for directory mode -- existing
- Rate limiting via configurable cooldown between API calls -- existing
- Graceful shutdown with failed-item retry file (`_failed.txt`) -- existing
- Cross-platform builds (linux/darwin/windows, amd64/arm64) -- existing
- BFS/DFS directory traversal options -- existing

### Active

- [ ] Rename Go module to `sydlexius/mxlrcsvc-go`
- [ ] Restructure flat main package into `cmd/mxlrcsvc-go/` + `internal/` layout
- [ ] Eliminate global `inputs` and `failed` variables
- [ ] Externalize Musixmatch token (CLI flag > env var > .env file)
- [ ] Define `Fetcher` interface for the Musixmatch client
- [ ] Export types and methods from internal packages
- [ ] Update Makefile, CI workflows, and goreleaser for new binary name and paths
- [ ] Update README for new module path and binary name

### Out of Scope

- New features or behavioral changes beyond restructuring -- M0 is structural only
- Additional test coverage beyond relocating existing tests -- deferred to later milestone
- Database or persistent state -- not needed yet
- Web server or API mode -- not planned for M0

## Context

This is a fork of `fashni/mxlrc-go`, itself a Go port of the Python MxLRC tool. The codebase is small (~5 files, single `main` package) but has known structural issues: global mutable state, hardcoded API token, flat layout that won't scale. M0 addresses all of these before any feature work begins.

The existing codebase has minimal test coverage (`utils_test.go` only). Quality gating relies heavily on linters (golangci-lint with 12 linters), pre-commit hooks, and CI.

Target layout after M0:
```
cmd/mxlrcsvc-go/main.go
internal/models/models.go       (from structs.go)
internal/musixmatch/client.go   (from musixmatch.go)
internal/lyrics/writer.go       (from lyrics.go + slugify)
internal/scanner/scanner.go     (from utils.go scanner functions)
```

## Constraints

- **Binary name**: `mxlrcsvc-go` (matches new module name)
- **No CGO**: Must remain CGO_ENABLED=0 for cross-compilation
- **Go 1.22+**: Minimum Go version from existing go.mod
- **Behavior preservation**: All existing CLI flags and behaviors must work identically after restructuring
- **Token precedence**: CLI flag > environment variable (`MUSIXMATCH_TOKEN`) > `.env` file

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Module path: `sydlexius/mxlrcsvc-go` | New fork identity, distinct from upstream | -- Pending |
| App struct for state ownership | Replaces global `inputs`/`failed` vars; enables testability | -- Pending |
| Token: flag + env + .env | Maximum flexibility; flag for scripting, env for CI, .env for local dev | -- Pending |
| `Fetcher` interface on Musixmatch client | Enables mocking in tests without hitting the real API | -- Pending |
| Move existing tests only, no new coverage | Keep M0 scope tight; test coverage is a separate concern | -- Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? -> Move to Out of Scope with reason
2. Requirements validated? -> Move to Validated with phase reference
3. New requirements emerged? -> Add to Active
4. Decisions to log? -> Add to Key Decisions
5. "What This Is" still accurate? -> Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check -- still the right priority?
3. Audit Out of Scope -- reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-10 after initialization*
