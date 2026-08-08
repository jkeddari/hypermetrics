-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS key_hash TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash) WHERE key_hash IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_api_keys_key_hash;
ALTER TABLE api_keys DROP COLUMN IF EXISTS key_hash;
DROP INDEX IF EXISTS idx_users_email;
DROP TABLE IF EXISTS users;
