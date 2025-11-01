# 001_initial_schema

Creates the initial database schema for the blastbeat API.

## Tables

- **releases** - Stores metal music releases with metadata (artist, title,
  genres, dates, follower counts, etc.)
- **genres** - Stores genre definitions with unique name and slug

## Indexes

- `idx_releases_release_date` - Index on release date for fast date range
  queries
- `idx_releases_follower_count` - Index on follower count for follower
  range filtering
- `idx_releases_artist` - Index on artist name for artist searches
