-- +goose Up
CREATE TABLE IF NOT EXISTS distribution_snapshots (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    computed_at TIMESTAMPTZ NOT NULL,
    position_distribution JSONB NOT NULL,
    pnl_distribution JSONB NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS distribution_snapshots;
