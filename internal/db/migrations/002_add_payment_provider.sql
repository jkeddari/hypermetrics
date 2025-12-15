-- +goose Up
-- Add payment provider column to track which payment service is used
-- Default to 'polar' for existing subscriptions

ALTER TABLE subscriptions
ADD COLUMN provider TEXT NOT NULL DEFAULT 'polar';

CREATE INDEX IF NOT EXISTS idx_subscriptions_provider ON subscriptions(provider);

-- +goose Down
-- Remove payment provider column and index

DROP INDEX IF EXISTS idx_subscriptions_provider;

-- SQLite doesn't support DROP COLUMN directly, so we need to recreate the table
-- PostgreSQL supports DROP COLUMN, so this works for both with goose
ALTER TABLE subscriptions DROP COLUMN provider;
