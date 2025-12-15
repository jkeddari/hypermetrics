-- +goose Up
ALTER TABLE goals RENAME COLUMN current_repetition TO current_step;

-- +goose Down
ALTER TABLE goals RENAME COLUMN current_step TO current_repetition;
