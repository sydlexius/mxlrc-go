package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/sydlexius/mxlrcsvc-go/internal/normalize"
)

// CacheRepo provides read/write access to the lyrics_cache table.
// All artist/title/album strings are normalized before storage and lookup.
type CacheRepo struct {
	db *sql.DB
}

// New returns a CacheRepo backed by db.
func New(db *sql.DB) *CacheRepo {
	return &CacheRepo{db: db}
}

// LookupExact returns the cached lyrics for the exact (artist, title, album) triple
// after normalization. Returns sql.ErrNoRows if not found.
func (r *CacheRepo) LookupExact(ctx context.Context, artist, title, album string) (string, error) {
	var lyrics string
	err := r.db.QueryRowContext(ctx,
		`SELECT lyrics FROM lyrics_cache WHERE artist=? AND title=? AND album=? LIMIT 1`,
		normalize.NormalizeKey(artist),
		normalize.NormalizeKey(title),
		normalize.NormalizeKey(album),
	).Scan(&lyrics)
	if errors.Is(err, sql.ErrNoRows) {
		return "", sql.ErrNoRows
	}
	if err != nil {
		return "", fmt.Errorf("cache: lookup exact: %w", err)
	}
	return lyrics, nil
}

// LookupFallback returns the first cached lyrics matching (artist, title) regardless
// of album, after normalization. Returns sql.ErrNoRows if not found.
func (r *CacheRepo) LookupFallback(ctx context.Context, artist, title string) (string, error) {
	var lyrics string
	err := r.db.QueryRowContext(ctx,
		`SELECT lyrics FROM lyrics_cache WHERE artist=? AND title=? LIMIT 1`,
		normalize.NormalizeKey(artist),
		normalize.NormalizeKey(title),
	).Scan(&lyrics)
	if errors.Is(err, sql.ErrNoRows) {
		return "", sql.ErrNoRows
	}
	if err != nil {
		return "", fmt.Errorf("cache: lookup fallback: %w", err)
	}
	return lyrics, nil
}

// Store inserts or updates (upsert) the lyrics for the (artist, title, album) triple.
// Keys are normalized before storage.
func (r *CacheRepo) Store(ctx context.Context, artist, title, album, lyrics string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO lyrics_cache (artist, title, album, lyrics)
         VALUES (?, ?, ?, ?)
         ON CONFLICT(artist, title, album) DO UPDATE SET
             lyrics     = excluded.lyrics,
             updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`,
		normalize.NormalizeKey(artist),
		normalize.NormalizeKey(title),
		normalize.NormalizeKey(album),
		lyrics,
	)
	if err != nil {
		return fmt.Errorf("cache: store: %w", err)
	}
	return nil
}
