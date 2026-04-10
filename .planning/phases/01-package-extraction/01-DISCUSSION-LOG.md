# Phase 1: Package Extraction - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md -- this log preserves the alternatives considered.

**Date:** 2026-04-10
**Phase:** 01-package-extraction
**Areas discussed:** Package boundaries, Interface design, Error handling migration, Naming & export conventions

---

## Package Boundaries

### InputsQueue location

| Option | Description | Selected |
|--------|-------------|----------|
| Keep it with song data | InputsQueue stays in models alongside Track/Song/Lyrics/etc. | |
| Put it in the app package | InputsQueue moves to internal/app since it's processing state, not a domain model. | ✓ |
| Remove it, use a plain list | Replace InputsQueue entirely with a plain []Inputs slice. | |

**User's choice:** Put it in the app package
**Notes:** None

### Args struct location

| Option | Description | Selected |
|--------|-------------|----------|
| Args in main entry point | Args stays in the main entry point since it's about parsing CLI flags. Internal packages receive individual values. | ✓ |
| Args in models with other types | Args lives in models alongside the other types. Any package can import it. | |

**User's choice:** Args in main entry point
**Notes:** None

### 5-package split confirmation

| Option | Description | Selected |
|--------|-------------|----------|
| Looks good, next area | The 5-package split (models, musixmatch, lyrics, scanner, app) from the roadmap looks right. | ✓ |
| More questions about packages | I have questions about which code goes in which package. | |

**User's choice:** Looks good, next area
**Notes:** None

---

## Interface Design

### Interface scope

| Option | Description | Selected |
|--------|-------------|----------|
| Just the Fetcher interface | Only create the Fetcher interface for the API client. Minimal abstraction. | |
| Fetcher + Writer interfaces | Create interfaces for both the API client and the lyrics writer. More flexibility for testing. | ✓ |

**User's choice:** Fetcher + Writer interfaces
**Notes:** None

### Interface location

| Option | Description | Selected |
|--------|-------------|----------|
| In the implementing package | Fetcher in internal/musixmatch, Writer in internal/lyrics. | ✓ |
| In models with the data types | Both interfaces in internal/models. | |

**User's choice:** In the implementing package
**Notes:** None

---

## Error Handling Migration

### Internal package behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Errors only, no logging | Internal packages never crash or print. Only return errors. | |
| Return errors + allow info logging | Internal packages return errors (no log.Fatal) but can still print info messages. | ✓ |

**User's choice:** Return errors + allow info logging
**Notes:** None

### Logging framework

| Option | Description | Selected |
|--------|-------------|----------|
| Keep basic log package | Keep using log.Println/log.Printf. Structured logging deferred to v2. | |
| Switch to slog now | Switch to Go's log/slog everywhere. Pulls LOG-01 forward from v2. | ✓ |

**User's choice:** Switch to slog now
**Notes:** None

### slog scope

| Option | Description | Selected |
|--------|-------------|----------|
| Only in internal packages | Just use slog in the new internal packages. Main keeps basic log. | |
| Everything uses slog | Switch everything to slog -- internal packages and main entry point. | ✓ |

**User's choice:** Everything uses slog
**Notes:** Deliberate scope expansion from LOG-01 v2 requirement

---

## Naming & Export Conventions

### Snake case variables

| Option | Description | Selected |
|--------|-------------|----------|
| Fix during the move | Rename snake_case to camelCase while moving code into new packages. | ✓ |
| Leave for later | Focus on structural changes only. Clean up naming separately. | |

**User's choice:** Fix during the move
**Notes:** None

### URL constant handling

| Option | Description | Selected |
|--------|-------------|----------|
| Private to musixmatch package | Move URL constant as a private constant inside musixmatch package. | ✓ |
| Configurable via constructor | Make API base URL configurable via parameter. | |

**User's choice:** Private to musixmatch package
**Notes:** None

---

## Agent's Discretion

- Constructor function signatures
- File layout within each internal package
- How to split utils.go helpers
- slugify regex compilation approach

## Deferred Ideas

None -- discussion stayed within phase scope
