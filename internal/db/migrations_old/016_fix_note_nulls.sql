-- +goose Up
-- Convert existing NULL note values to empty strings
UPDATE goal_entries SET note = '' WHERE note IS NULL;

-- +goose Down
-- No rollback needed (data migration)
