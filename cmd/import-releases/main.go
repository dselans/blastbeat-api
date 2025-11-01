package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/dselans/blastbeat-api/backends/db"
	"github.com/dselans/blastbeat-api/backends/gensql"
)

var httpClient = &http.Client{Timeout: 20 * time.Second}

const (
	defaultContactEmail = "admin@example.com"
	spotifyTokenURL     = "https://accounts.spotify.com/api/token"
	spotifySearchBase   = "https://api.spotify.com/v1/search"
	spotifyAlbumBase    = "https://api.spotify.com/v1/albums/"
	youtubeSearchBase   = "https://www.googleapis.com/youtube/v3/search"
	youtubeWatchBase    = "https://www.youtube.com/watch?v="
	maSearchBase        = "https://www.metal-archives.com/search"
	maBase              = "https://www.metal-archives.com"
	maAdvancedSearch    = "https://www.metal-archives.com/search/ajax-advanced/searching/bands/"
	discogsSearchBase   = "https://api.discogs.com/database/search"
	discogsArtistBase   = "https://api.discogs.com/artists"
	discogsLabelsBase   = "https://api.discogs.com/labels"
	discogsBase         = "https://www.discogs.com"
	musicBrainzBase     = "https://musicbrainz.org/ws/2"
	placeholderArtURL   = "https://via.placeholder.com/300"
)

var (
	logLevel    string
	levelDebug  bool
	enableWrite bool
	workers     int
	spotTok     string
	spotExp     time.Time
)

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}

	return def
}

func validateEnvVars() error {
	var missing []string

	if os.Getenv("SPOTIFY_CLIENT_ID") == "" {
		missing = append(missing, "SPOTIFY_CLIENT_ID")
	}

	if os.Getenv("SPOTIFY_CLIENT_SECRET") == "" {
		missing = append(missing, "SPOTIFY_CLIENT_SECRET")
	}

	if os.Getenv("DISCOGS_TOKEN") == "" {
		missing = append(missing, "DISCOGS_TOKEN")
	}

	if os.Getenv("YOUTUBE_API_KEY") == "" {
		missing = append(missing, "YOUTUBE_API_KEY")
	}

	if len(missing) > 0 {
		return fmt.Errorf("required environment variables not set: %s",
			strings.Join(missing, ", "))
	}

	return nil
}

func setLogLevel() {
	logLevel = strings.ToLower(getenv("LOG_LEVEL", "info"))
	levelDebug = (logLevel == "debug")

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	switch logLevel {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}
}

func main() {
	godotenv.Load()

	inPath := flag.String("in", "", "input CSV path (YYYY-MM-DD,Artist,Album,Label)")
	flag.BoolVar(&enableWrite, "enable-write", false, "enable writing to database (default: dry-run mode)")
	flag.IntVar(&workers, "workers", 1, "number of concurrent workers (default: 1)")
	flag.Parse()

	if *inPath == "" {
		log.Fatal("missing -in flag")
	}

	setLogLevel()

	if err := validateEnvVars(); err != nil {
		log.Fatalf("missing required environment variables: %v", err)
	}

	contact := getenv("CONTACT_EMAIL", defaultContactEmail)

	if !enableWrite {
		logrus.Info("DRY RUN MODE - no database writes will occur")
	}

	logrus.Infof("CSV enrich start (LOG_LEVEL=%s, contact=%s, file=%s, enable-write=%v, workers=%d)",
		logLevel, contact, *inPath, enableWrite, workers)

	var dbBackend *db.DB
	if enableWrite {
		dbPort := 5432
		if portStr := getenv("BLASTBEAT_API_DB_PORT", "5432"); portStr != "" {
			if p, err := strconv.Atoi(portStr); err == nil {
				dbPort = p
			}
		}

		var err error
		dbBackend, err = db.New(&db.Options{
			User:     getenv("BLASTBEAT_API_DB_USER", "blastbeat"),
			Password: getenv("BLASTBEAT_API_DB_PASSWORD", "blastbeat"),
			Host:     getenv("BLASTBEAT_API_DB_HOST", "localhost"),
			Port:     dbPort,
			DBName:   getenv("BLASTBEAT_API_DB_NAME", "blastbeat"),
			SSLMode:  getenv("BLASTBEAT_API_DB_SSL_MODE", "disable"),
		})
		if err != nil {
			log.Fatalf("failed to connect to database: %v", err)
		}
		defer dbBackend.GetDB().Close()
	}

	f, err := os.Open(*inPath)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = 4
	r.TrimLeadingSpace = true

	if workers < 1 {
		workers = 1
	}

	logrus.Infof("Starting import with %d worker(s)", workers)

	ctx := context.Background()

	type csvRow struct {
		rowNum  int
		dateISO string
		artist  string
		album   string
		label   string
	}

	type result struct {
		rowNum int
		err    error
		status string
	}

	csvRows := make(chan csvRow, workers*2)
	results := make(chan result, workers*2)
	var wg sync.WaitGroup

	seen := make(map[string]bool)
	var seenMu sync.Mutex

	var totalRows int64
	successCount := int64(0)
	skipCount := int64(0)
	errorCount := int64(0)

	rowNum := 0

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for row := range csvRows {
				dateISO := row.dateISO
				artist := row.artist
				album := row.album
				label := row.label

				key := releaseKey(dateISO, artist, album)
				seenMu.Lock()
				if seen[key] {
					seenMu.Unlock()
					logrus.Warnf("DUPE DETECTED! %d: %s | %s | %s",
						row.rowNum, dateISO, artist, album)
					results <- result{rowNum: row.rowNum, status: "dupe_skip"}
					continue
				}
				seen[key] = true
				seenMu.Unlock()

				logrus.Infof("Enriching release: %s - %s", artist, album)
				enriched := enrichRelease(dateISO, artist, album, label, contact)
				logrus.Infof("Enrichment complete - genres: %v, country: %s, sources: %v",
					enriched.Genres, enriched.Country, enriched.Sources)

				if !enableWrite {
					b, _ := json.MarshalIndent(enriched, "", "  ")
					logrus.Infof("DRY RUN - would insert release:\n%s", string(b))
					results <- result{rowNum: row.rowNum, status: "success"}
					continue
				}

				releaseDate, err := time.Parse("2006-01-02", dateISO)
				if err != nil {
					logrus.Errorf("row %d failed to parse date: %v", row.rowNum, err)
					results <- result{rowNum: row.rowNum, err: err, status: "error"}
					continue
				}

				exists, err := releaseExists(ctx, dbBackend, artist, album, releaseDate)
				if err != nil {
					logrus.Errorf("row %d failed to check for existing release: %v",
						row.rowNum, err)
					results <- result{rowNum: row.rowNum, err: err, status: "error"}
					continue
				}

				if exists {
					logrus.Warnf("row %d: release already exists - %s: %s (date: %s), skipping",
						row.rowNum, artist, album, dateISO)
					results <- result{rowNum: row.rowNum, status: "exists_skip"}
					continue
				}

				release, err := createReleaseFromEnriched(ctx, dbBackend, enriched)
				if err != nil {
					logrus.Errorf("row %d failed to insert: %v", row.rowNum, err)
					results <- result{rowNum: row.rowNum, err: err, status: "error"}
					continue
				}

				logrus.Infof("row %d: inserted release %s - %s: %s",
					row.rowNum, release.ID, release.Artist, release.Title)
				results <- result{rowNum: row.rowNum, status: "success"}
			}
		}()
	}

	go func() {
		for {
			rec, err := r.Read()
			if err == io.EOF {
				close(csvRows)
				return
			}
			if err != nil {
				logrus.Warnf("csv read: %v", err)
				atomic.AddInt64(&errorCount, 1)
				continue
			}
			rowNum++

			dateISO := strings.TrimSpace(rec[0])
			artist := strings.TrimSpace(rec[1])
			album := strings.TrimSpace(rec[2])
			label := strings.TrimSpace(rec[3])

			logrus.Infof("Processing row %d: %s | %s | %s", rowNum, dateISO, artist, album)

			if dateISO == "" || artist == "" || album == "" {
				logrus.Warnf("row %d missing required fields", rowNum)
				atomic.AddInt64(&skipCount, 1)
				continue
			}

			if _, err := time.Parse("2006-01-02", dateISO); err != nil {
				logrus.Warnf("row %d bad date %q: %v", rowNum, dateISO, err)
				atomic.AddInt64(&skipCount, 1)
				continue
			}

			atomic.AddInt64(&totalRows, 1)
			csvRows <- csvRow{
				rowNum:  rowNum,
				dateISO: dateISO,
				artist:  artist,
				album:   album,
				label:   label,
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		switch res.status {
		case "success":
			atomic.AddInt64(&successCount, 1)
		case "exists_skip", "dupe_skip":
			atomic.AddInt64(&skipCount, 1)
		case "error":
			atomic.AddInt64(&errorCount, 1)
		}
	}

	logrus.Infof("Done. Processed: %d, Success: %d, Skipped: %d, Errors: %d",
		atomic.LoadInt64(&totalRows), atomic.LoadInt64(&successCount),
		atomic.LoadInt64(&skipCount), atomic.LoadInt64(&errorCount))
}

func createReleaseFromEnriched(ctx context.Context, dbBackend *db.DB,
	enriched *enrichedRelease) (*gensql.Release, error) {

	releaseDate, err := time.Parse("2006-01-02", enriched.DateYMD)
	if err != nil {
		return nil, errors.Wrap(err, "invalid date")
	}
	genresJSON, err := json.Marshal(enriched.Genres)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal genres")
	}

	externalLinks := map[string]string{}

	if enriched.SpotifyAlbumURL != "" {
		externalLinks["spotify"] = enriched.SpotifyAlbumURL
	}

	if enriched.YoutubePreviewURL != "" {
		externalLinks["youtube"] = enriched.YoutubePreviewURL
	}

	if enriched.LabelDiscogsURL != "" {
		externalLinks["discogs"] = enriched.LabelDiscogsURL
	}

	externalLinksJSON, err := json.Marshal(externalLinks)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal external links")
	}

	spotifyURL := sql.NullString{}

	if enriched.SpotifyAlbumURL != "" {
		spotifyURL.String = enriched.SpotifyAlbumURL
		spotifyURL.Valid = true
	}

	youtubeURL := sql.NullString{}

	if enriched.YoutubePreviewURL != "" {
		youtubeURL.String = enriched.YoutubePreviewURL
		youtubeURL.Valid = true
	}

	labelURL := sql.NullString{}

	if enriched.LabelURL != "" {
		labelURL.String = enriched.LabelURL
		labelURL.Valid = true
	}

	if enriched.CoverArtURL == "" {
		enriched.CoverArtURL = placeholderArtURL
	}

	country := sql.NullString{}

	if enriched.Country != "" {
		country.String = strings.ToUpper(enriched.Country)
		country.Valid = true
	}

	release, err := dbBackend.CreateRelease(ctx, gensql.CreateReleaseParams{
		ID:            uuid.New(),
		Title:         enriched.Album,
		Artist:        enriched.Artist,
		AlbumArtUrl:   enriched.CoverArtURL,
		ReleaseDate:   releaseDate,
		Label:         enriched.Label,
		LabelUrl:      labelURL,
		FollowerCount: int32(enriched.SpotifyFollowers),
		Genres:        genresJSON,
		Country:       country,
		ExternalLinks: externalLinksJSON,
		SpotifyUrl:    spotifyURL,
		YoutubeUrl:    youtubeURL,
		BandcampUrl:   sql.NullString{},
	})
	if err != nil {
		return nil, err
	}
	return &release, nil
}

func releaseExists(ctx context.Context, dbBackend *db.DB,
	artist, album string, releaseDate time.Time) (bool, error) {
	query := `
		SELECT COUNT(*) 
		FROM releases 
		WHERE LOWER(artist) = LOWER($1) 
		  AND LOWER(title) = LOWER($2) 
		  AND release_date = $3
	`

	var count int
	err := dbBackend.GetDB().QueryRowContext(ctx, query, artist, album, releaseDate).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

type enrichedRelease struct {
	DateYMD           string            `json:"date_ymd"`
	Artist            string            `json:"artist"`
	Album             string            `json:"album"`
	Label             string            `json:"label"`
	Genres            []string          `json:"genres"`
	Country           string            `json:"country"`
	SpotifyPreviewURL string            `json:"spotify_preview_url"`
	YoutubePreviewURL string            `json:"youtube_preview_url"`
	SpotifyAlbumURL   string            `json:"spotify_album_url"`
	CoverArtURL       string            `json:"cover_art_url"`
	SpotifyFollowers  int64             `json:"spotify_followers"`
	SpotifyPopularity int               `json:"spotify_popularity"`
	Score             int               `json:"score"`
	LabelDiscogsURL   string            `json:"label_discogs_url"`
	LabelURL          string            `json:"label_url"`
	Sources           map[string]string `json:"sources"`
}

func enrichRelease(dateISO, artist, album, label, contact string) *enrichedRelease {
	out := &enrichedRelease{
		DateYMD: dateISO,
		Artist:  artist,
		Album:   album,
		Label:   label,
		Genres:  []string{},
		Sources: map[string]string{"csv": "1"},
	}

	logrus.Debugf("Starting Spotify lookup for %s - %s", artist, album)
	aid, fol, pop, albURL, cover, spGenres, spotAlbumID :=
		resolveSpotifyMetricsAndAlbum(artist, album)

	out.SpotifyFollowers = fol
	out.SpotifyPopularity = pop
	out.SpotifyAlbumURL = albURL
	out.CoverArtURL = cover

	if aid != "" {
		logrus.Debugf("Spotify artist found: ID=%s, followers=%d, popularity=%d",
			aid, fol, pop)
	} else {
		logrus.Debugf("Spotify artist not found for %s", artist)
	}

	if albURL != "" {
		out.SpotifyPreviewURL = albURL
		out.Sources["spotify_album"] = "1"
		logrus.Debugf("Spotify album found: %s", albURL)
	}

	if strings.TrimSpace(out.Label) == "" && spotAlbumID != "" {
		logrus.Debugf("Label missing, fetching from Spotify album %s", spotAlbumID)
		if l := getSpotifyAlbumLabel(spotAlbumID); l != "" {
			out.Label = l
			out.Sources["spotify_label"] = "1"
			logrus.Debugf("Label found from Spotify: %s", l)
		}
	}

	logrus.Debugf("Starting YouTube lookup for %s - %s", artist, album)
	if yt := findYouTubePreview(artist, album); yt != "" {
		out.YoutubePreviewURL = yt
		out.Sources["youtube_preview"] = "1"
		logrus.Debugf("YouTube preview found: %s", yt)
	} else {
		logrus.Debugf("YouTube preview not found")
	}

	logrus.Debugf("Starting Metal Archives lookup for %s", artist)
	ma := lookupMetalArchivesBandGenres(artist, contact)

	if len(ma) > 0 {
		out.Sources["metal_archives_band"] = "1"
		logrus.Debugf("Metal Archives genres found: %v", ma)
	} else {
		logrus.Debugf("Metal Archives genres not found")
	}

	logrus.Debugf("Starting Metal Archives country lookup for %s", artist)
	if out.Country == "" {
		if country := lookupCountryFromMetalArchives(artist); country != "" {
			out.Country = country
			out.Sources["metal_archives_country"] = "1"
			logrus.Debugf("Metal Archives country found: %s", country)
		} else {
			logrus.Debugf("Metal Archives country not found")
		}
	}

	logrus.Debugf("Starting Discogs styles lookup for %s - %s", artist, album)
	dc := lookupDiscogsStyles(artist, album, contact)

	if len(dc) > 0 {
		out.Sources["discogs_style"] = "1"
		logrus.Debugf("Discogs styles found: %v", dc)
	} else {
		logrus.Debugf("Discogs styles not found")
	}

	logrus.Debugf("Starting MusicBrainz country lookup for %s", artist)
	if out.Country == "" {
		if country := lookupCountryFromMusicBrainz(artist, contact); country != "" {
			out.Country = country
			out.Sources["musicbrainz_country"] = "1"
			logrus.Debugf("MusicBrainz country found: %s", country)
		} else {
			logrus.Debugf("MusicBrainz country not found")
		}
	}

	logrus.Debugf("Starting Discogs artist country lookup for %s", artist)
	if out.Country == "" {
		if country := lookupCountryFromDiscogsArtist(artist, contact); country != "" {
			out.Country = country
			out.Sources["discogs_country"] = "1"
			logrus.Debugf("Discogs country found: %s", country)
		} else {
			logrus.Debugf("Discogs country not found")
		}
	} else {
		logrus.Debugf("Country already found (%s), skipping Discogs lookup", out.Country)
	}

	sp := normalizeList(spGenres)

	if len(sp) > 0 && aid != "" {
		out.Sources["spotify_genres"] = "1"
		logrus.Debugf("Spotify genres: %v", sp)
	}

	out.Genres = unionPreserve(ma, dc, sp)
	logrus.Debugf("Combined genres: %v", out.Genres)

	logrus.Debugf("Starting label info resolution (current label: %s)", out.Label)
	discogsLink, website, finalName :=
		resolveLabelInfo(artist, album, out.Label, contact)

	if discogsLink != "" {
		out.LabelDiscogsURL = discogsLink
		out.Sources["discogs_label"] = "1"
		logrus.Debugf("Label Discogs URL found: %s", discogsLink)
	}

	if website != "" {
		normalized := normalizeURL(website)
		if normalized != "" {
			out.LabelURL = normalized
			out.Sources["label_website"] = "1"
			logrus.Debugf("Label website found: %s", normalized)
		} else {
			logrus.Debugf("Invalid website URL format, skipping: %s", website)
		}
	}

	if strings.TrimSpace(out.Label) == "" && finalName != "" {
		out.Label = finalName
		out.Sources["discogs_label_name"] = "1"
		logrus.Debugf("Label name found from Discogs: %s", finalName)
	}

	out.Score = computeScore(out.SpotifyFollowers, out.SpotifyPopularity)
	logrus.Debugf("Computed score: %d (followers: %d, popularity: %d)",
		out.Score, out.SpotifyFollowers, out.SpotifyPopularity)

	return out
}

func releaseKey(date, artist, album string) string {
	return strings.Join([]string{date, norm(artist), norm(album)}, "|")
}

func getSpotifyToken() string {
	if spotTok != "" && time.Now().Before(spotExp) {
		return spotTok
	}
	id := os.Getenv("SPOTIFY_CLIENT_ID")
	sec := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if id == "" || sec == "" {
		logrus.Warnf("Spotify credentials missing; skipping Spotify enrichment")
		return ""
	}
	form := url.Values{"grant_type": {"client_credentials"}}
	req, _ := http.NewRequest("POST", spotifyTokenURL,
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(id, sec)
	logrus.Debugf("REQ POST %s", req.URL.String())
	resp, err := httpClient.Do(req)
	if err != nil {
		logrus.Warnf("Spotify token: %v", err)
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		logrus.Warnf("Spotify token %d: %s", resp.StatusCode,
			strings.TrimSpace(string(b)))
		return ""
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	b, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(b, &tok)

	if tok.AccessToken == "" {
		return ""
	}

	spotTok = tok.AccessToken
	spotExp = time.Now().Add(time.Duration(tok.ExpiresIn-60) * time.Second)

	return spotTok
}

func resolveSpotifyMetricsAndAlbum(artist, album string) (artistID string,
	followers int64, popularity int, albumURL, coverURL string,
	artistGenres []string, albumID string) {
	tok := getSpotifyToken()

	if tok == "" {
		return
	}

	qA := url.QueryEscape(`artist:"` + artist + `"`)
	reqA, _ := http.NewRequest("GET",
		spotifySearchBase+"?type=artist&limit=1&q="+qA, nil)
	reqA.Header.Set("Authorization", "Bearer "+tok)
	logrus.Debugf("REQ GET %s", reqA.URL.String())

	respA, err := httpClient.Do(reqA)
	if err != nil {
		logrus.Warnf("Spotify artist search: %v", err)
		return
	}
	defer respA.Body.Close()
	var sa struct {
		Artists struct {
			Items []struct {
				ID        string `json:"id"`
				Followers struct {
					Total int64 `json:"total"`
				} `json:"followers"`
				Popularity int      `json:"popularity"`
				Genres     []string `json:"genres"`
			} `json:"items"`
		} `json:"artists"`
	}

	bA, _ := io.ReadAll(respA.Body)
	_ = json.Unmarshal(bA, &sa)

	if len(sa.Artists.Items) == 0 {
		return
	}

	a := sa.Artists.Items[0]

	artistID = a.ID
	followers = a.Followers.Total
	popularity = a.Popularity
	artistGenres = a.Genres

	qAlb := url.QueryEscape(fmt.Sprintf(`album:"%s" artist:"%s"`, album, artist))
	reqB, _ := http.NewRequest("GET",
		spotifySearchBase+"?type=album&limit=1&q="+qAlb, nil)
	reqB.Header.Set("Authorization", "Bearer "+tok)
	logrus.Debugf("REQ GET %s", reqB.URL.String())

	respB, err := httpClient.Do(reqB)
	if err != nil {
		logrus.Warnf("Spotify album search: %v", err)
		return
	}
	defer respB.Body.Close()
	var sb struct {
		Albums struct {
			Items []struct {
				ID           string            `json:"id"`
				ExternalURLs map[string]string `json:"external_urls"`
				Images       []struct {
					URL string `json:"url"`
				} `json:"images"`
			} `json:"items"`
		} `json:"albums"`
	}

	bB, _ := io.ReadAll(respB.Body)
	_ = json.Unmarshal(bB, &sb)

	if len(sb.Albums.Items) > 0 {
		albumID = sb.Albums.Items[0].ID
		albumURL = sb.Albums.Items[0].ExternalURLs["spotify"]

		if len(sb.Albums.Items[0].Images) > 0 {
			coverURL = sb.Albums.Items[0].Images[0].URL
		}
	}

	return
}

func getSpotifyAlbumLabel(albumID string) string {
	if albumID == "" {
		return ""
	}

	tok := getSpotifyToken()

	if tok == "" {
		return ""
	}

	u := spotifyAlbumBase + url.PathEscape(albumID)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	logrus.Debugf("REQ GET %s", u)

	resp, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var out struct {
		Label string `json:"label"`
	}

	b, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(b, &out)

	return strings.TrimSpace(out.Label)
}

func computeScore(followers int64, popularity int) int {
	l := int(math.Floor(math.Log1p(float64(followers))))

	if l > popularity {
		if l > 100 {
			return 100
		}

		return l
	}

	return popularity
}

func findYouTubePreview(artist, album string) string {
	key := os.Getenv("YOUTUBE_API_KEY")

	if key == "" {
		return ""
	}

	q := url.QueryEscape(artist + " " + album + " full album")
	u := youtubeSearchBase + "?part=snippet&maxResults=1&type=video&q=" + q + "&key=" + key
	req, _ := http.NewRequest("GET", u, nil)
	logrus.Debugf("REQ GET %s", u)

	resp, err := httpClient.Do(req)
	if err != nil {
		logrus.Warnf("YouTube search: %v", err)
		return ""
	}
	defer resp.Body.Close()

	var out struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
		} `json:"items"`
	}

	b, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(b, &out)

	if len(out.Items) == 0 {
		return ""
	}

	return youtubeWatchBase + out.Items[0].ID.VideoID
}

func lookupMetalArchivesBandGenres(artist, contact string) []string {
	ua := "metal-aggregator/1.0 (" + getenv("CONTACT_EMAIL", "admin@example.com") + ")"
	want := norm(artist)

	if g := maAdvancedJSONGenres(artist, true, ua, want); len(g) > 0 {
		return g
	}

	if g := maAdvancedJSONGenres(artist, false, ua, want); len(g) > 0 {
		return g
	}

	return maHTMLGenresFallback(artist, ua, want)
}

func maAdvancedJSONGenres(artist string, exact bool, ua, want string) []string {
	exactStr := "0"

	if exact {
		exactStr = "1"
	}

	u := maAdvancedSearch + "?bandName=" +
		url.QueryEscape(artist) + "&exactBandMatch=" + exactStr
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", ua)

	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return nil
	}
	defer resp.Body.Close()

	var payload struct {
		AaData [][]any `json:"aaData"`
	}

	b, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil
	}
	best := -1

	for i, row := range payload.AaData {
		if len(row) < 2 {
			continue
		}

		name := stripTags(fmt.Sprint(row[0]))

		if norm(name) == want {
			best = i
			break
		}
	}

	if best == -1 && len(payload.AaData) > 0 {
		for i, row := range payload.AaData {
			if len(row) < 2 {
				continue
			}

			name := norm(stripTags(fmt.Sprint(row[0])))
			ok := true

			for _, t := range strings.Split(want, " ") {
				if !strings.Contains(name, t) {
					ok = false
					break
				}
			}

			if ok {
				best = i
				break
			}
		}
	}

	if best >= 0 {
		genre := strings.TrimSpace(stripTags(fmt.Sprint(payload.AaData[best][1])))

		return parseMAGenres(genre)
	}

	return nil
}

func maHTMLGenresFallback(artist, ua, want string) []string {
	search := maSearchBase + "?type=band&searchString=" +
		url.QueryEscape(artist)
	req, _ := http.NewRequest("GET", search, nil)
	req.Header.Set("User-Agent", ua)

	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return nil
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	html := string(b)

	linkRe := regexp.MustCompile(`href="(/bands/[^"]+)"[^>]*>(.*?)</a>`)
	cands := linkRe.FindAllStringSubmatch(html, -1)
	best := ""

	for _, m := range cands {
		if len(m) < 3 {
			continue
		}

		if norm(htmlUnescape(m[2])) == norm(artist) {
			best = maBase + m[1]
			break
		}
	}

	if best == "" && len(cands) > 0 {
		for _, m := range cands {
			name := norm(htmlUnescape(m[2]))
			ok := true

			for _, t := range strings.Split(norm(artist), " ") {
				if !strings.Contains(name, t) {
					ok = false
					break
				}
			}

			if ok {
				best = maBase + m[1]
				break
			}
		}
	}

	if best == "" {
		return nil
	}

	req2, _ := http.NewRequest("GET", best, nil)
	req2.Header.Set("User-Agent", ua)

	resp2, err := httpClient.Do(req2)
	if err != nil || resp2.StatusCode != 200 {
		return nil
	}
	defer resp2.Body.Close()

	b2, _ := io.ReadAll(resp2.Body)
	page := string(b2)
	re := regexp.MustCompile(`(?is)<dt>\s*Genre:\s*</dt>\s*<dd>(.*?)</dd>`)

	if mm := re.FindStringSubmatch(page); len(mm) >= 2 {
		return parseMAGenres(strings.TrimSpace(htmlUnescape(mm[1])))
	}

	return nil
}

func lookupCountryFromMetalArchives(artist string) string {
	ua := "metal-aggregator/1.0 (" + getenv("CONTACT_EMAIL", defaultContactEmail) + ")"
	search := maSearchBase + "?type=band&searchString=" +
		url.QueryEscape(artist)
	logrus.Debugf("Metal Archives country search: %s", search)
	req, _ := http.NewRequest("GET", search, nil)
	req.Header.Set("User-Agent", ua)

	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		logrus.Debugf("Metal Archives search failed: err=%v, status=%d", err, resp.StatusCode)
		return ""
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	html := string(b)

	linkRe := regexp.MustCompile(`href="(/bands/[^"]+)"[^>]*>(.*?)</a>`)
	cands := linkRe.FindAllStringSubmatch(html, -1)
	logrus.Debugf("Metal Archives found %d candidate bands", len(cands))
	best := ""

	for _, m := range cands {
		if len(m) < 3 {
			continue
		}

		if norm(htmlUnescape(m[2])) == norm(artist) {
			best = maBase + m[1]
			break
		}
	}

	if best == "" && len(cands) > 0 {
		for _, m := range cands {
			name := norm(htmlUnescape(m[2]))
			ok := true

			for _, t := range strings.Split(norm(artist), " ") {
				if !strings.Contains(name, t) {
					ok = false
					break
				}
			}

			if ok {
				best = maBase + m[1]
				break
			}
		}
	}

	if best == "" {
		logrus.Debugf("No matching Metal Archives band found for %s", artist)
		return ""
	}

	logrus.Debugf("Fetching Metal Archives band page: %s", best)
	req2, _ := http.NewRequest("GET", best, nil)
	req2.Header.Set("User-Agent", ua)

	resp2, err := httpClient.Do(req2)
	if err != nil || resp2.StatusCode != 200 {
		logrus.Debugf("Metal Archives band page fetch failed: err=%v, status=%d",
			err, resp2.StatusCode)
		return ""
	}
	defer resp2.Body.Close()

	b2, _ := io.ReadAll(resp2.Body)
	page := string(b2)

	countryRe := regexp.MustCompile(`(?is)<dt>\s*Country of origin:\s*</dt>\s*<dd>(.*?)</dd>`)

	if mm := countryRe.FindStringSubmatch(page); len(mm) >= 2 {
		countryHTML := mm[1]
		countryName := strings.TrimSpace(stripTags(countryHTML))
		countryName = htmlUnescape(countryName)

		if countryName != "" {
			isoCode := countryNameToISO(countryName)
			logrus.Debugf("Metal Archives country: %s -> %s", countryName, isoCode)
			return isoCode
		}
	}

	logrus.Debugf("Country not found in Metal Archives page")
	return ""
}

func parseMAGenres(s string) []string {
	if s == "" {
		return nil
	}

	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " / ", "/")

	for _, sep := range []string{"/", ",", ";"} {
		s = strings.ReplaceAll(s, sep, "|")
	}

	parts := strings.Split(s, "|")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}

	for _, p := range parts {
		p = strings.TrimSpace(p)

		if p == "" || seen[p] {
			continue
		}

		seen[p] = true
		out = append(out, p)
	}

	return out
}

func resolveLabelInfo(artist, album, labelHint, contact string) (string, string, string) {
	tok := os.Getenv("DISCOGS_TOKEN")
	if tok == "" {
		logrus.Warnf("DISCOGS_TOKEN not set; cannot resolve label links")
		return "", "", ""
	}

	name, dlink, site := resolveFromDiscogsRelease(artist, album, tok, contact)

	if dlink != "" || site != "" {
		if name == "" {
			name = strings.TrimSpace(labelHint)
		}

		return dlink, site, name
	}

	q := labelHint

	if strings.TrimSpace(q) == "" {
		q = artist + " " + album
	}

	return resolveFromDiscogsLabelSearch(q, tok, contact)
}

func resolveFromDiscogsRelease(artist, album, tok, contact string) (labelName,
	discogsLink, website string) {
	q := url.QueryEscape(artist + " " + album)
	u := discogsSearchBase + "?q=" + q +
		"&type=release&per_page=1&token=" + tok
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "metal-aggregator/1.0 ("+contact+")")
	logrus.Debugf("REQ GET %s", u)

	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var sr struct {
		Results []struct {
			URI         string   `json:"uri"`
			ResourceURL string   `json:"resource_url"`
			Label       []string `json:"label"`
			Title       string   `json:"title"`
		} `json:"results"`
	}

	b, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(b, &sr)

	if len(sr.Results) == 0 {
		return
	}

	if len(sr.Results[0].Label) > 0 {
		labelName = strings.TrimSpace(sr.Results[0].Label[0])
	}

	if sr.Results[0].URI != "" {
		discogsLink = sr.Results[0].URI

		if strings.HasPrefix(discogsLink, "/") {
			discogsLink = discogsBase + discogsLink
		}
	}

	if sr.Results[0].ResourceURL != "" {
		rr := sr.Results[0].ResourceURL + "?token=" + tok
		req2, _ := http.NewRequest("GET", rr, nil)
		req2.Header.Set("User-Agent", "metal-aggregator/1.0 ("+contact+")")
		logrus.Debugf("REQ GET %s", rr)

		resp2, err := httpClient.Do(req2)
		if err == nil {
			defer resp2.Body.Close()
			var rel struct {
				Labels []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
					URI  string `json:"uri"`
				} `json:"labels"`
			}

			b2, _ := io.ReadAll(resp2.Body)
			_ = json.Unmarshal(b2, &rel)

			if len(rel.Labels) > 0 {
				lid := rel.Labels[0].ID

				if labelName == "" {
					labelName = strings.TrimSpace(rel.Labels[0].Name)
				}

				ll := fmt.Sprintf("%s/%d?token=%s", discogsLabelsBase, lid, tok)
				req3, _ := http.NewRequest("GET", ll, nil)
				req3.Header.Set("User-Agent", "metal-aggregator/1.0 ("+contact+")")
				logrus.Debugf("REQ GET %s", ll)

				resp3, err := httpClient.Do(req3)
				if err == nil {
					defer resp3.Body.Close()

					var ld struct {
						URLs []string `json:"urls"`
						URI  string   `json:"uri"`
						Name string   `json:"name"`
					}

					b3, _ := io.ReadAll(resp3.Body)
					_ = json.Unmarshal(b3, &ld)

					if discogsLink == "" && ld.URI != "" {
						discogsLink = ld.URI

						if strings.HasPrefix(discogsLink, "/") {
							discogsLink = "https://www.discogs.com" + discogsLink
						}
					}

					if website == "" {
						website = pickOfficialWebsite(ld.URLs)
					}

					if labelName == "" && ld.Name != "" {
						labelName = strings.TrimSpace(ld.Name)
					}
				}
			}
		}
	}
	return
}

func resolveFromDiscogsLabelSearch(query, tok, contact string) (discogsLink,
	website, labelName string) {
	q := url.QueryEscape(query)
	u := discogsSearchBase + "?q=" + q +
		"&type=label&per_page=1&token=" + tok
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "metal-aggregator/1.0 ("+contact+")")
	logrus.Debugf("REQ GET %s", u)

	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var search struct {
		Results []struct {
			ID    int    `json:"id"`
			URI   string `json:"uri"`
			Title string `json:"title"`
		} `json:"results"`
	}

	b, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(b, &search)

	if len(search.Results) == 0 {
		return
	}

	id := search.Results[0].ID
	labelName = strings.TrimSpace(search.Results[0].Title)
	discogsLink = search.Results[0].URI

	if strings.HasPrefix(discogsLink, "/") {
		discogsLink = "https://www.discogs.com" + discogsLink
	}

	ll := fmt.Sprintf("%s/%d?token=%s", discogsLabelsBase, id, tok)
	req2, _ := http.NewRequest("GET", ll, nil)
	req2.Header.Set("User-Agent", "metal-aggregator/1.0 ("+contact+")")
	logrus.Debugf("REQ GET %s", ll)

	resp2, err := httpClient.Do(req2)
	if err != nil {
		return
	}
	defer resp2.Body.Close()

	var info struct {
		URLs []string `json:"urls"`
	}

	b2, _ := io.ReadAll(resp2.Body)
	_ = json.Unmarshal(b2, &info)
	website = pickOfficialWebsite(info.URLs)

	return
}

func pickOfficialWebsite(urls []string) string {
	if len(urls) == 0 {
		return ""
	}

	urlRx := regexp.MustCompile(`https?://[^\s'"<>)\]]+`)
	candsMap := make(map[string]struct{})

	for _, raw := range urls {
		if strings.Contains(raw, "#Not_On_Label") {
			logrus.Debugf("Skipping #Not_On_Label URL: %s", raw)
			return ""
		}

		raw = strings.TrimSpace(raw)

		matches := urlRx.FindAllString(raw, -1)

		if len(matches) == 0 && raw != "" && !strings.Contains(raw, "://") {
			matches = []string{"http://" + raw}
		}

		for _, u := range matches {
			u = strings.TrimRight(u, ".,);]")

			if strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") {
				candsMap[u] = struct{}{}
			}
		}
	}

	cands := make([]string, 0, len(candsMap))

	for u := range candsMap {
		cands = append(cands, u)
	}

	sort.SliceStable(cands, func(i, j int) bool {
		if len(cands[i]) == len(cands[j]) {
			return cands[i] < cands[j]
		}

		return len(cands[i]) < len(cands[j])
	})

	bad := []string{
		"facebook.com", "instagram.com", "twitter.com", "x.com",
		"bandcamp.com", "soundcloud.com", "youtube.com", "tiktok.com",
		"linktr.ee",
	}

	isBad := func(u string) bool {
		lu := strings.ToLower(u)

		for _, h := range bad {
			if strings.Contains(lu, h) {
				return true
			}
		}

		return false
	}

	for _, u := range cands {
		if !isBad(u) {
			if validURL := normalizeURL(u); validURL != "" {
				logrus.Debugf("Selected website URL: %s", validURL)
				return validURL
			}
		}
	}

	if len(cands) > 0 {
		if validURL := normalizeURL(cands[0]); validURL != "" {
			logrus.Debugf("Selected fallback website URL: %s", validURL)
			return validURL
		}
	}

	return ""
}

func normalizeURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	rawURL = strings.TrimSpace(rawURL)

	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return ""
	}

	urlRx := regexp.MustCompile(`https?://`)

	protocolMatches := urlRx.FindAllStringIndex(rawURL, -1)

	if len(protocolMatches) == 0 {
		return ""
	}

	var firstURL string

	if len(protocolMatches) > 1 {
		logrus.Debugf("Detected %d concatenated URLs in: %s", len(protocolMatches), rawURL)
		endIdx := protocolMatches[1][0]
		firstURL = rawURL[:endIdx]
		logrus.Debugf("Using first URL: %s", firstURL)
	} else {
		firstURL = rawURL
	}

	parsed, err := url.Parse(firstURL)
	if err != nil {
		logrus.Debugf("Invalid URL format: %s, error: %v", firstURL, err)
		return ""
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		logrus.Debugf("Invalid URL (missing scheme/host): %s", firstURL)
		return ""
	}

	normalized := parsed.Scheme + "://" + parsed.Host + parsed.Path

	if parsed.RawQuery != "" {
		normalized += "?" + parsed.RawQuery
	}

	if normalized != firstURL {
		logrus.Debugf("Normalized URL: %s -> %s", firstURL, normalized)
	}

	return normalized
}

func lookupDiscogsStyles(artist, album, contact string) []string {
	tok := os.Getenv("DISCOGS_TOKEN")

	if tok == "" {
		return nil
	}

	q := url.QueryEscape(artist + " " + album)
	u := discogsSearchBase + "?q=" + q +
		"&type=release&per_page=1&token=" + tok
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "metal-aggregator/1.0 ("+contact+")")
	logrus.Debugf("REQ GET %s", u)

	resp, err := httpClient.Do(req)
	if err != nil {
		logrus.Warnf("Discogs style: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var out struct {
		Results []struct {
			Style []string `json:"style"`
		} `json:"results"`
	}

	b, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(b, &out)

	if len(out.Results) == 0 {
		return nil
	}

	return normalizeList(out.Results[0].Style)
}

func lookupCountryFromMusicBrainz(artist, contact string) string {
	ua := "metal-aggregator/1.0 (" + contact + ")"

	searchURL := musicBrainzBase + "/artist/?query=artist:" +
		url.QueryEscape(artist) + "&fmt=json&limit=1"
	logrus.Debugf("MusicBrainz artist search: %s", searchURL)

	req, _ := http.NewRequest("GET", searchURL, nil)
	req.Header.Set("User-Agent", ua)

	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		logrus.Debugf("MusicBrainz search failed: err=%v, status=%d", err, resp.StatusCode)
		return ""
	}
	defer resp.Body.Close()

	var searchResp struct {
		Artists []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"artists"`
	}

	b, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(b, &searchResp); err != nil {
		logrus.Debugf("MusicBrainz search parse failed: %v", err)
		return ""
	}

	if len(searchResp.Artists) == 0 {
		logrus.Debugf("No MusicBrainz artist found for %s", artist)
		return ""
	}

	mbid := searchResp.Artists[0].ID
	logrus.Debugf("MusicBrainz found artist: %s (MBID: %s)", searchResp.Artists[0].Name, mbid)

	artistURL := musicBrainzBase + "/artist/" + mbid + "?fmt=json&inc=area-rels"
	logrus.Debugf("Fetching MusicBrainz artist details: %s", artistURL)

	req2, _ := http.NewRequest("GET", artistURL, nil)
	req2.Header.Set("User-Agent", ua)

	resp2, err := httpClient.Do(req2)
	if err != nil || resp2.StatusCode != 200 {
		logrus.Debugf("MusicBrainz artist fetch failed: err=%v, status=%d",
			err, resp2.StatusCode)
		return ""
	}
	defer resp2.Body.Close()

	var artistResp struct {
		Area struct {
			Name          string   `json:"name"`
			ISO31661Codes []string `json:"iso-3166-1-codes"`
		} `json:"area"`
	}

	b2, _ := io.ReadAll(resp2.Body)
	if err := json.Unmarshal(b2, &artistResp); err != nil {
		logrus.Debugf("MusicBrainz artist parse failed: %v", err)
		return ""
	}

	if len(artistResp.Area.ISO31661Codes) > 0 {
		isoCode := strings.ToUpper(artistResp.Area.ISO31661Codes[0])
		logrus.Debugf("MusicBrainz country: %s -> %s",
			artistResp.Area.Name, isoCode)
		return isoCode
	}

	if artistResp.Area.Name != "" {
		isoCode := countryNameToISO(artistResp.Area.Name)
		if isoCode != "" {
			logrus.Debugf("MusicBrainz country (mapped): %s -> %s",
				artistResp.Area.Name, isoCode)
			return isoCode
		}
	}

	logrus.Debugf("MusicBrainz artist has no area/country information")
	return ""
}

func lookupCountryFromDiscogsArtist(artist, contact string) string {
	tok := os.Getenv("DISCOGS_TOKEN")

	if tok == "" {
		logrus.Debugf("DISCOGS_TOKEN not set, skipping Discogs artist country lookup")
		return ""
	}

	q := url.QueryEscape(artist)
	u := discogsSearchBase + "?q=" + q + "&type=artist&per_page=1&token=" + tok
	logrus.Debugf("Discogs artist search: %s", u)

	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "metal-aggregator/1.0 ("+contact+")")

	resp, err := httpClient.Do(req)
	if err != nil {
		logrus.Debugf("Discogs artist search failed: %v", err)
		return ""
	}
	defer resp.Body.Close()

	var sr struct {
		Results []struct {
			ID          int    `json:"id"`
			ResourceURL string `json:"resource_url"`
		} `json:"results"`
	}

	b, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(b, &sr)

	if len(sr.Results) == 0 {
		logrus.Debugf("No Discogs artist found for %s", artist)
		return ""
	}

	artistID := sr.Results[0].ID
	artistURL := fmt.Sprintf("%s/%d?token=%s", discogsArtistBase, artistID, tok)
	logrus.Debugf("Fetching Discogs artist: %s", artistURL)

	req2, _ := http.NewRequest("GET", artistURL, nil)
	req2.Header.Set("User-Agent", "metal-aggregator/1.0 ("+contact+")")

	resp2, err := httpClient.Do(req2)
	if err != nil {
		logrus.Debugf("Discogs artist fetch failed: %v", err)
		return ""
	}
	defer resp2.Body.Close()

	var artistResp struct {
		Profile string `json:"profile"`
	}

	b2, _ := io.ReadAll(resp2.Body)
	_ = json.Unmarshal(b2, &artistResp)

	if artistResp.Profile != "" {
		countryRe := regexp.MustCompile(`(?i)(?:Country|Origin|from|based in)[\s:]+([A-Za-z\s]+)`)
		if mm := countryRe.FindStringSubmatch(artistResp.Profile); len(mm) >= 2 {
			countryName := strings.TrimSpace(mm[1])

			isoCode := countryNameToISO(countryName)
			if isoCode != "" {
				logrus.Debugf("Discogs artist profile country: %s -> %s",
					countryName, isoCode)
				return isoCode
			}
		}
		logrus.Debugf("Country pattern not found in Discogs artist profile")
	} else {
		logrus.Debugf("Discogs artist profile is empty")
	}

	return ""
}

func countryNameToISO(countryName string) string {
	if countryName == "" {
		return ""
	}

	countryName = strings.TrimSpace(countryName)
	originalName := countryName

	countryMap := map[string]string{
		"united states":            "US",
		"united states of america": "US",
		"usa":                      "US",
		"united kingdom":           "GB",
		"uk":                       "GB",
		"great britain":            "GB",
		"germany":                  "DE",
		"sweden":                   "SE",
		"norway":                   "NO",
		"finland":                  "FI",
		"denmark":                  "DK",
		"france":                   "FR",
		"italy":                    "IT",
		"spain":                    "ES",
		"portugal":                 "PT",
		"netherlands":              "NL",
		"belgium":                  "BE",
		"switzerland":              "CH",
		"austria":                  "AT",
		"poland":                   "PL",
		"czech republic":           "CZ",
		"czechia":                  "CZ",
		"russia":                   "RU",
		"greece":                   "GR",
		"turkey":                   "TR",
		"japan":                    "JP",
		"china":                    "CN",
		"south korea":              "KR",
		"australia":                "AU",
		"new zealand":              "NZ",
		"canada":                   "CA",
		"mexico":                   "MX",
		"brazil":                   "BR",
		"argentina":                "AR",
		"chile":                    "CL",
		"south africa":             "ZA",
		"israel":                   "IL",
		"india":                    "IN",
		"indonesia":                "ID",
		"thailand":                 "TH",
		"philippines":              "PH",
		"ireland":                  "IE",
		"iceland":                  "IS",
		"estonia":                  "EE",
		"latvia":                   "LV",
		"lithuania":                "LT",
		"ukraine":                  "UA",
		"belarus":                  "BY",
		"romania":                  "RO",
		"bulgaria":                 "BG",
		"croatia":                  "HR",
		"serbia":                   "RS",
		"slovenia":                 "SI",
		"slovakia":                 "SK",
		"hungary":                  "HU",
	}

	lower := strings.ToLower(countryName)

	if code, ok := countryMap[lower]; ok {
		logrus.Debugf("Country mapping: %s -> %s", originalName, code)
		return code
	}

	if len(countryName) == 2 {
		upper := strings.ToUpper(countryName)
		logrus.Debugf("Country already ISO code: %s -> %s", originalName, upper)
		return upper
	}

	logrus.Debugf("Country name not in mapping: %s (returning empty)", originalName)
	return ""
}

func norm(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer(
		"'", "'", "'", "'", `"`, `"`, `"`, `"`,
		"–", "-", "—", "-", "&", " and ",
		"é", "e", "è", "e", "á", "a", "à", "a", "ó", "o", "ö", "o",
		"ü", "u", "í", "i", "ï", "i", "ç", "c",
	).Replace(s)
	s = strings.TrimPrefix(s, "the ")
	buf := make([]rune, 0, len(s))

	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' {
			buf = append(buf, r)
		}
	}

	spaceRe := regexp.MustCompile(`\s+`)

	return strings.TrimSpace(spaceRe.ReplaceAllString(string(buf), " "))
}

func stripTags(s string) string {
	return regexp.MustCompile(`(?s)<[^>]*>`).ReplaceAllString(s, "")
}

func htmlUnescape(s string) string {
	r := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">",
		"&quot;", `"`, "&#39;", "'")
	return r.Replace(s)
}

func normalizeList(in []string) []string {
	out, seen := make([]string, 0, len(in)), map[string]bool{}

	for _, s := range in {
		s = strings.ToLower(strings.TrimSpace(s))

		if s == "" || seen[s] {
			continue
		}

		seen[s] = true
		out = append(out, s)
	}

	return out
}

func unionPreserve(a, b, c []string) []string {
	seen := map[string]bool{}
	out := []string{}

	for _, list := range [][]string{a, b, c} {
		for _, s := range list {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}

	return out
}
