CREATE TABLE IF NOT EXISTS releases (
  id UUID PRIMARY KEY,
  title TEXT NOT NULL,
  artist TEXT NOT NULL,
  album_art_url TEXT NOT NULL,
  release_date DATE NOT NULL,
  label TEXT NOT NULL,
  label_url TEXT,
  follower_count INTEGER NOT NULL DEFAULT 0,
  genres JSONB NOT NULL,
  country CHAR(2),
  external_links JSONB NOT NULL,
  spotify_url TEXT,
  youtube_url TEXT,
  bandcamp_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_releases_release_date ON releases (release_date);
CREATE INDEX IF NOT EXISTS idx_releases_follower_count ON releases (follower_count);
CREATE INDEX IF NOT EXISTS idx_releases_artist ON releases (artist);

CREATE TABLE IF NOT EXISTS genres (
  id UUID PRIMARY KEY,
  name TEXT UNIQUE NOT NULL,
  slug TEXT UNIQUE NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_releases_updated_at
  BEFORE UPDATE ON releases
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_genres_updated_at
  BEFORE UPDATE ON genres
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();

