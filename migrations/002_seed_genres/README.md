# 002_seed_genres

Seeds the genres table with 90+ metal subgenres.

This migration populates the genres table with common metal subgenres
including Death Metal, Black Metal, Thrash Metal, Doom Metal, and many
more variations and combinations.

Genres are inserted with unique slugs and names. The migration uses
`ON CONFLICT (name) DO NOTHING` to allow safe re-runs.
