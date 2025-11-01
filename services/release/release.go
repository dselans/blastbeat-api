package release

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/superpowerdotcom/go-common-lib/clog"
	"go.uber.org/zap"

	"github.com/dselans/blastbeat-api/backends/db"
	"github.com/dselans/blastbeat-api/backends/gensql"
)

type IRelease interface {
	GetReleases(ctx context.Context, filters *ReleaseFilters) ([]*ReleaseResponse, error)
}

type Release struct {
	opts *Options
	log  clog.ICustomLog
}

type Options struct {
	Backend *db.DB
	Log     clog.ICustomLog
}

type ReleaseFilters struct {
	DateFrom         *time.Time
	DateTo           *time.Time
	DateExact        *time.Time
	IncludedGenres   []string
	ExcludedGenres   []string
	ExcludedKeywords []string
	FollowerRange    string
}

type ReleaseResponse struct {
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	Artist        string         `json:"artist"`
	AlbumArt      string         `json:"albumArt"`
	ReleaseDate   string         `json:"releaseDate"`
	Label         string         `json:"label"`
	LabelUrl      *string        `json:"labelUrl,omitempty"`
	FollowerCount int32          `json:"followerCount"`
	Genres        []string       `json:"genres"`
	Country       *string        `json:"country,omitempty"`
	ExternalLinks []ExternalLink `json:"externalLinks,omitempty"`
	PreviewLinks  PreviewLinks   `json:"previewLinks"`
}

type ExternalLink struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type PreviewLinks struct {
	Spotify  *string `json:"spotify,omitempty"`
	Youtube  *string `json:"youtube,omitempty"`
	Bandcamp *string `json:"bandcamp,omitempty"`
}

func New(opts *Options) (*Release, error) {
	if err := validateOptions(opts); err != nil {
		return nil, errors.Wrap(err, "failed to validate options")
	}

	return &Release{
		opts: opts,
		log:  opts.Log.With(zap.String("pkg", "release")),
	}, nil
}

func validateOptions(opts *Options) error {
	if opts == nil {
		return errors.New("options cannot be nil")
	}

	if opts.Backend == nil {
		return errors.New("backend cannot be nil")
	}

	if opts.Log == nil {
		return errors.New("log cannot be nil")
	}

	return nil
}

func (r *Release) GetReleases(ctx context.Context,
	filters *ReleaseFilters) ([]*ReleaseResponse, error) {
	logger := r.log.With(zap.String("method", "GetReleases"))
	logger.Debug("Fetching releases", zap.Any("filters", filters))

	var dbReleases []gensql.Release
	var err error

	if filters.DateExact != nil {
		dbReleases, err = r.opts.Backend.ListReleasesByExactDate(ctx,
			*filters.DateExact)
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch releases by exact date")
		}
	} else if filters.DateFrom != nil {
		var dateTo time.Time

		if filters.DateTo != nil {
			dateTo = *filters.DateTo
		} else {
			dateTo = *filters.DateFrom
		}

		dbReleases, err = r.opts.Backend.ListReleasesByDateRange(ctx,
			gensql.ListReleasesByDateRangeParams{
				ReleaseDate:   *filters.DateFrom,
				ReleaseDate_2: dateTo,
			})
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch releases by date range")
		}
	} else {
		dbReleases, err = r.opts.Backend.ListReleases(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch releases")
		}
	}

	// Convert to response format
	releases := make([]*ReleaseResponse, 0, len(dbReleases))
	for _, dbRelease := range dbReleases {
		release := convertDBReleaseToResponse(dbRelease)
		releases = append(releases, release)
	}

	releases = r.applyFilters(releases, filters)

	logger.Debug("Returning releases", zap.Int("count", len(releases)))
	return releases, nil
}

func convertDBReleaseToResponse(
	dbRelease gensql.Release) *ReleaseResponse {
	var genres []string
	if err := json.Unmarshal(dbRelease.Genres, &genres); err != nil {
		genres = []string{}
	}

	var externalLinks []ExternalLink

	if len(dbRelease.ExternalLinks) > 0 {
		if err := json.Unmarshal(dbRelease.ExternalLinks, &externalLinks); err != nil {
			externalLinks = []ExternalLink{}
		}
	}

	response := &ReleaseResponse{
		ID:            dbRelease.ID.String(),
		Title:         dbRelease.Title,
		Artist:        dbRelease.Artist,
		AlbumArt:      dbRelease.AlbumArtUrl,
		ReleaseDate:   dbRelease.ReleaseDate.Format("2006-01-02"),
		Label:         dbRelease.Label,
		FollowerCount: dbRelease.FollowerCount,
		Genres:        genres,
		ExternalLinks: externalLinks,
		PreviewLinks:  PreviewLinks{},
	}

	// Handle optional fields
	if dbRelease.LabelUrl.Valid {
		response.LabelUrl = &dbRelease.LabelUrl.String
	}

	if dbRelease.Country.Valid {
		response.Country = &dbRelease.Country.String
	}

	if dbRelease.SpotifyUrl.Valid {
		response.PreviewLinks.Spotify = &dbRelease.SpotifyUrl.String
	}

	if dbRelease.YoutubeUrl.Valid {
		response.PreviewLinks.Youtube = &dbRelease.YoutubeUrl.String
	}

	if dbRelease.BandcampUrl.Valid {
		response.PreviewLinks.Bandcamp = &dbRelease.BandcampUrl.String
	}

	return response
}

func (r *Release) applyFilters(releases []*ReleaseResponse,
	filters *ReleaseFilters) []*ReleaseResponse {
	filtered := make([]*ReleaseResponse, 0)

	for _, release := range releases {
		if len(filters.IncludedGenres) > 0 {
			if !hasAllGenres(release.Genres,
				filters.IncludedGenres) {
				continue
			}
		}

		if len(filters.ExcludedGenres) > 0 {
			if hasAnyGenre(release.Genres,
				filters.ExcludedGenres) {
				continue
			}
		}

		if len(filters.ExcludedKeywords) > 0 {
			if containsKeywords(release.Title, release.Artist,
				filters.ExcludedKeywords) {
				continue
			}
		}

		if filters.FollowerRange != "" {
			if !matchesFollowerRange(release.FollowerCount,
				filters.FollowerRange) {
				continue
			}
		}

		filtered = append(filtered, release)
	}

	return filtered
}

func hasAllGenres(releaseGenres []string,
	requiredGenres []string) bool {
	releaseGenreMap := make(map[string]bool)
	for _, genre := range releaseGenres {
		releaseGenreMap[strings.ToLower(genre)] = true
	}

	for _, required := range requiredGenres {
		if !releaseGenreMap[strings.ToLower(required)] {
			return false
		}
	}
	return true
}

func hasAnyGenre(releaseGenres []string, excludedGenres []string) bool {
	releaseGenreMap := make(map[string]bool)
	for _, genre := range releaseGenres {
		releaseGenreMap[strings.ToLower(genre)] = true
	}

	for _, excluded := range excludedGenres {
		if releaseGenreMap[strings.ToLower(excluded)] {
			return true
		}
	}
	return false
}

func containsKeywords(title, artist string, keywords []string) bool {
	text := strings.ToLower(title + " " + artist)
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func matchesFollowerRange(followerCount int32, rangeKey string) bool {
	buckets := map[string]struct {
		min int32
		max int32
	}{
		"<1K":   {0, 999},
		"1K+":   {1000, 9999},
		"10K+":  {10000, 99999},
		"100K+": {100000, 999999},
		"1M+":   {1000000, 1999999},
		"2M+":   {2000000, 4999999},
		"5M+":   {5000000, 2147483647}, // Max int32
	}

	bucket, exists := buckets[rangeKey]
	if !exists {
		return true // Unknown range, don't filter
	}

	return followerCount >= bucket.min && followerCount <= bucket.max
}
