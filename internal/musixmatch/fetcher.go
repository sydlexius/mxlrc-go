package musixmatch

import "github.com/sydlexius/mxlrcsvc-go/internal/models"

// Fetcher abstracts lyrics lookup from the Musixmatch API.
type Fetcher interface {
	FindLyrics(track models.Track) (models.Song, error)
}
