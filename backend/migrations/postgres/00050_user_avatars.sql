-- +goose Up
-- User profile pictures. The bytes live in their OWN table, not a column on
-- `users`: GORM reads users with SELECT * (login, Me, ListUsers, ResolveUsers,
-- the auth interceptor), so a BYTEA on `users` would ride along on every one of
-- those reads. Keeping it separate means the avatar is fetched only by the one
-- RPC that wants it.
--
-- TWO renditions per upload (the two-rendition HARD RULE in CLAUDE.md):
--   image_data — the ORIGINAL, bounded on the client (not the raw camera file)
--   thumb_data — a small square THUMB that every avatar / list / fast-load
--                surface reads instead, so a table of 25 users pulls ~25 x a
--                few KB rather than ~25 x a few hundred KB.
-- thumb_data is NOT NULL: a row without a thumbnail would silently push those
-- surfaces back onto the original, which is the whole thing this prevents.
--
-- ON DELETE CASCADE because an avatar has no meaning without its user. Users are
-- soft-deleted (active = false) today, so this only fires on a real hard delete.
CREATE TABLE user_avatars (
  user_id      UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  content_type TEXT  NOT NULL,
  image_data   BYTEA NOT NULL,
  thumb_data   BYTEA NOT NULL,
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Denormalized "has an avatar, and how fresh" marker on the user row. NULL = no
-- avatar. This is what lets ListUsers / Me tell the UI whether to fetch bytes at
-- all, and doubles as the cache-buster in the frontend query key so a re-upload
-- is picked up without a manual refresh. Kept in sync by the avatar RPCs.
ALTER TABLE users ADD COLUMN avatar_updated_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS avatar_updated_at;
DROP TABLE IF EXISTS user_avatars;
