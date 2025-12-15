-- +goose Up
-- Make password optional for passwordless authentication
-- Users can now authenticate via magic link or OAuth without a password

-- Create new table with nullable password_hash
CREATE TABLE users_new (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT,
    pending_email TEXT,
    email_verified_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Copy data from old table
INSERT INTO users_new (id, email, password_hash, pending_email, email_verified_at, created_at)
SELECT id, email, password_hash, pending_email, email_verified_at, created_at FROM users;

-- Drop old table (CASCADE for PostgreSQL to handle foreign keys)
DROP TABLE users CASCADE;

-- Rename new table
ALTER TABLE users_new RENAME TO users;

-- Recreate foreign key constraints (PostgreSQL needs them re-added after CASCADE)
-- SQLite will ignore these if they don't apply

-- tokens table
ALTER TABLE tokens DROP CONSTRAINT IF EXISTS tokens_user_id_fkey;
ALTER TABLE tokens ADD CONSTRAINT tokens_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- files table
ALTER TABLE files DROP CONSTRAINT IF EXISTS files_user_id_fkey;
ALTER TABLE files ADD CONSTRAINT files_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- profiles table
ALTER TABLE profiles DROP CONSTRAINT IF EXISTS profiles_user_id_fkey;
ALTER TABLE profiles ADD CONSTRAINT profiles_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- subscriptions table
ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subscriptions_user_id_fkey;
ALTER TABLE subscriptions ADD CONSTRAINT subscriptions_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- goals table
ALTER TABLE goals DROP CONSTRAINT IF EXISTS goals_user_id_fkey;
ALTER TABLE goals ADD CONSTRAINT goals_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- goal_entries table
ALTER TABLE goal_entries DROP CONSTRAINT IF EXISTS goal_entries_user_id_fkey;
ALTER TABLE goal_entries ADD CONSTRAINT goal_entries_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- Add index for users without passwords (for analytics/filtering)
CREATE INDEX IF NOT EXISTS idx_users_passwordless ON users(id) WHERE password_hash IS NULL;

-- +goose Down
-- Recreate table with NOT NULL constraint
CREATE TABLE users_new (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    pending_email TEXT,
    email_verified_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Copy data (only users with passwords)
INSERT INTO users_new (id, email, password_hash, pending_email, email_verified_at, created_at)
SELECT id, email, password_hash, pending_email, email_verified_at, created_at FROM users WHERE password_hash IS NOT NULL;

DROP TABLE users;
ALTER TABLE users_new RENAME TO users;
DROP INDEX IF EXISTS idx_users_passwordless;
