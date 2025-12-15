-- +goose Up
CREATE TABLE IF NOT EXISTS tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL,
    token TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Index for token lookups
CREATE INDEX IF NOT EXISTS idx_tokens_token ON tokens(token);

-- Index for cleanup of expired tokens
CREATE INDEX IF NOT EXISTS idx_tokens_expires_at ON tokens(expires_at);

-- Index for user's tokens
CREATE INDEX IF NOT EXISTS idx_tokens_user_id ON tokens(user_id);

-- +goose Down
DROP TABLE IF EXISTS tokens;
