-- +goose Up
-- SQLite doesn't support DROP COLUMN directly, so we need to recreate the table
CREATE TABLE files_new (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL,
    filename TEXT NOT NULL,
    original_name TEXT,
    mime_type TEXT,
    size INTEGER,
    storage_path TEXT NOT NULL,
    public BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Copy data from old table to new table (excluding storage_provider)
INSERT INTO files_new (id, user_id, type, filename, original_name, mime_type, size, storage_path, public, created_at)
SELECT id, user_id, type, filename, original_name, mime_type, size, storage_path, public, created_at
FROM files;

-- Drop old table
DROP TABLE files;

-- Rename new table to original name
ALTER TABLE files_new RENAME TO files;

-- Recreate indexes
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_file_type ON files(user_id, type) WHERE type IN ('avatar');
CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);

-- +goose Down
-- Recreate table with storage_provider
CREATE TABLE files_new (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL,
    filename TEXT NOT NULL,
    original_name TEXT,
    mime_type TEXT,
    size INTEGER,
    storage_provider TEXT DEFAULT 'local',
    storage_path TEXT NOT NULL,
    public BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

INSERT INTO files_new (id, user_id, type, filename, original_name, mime_type, size, storage_provider, storage_path, public, created_at)
SELECT id, user_id, type, filename, original_name, mime_type, size, 'local', storage_path, public, created_at
FROM files;

DROP TABLE files;
ALTER TABLE files_new RENAME TO files;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_file_type ON files(user_id, type) WHERE type IN ('avatar');
CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);
