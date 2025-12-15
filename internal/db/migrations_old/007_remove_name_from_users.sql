-- +goose Up
-- Remove name column from users table (now in profiles)
ALTER TABLE users DROP COLUMN name;

-- +goose Down
-- Restore name column to users table
ALTER TABLE users ADD COLUMN name TEXT;

-- Restore name data from profiles
UPDATE users
SET name = (
    SELECT name
    FROM profiles
    WHERE profiles.user_id = users.id
);
