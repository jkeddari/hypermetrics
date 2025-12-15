-- +goose Up
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMP;

-- +goose Down
ALTER TABLE users DROP COLUMN email_verified_at;
