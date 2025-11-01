# Import Releases

This script enriches release data from CSV files and imports them into the
database. It fetches additional metadata from external APIs (Spotify, YouTube,
Metal Archives, Discogs) and stores enriched release information.

## Purpose

The import script is the standard way to add new releases to the database. It
takes a simple CSV input (date, artist, album, optional label) and enriches
each release with:

- Spotify follower counts and popularity
- Spotify album URLs and cover art
- YouTube preview URLs
- Genre information from multiple sources (Spotify, Metal Archives, Discogs)
- Label information and official websites
- External links and metadata

## When to Use

Use this script when you have new releases to add to the database:

- After scraping release data from sources like Metal Archives
- When manually curating a list of releases
- When importing releases from external datasets
- When adding bulk releases in CSV format

## CSV Format

The input CSV must have exactly 4 columns:

```csv
YYYY-MM-DD,Artist,Album,Label
2024-01-15,Metallica,Master of Puppets,Elektra
2024-02-20,Slayer,Reign in Blood,Def Jam
```

Columns:

1. **Date** - Release date in `YYYY-MM-DD` format (required)
2. **Artist** - Artist name (required)
3. **Album** - Album title (required)
4. **Label** - Record label name (optional, will be fetched if missing)

## Environment Variables

### Required

- `SPOTIFY_CLIENT_ID` - Spotify API client ID
- `SPOTIFY_CLIENT_SECRET` - Spotify API client secret

### Optional

- `YOUTUBE_API_KEY` - YouTube Data API key (enables YouTube preview URLs)
- `DISCOGS_TOKEN` - Discogs API token (enables Discogs label/website lookups)
- `CONTACT_EMAIL` - Contact email for API user agents (default: admin@example.com)
- `LOG_LEVEL` - Logging level: `debug`, `info`, `warn`, `error` (default: `info`)

The script will error and exit if required environment variables are not set.

## Usage

### Dry Run (Default)

By default, the script runs in dry-run mode and will not write to the database.
It will show you what would be inserted:

```bash
go run cmd/import-releases/main.go -in assets/bb-etl/releases.csv
```

Or using Make:

```bash
make import/releases-dry IN=assets/bb-etl/releases.csv
```

### Write to Database

To actually write releases to the database, use the `--enable-write` flag:

```bash
go run cmd/import-releases/main.go -in assets/bb-etl/releases.csv --enable-write
```

Or using Make:

```bash
make import/releases IN=assets/bb-etl/releases.csv ENABLE_WRITE=1
```

## How It Works

1. **Reads CSV** - Parses input CSV file with release data
2. **Deduplicates** - Skips duplicate releases based on date/artist/album
3. **Enriches Each Release**:
   - Searches Spotify for artist/album data
   - Fetches follower counts, popularity, cover art
   - Searches YouTube for preview videos (if API key provided)
   - Looks up genres from Metal Archives
   - Looks up genres/styles from Discogs (if token provided)
   - Resolves label information and official websites
4. **Validates** - Checks all required fields are present
5. **Writes to Database** - Inserts releases using generated SQL methods (if `--enable-write` is set)

The script uses the generated SQL insert methods from `backends/gensql`,
ensuring type safety and consistency with the database schema.

## Output

The script outputs:

- Progress information for each release processed
- Enrichment sources used (Spotify, YouTube, Metal Archives, Discogs)
- Summary statistics: processed, successful, skipped, errors
- In dry-run mode: JSON representation of what would be inserted
