-- +goose Up
ALTER TABLE files ADD COLUMN public BOOLEAN DEFAULT true;

-- +goose Down
ALTER TABLE files DROP COLUMN public;
