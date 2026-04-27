# Summary: M4 issues #18 and #44

## Completed

- Added a SQLite-backed work queue with normalized artist/title dedupe, atomic enqueue/dequeue, completion, and geometric retry backoff.
- Added migration coverage for work queue retry/dedupe columns and indexes.
- Added app-level fake Musixmatch tests covering successful fetch-to-write and fetch-failure retry-file behavior.

## Verification

```bash
go test ./...
go test -count=1 -coverprofile=/tmp/stillwater-cover.out ./...
go test -race -count=1 -covermode=atomic -coverprofile=/tmp/mxlrc-cover.out ./...
golangci-lint run
git diff --check
```

All checks passed.
