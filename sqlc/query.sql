-- name: GetRelease :one
SELECT *
FROM releases
WHERE id = $1
LIMIT 1;

-- name: ListReleases :many
SELECT *
FROM releases
ORDER BY release_date DESC, created_at DESC;

-- name: ListReleasesByDateRange :many
SELECT *
FROM releases
WHERE release_date BETWEEN $1 AND $2
ORDER BY release_date DESC, created_at DESC;

-- name: ListReleasesByExactDate :many
SELECT *
FROM releases
WHERE release_date = $1
ORDER BY created_at DESC;

-- name: ListReleasesByArtist :many
SELECT *
FROM releases
WHERE artist LIKE '%' || $1 || '%'
ORDER BY release_date DESC, created_at DESC;

-- name: SearchReleases :many
SELECT *
FROM releases
WHERE artist LIKE '%' || $1 || '%'
   OR title LIKE '%' || $1 || '%'
ORDER BY release_date DESC, created_at DESC;

-- name: ListReleasesByGenre :many
SELECT r.*
FROM releases AS r
WHERE EXISTS (
  SELECT 1
  FROM jsonb_array_elements_text(r.genres) AS genre
  WHERE LOWER(genre) = LOWER($1)
)
ORDER BY r.release_date DESC, r.created_at DESC;

-- name: ListReleasesByGenresAny :many
SELECT r.*
FROM releases r
WHERE EXISTS (
  SELECT 1
  FROM jsonb_array_elements_text(r.genres) g(genre)
  WHERE g.genre = ANY($1::text[])
)
ORDER BY r.release_date DESC, r.created_at DESC;

-- name: ListReleasesByFollowerRange :many
SELECT *
FROM releases
WHERE follower_count BETWEEN $1 AND $2
ORDER BY follower_count DESC, release_date DESC;

-- name: CreateRelease :one
INSERT INTO releases (
  id,
  title,
  artist,
  album_art_url,
  release_date,
  label,
  label_url,
  follower_count,
  genres,
  country,
  external_links,
  spotify_url,
  youtube_url,
  bandcamp_url
) VALUES (
  $1,  -- id
  $2,  -- title
  $3,  -- artist
  $4,  -- album_art_url
  $5,  -- release_date
  $6,  -- label
  $7,  -- label_url
  $8,  -- follower_count
  $9,  -- genres (jsonb)
  $10, -- country
  $11, -- external_links (jsonb)
  $12, -- spotify_url
  $13, -- youtube_url
  $14  -- bandcamp_url
)
RETURNING *;

-- name: UpdateRelease :one
UPDATE releases
SET
  title = $2,
  artist = $3,
  album_art_url = $4,
  release_date = $5,
  label = $6,
  label_url = $7,
  follower_count = $8,
  genres = $9,
  country = $10,
  external_links = $11,
  spotify_url = $12,
  youtube_url = $13,
  bandcamp_url = $14,
  updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteRelease :exec
DELETE FROM releases
WHERE id = $1;



-- name: GetGenre :one
SELECT *
FROM genres
WHERE id = $1
LIMIT 1;

-- name: GetGenreBySlug :one
SELECT *
FROM genres
WHERE slug = $1
LIMIT 1;

-- name: ListGenres :many
SELECT *
FROM genres
ORDER BY name;

-- name: CreateGenre :one
INSERT INTO genres (
  id,
  name,
  slug
) VALUES (
  $1,
  $2,
  $3
)
RETURNING *;

-- name: UpdateGenre :one
UPDATE genres
SET
  name = $2,
  slug = $3
WHERE id = $1
RETURNING *;

-- name: DeleteGenre :exec
DELETE FROM genres
WHERE id = $1;
