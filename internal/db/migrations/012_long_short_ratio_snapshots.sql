-- +goose Up
CREATE TABLE IF NOT EXISTS long_short_ratio_snapshots (
    symbol TEXT NOT NULL,
    interval TEXT NOT NULL CHECK (interval IN ('5m', '1h', '1d')),
    snapshot_at TIMESTAMPTZ NOT NULL,
    positioned_wallet_count BIGINT NOT NULL,
    long_wallet_count BIGINT NOT NULL,
    short_wallet_count BIGINT NOT NULL,
    PRIMARY KEY (symbol, interval, snapshot_at)
);

CREATE INDEX IF NOT EXISTS long_short_ratio_snapshots_lookup_idx
    ON long_short_ratio_snapshots (symbol, interval, snapshot_at DESC);

-- +goose Down
DROP TABLE IF EXISTS long_short_ratio_snapshots;
