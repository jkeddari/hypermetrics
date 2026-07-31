-- +goose Up
CREATE TABLE IF NOT EXISTS s3_ingestion_objects (
    object_key TEXT PRIMARY KEY,
    etag TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    wallet_count INTEGER NOT NULL,
    new_wallet_count INTEGER NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS s3_ingestion_objects;
