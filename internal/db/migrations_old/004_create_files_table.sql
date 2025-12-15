-- +goose Up
CREATE TABLE IF NOT EXISTS files (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL,
    filename TEXT NOT NULL,
    original_name TEXT,
    mime_type TEXT,
    size INTEGER,
    storage_provider TEXT DEFAULT 'local',
    storage_path TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Ensure one file per type per user (e.g., only one avatar)
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_file_type ON files(user_id, type) WHERE type IN ('avatar');

-- Index for faster user file lookups
CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_files_user_id;
DROP INDEX IF EXISTS idx_user_file_type;
DROP TABLE IF EXISTS files;
