-- +goose Up
ALTER TABLE users ADD COLUMN pending_email TEXT;

-- +goose Down
ALTER TABLE users DROP COLUMN pending_email;
