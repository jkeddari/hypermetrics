-- +goose Up
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

-- +goose Down
UPDATE users SET password_hash = '' WHERE password_hash IS NULL;
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
