CREATE TABLE releases (
  id UUID PRIMARY KEY,
  title TEXT NOT NULL,
  artist TEXT NOT NULL,
  album_art_url TEXT NOT NULL,
  release_date DATE NOT NULL,
  label TEXT NOT NULL,
  label_url TEXT,
  follower_count INTEGER NOT NULL DEFAULT 0,
  genres JSONB NOT NULL, -- keep as JSON array
  country CHAR(2),
  external_links JSONB NOT NULL,
  spotify_url TEXT,
  youtube_url TEXT,
  bandcamp_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_releases_release_date ON releases (release_date);
CREATE INDEX idx_releases_follower_count ON releases (follower_count);
CREATE INDEX idx_releases_artist ON releases (artist);

CREATE TABLE genres (
  id UUID PRIMARY KEY,
  name TEXT UNIQUE NOT NULL,
  slug TEXT UNIQUE NOT NULL
);
