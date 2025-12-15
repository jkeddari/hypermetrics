-- +goose Up
ALTER TABLE subscriptions ADD COLUMN amount INTEGER;
ALTER TABLE subscriptions ADD COLUMN currency TEXT;
ALTER TABLE subscriptions ADD COLUMN interval TEXT;

-- +goose Down
ALTER TABLE subscriptions DROP COLUMN amount;
ALTER TABLE subscriptions DROP COLUMN currency;
ALTER TABLE subscriptions DROP COLUMN interval;
