-- +goose Up
CREATE TABLE IF NOT EXISTS goals (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    current_repetition INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Index for user lookups
CREATE INDEX IF NOT EXISTS idx_goals_user_id ON goals(user_id);

-- Index for status queries
CREATE INDEX IF NOT EXISTS idx_goals_status ON goals(status);

-- Index for combined user + status queries
CREATE INDEX IF NOT EXISTS idx_goals_user_status ON goals(user_id, status);

-- +goose Down
DROP INDEX IF EXISTS idx_goals_user_status;
DROP INDEX IF EXISTS idx_goals_status;
DROP INDEX IF EXISTS idx_goals_user_id;
DROP TABLE IF EXISTS goals;
