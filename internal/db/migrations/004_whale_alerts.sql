-- +goose Up
CREATE TABLE IF NOT EXISTS whale_alerts (
    id BIGSERIAL PRIMARY KEY,
    wallet_address TEXT NOT NULL REFERENCES wallets(address) ON DELETE CASCADE,
    symbol TEXT NOT NULL,
    position_size DOUBLE PRECISION NOT NULL,
    entry_price DOUBLE PRECISION NOT NULL,
    liquidation_price DOUBLE PRECISION NOT NULL,
    position_value_usd DOUBLE PRECISION NOT NULL,
    position_action SMALLINT NOT NULL CHECK (position_action IN (1, 2)),
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (wallet_address, symbol, position_action, created_at)
);

CREATE INDEX IF NOT EXISTS whale_alerts_recent_idx
    ON whale_alerts (created_at DESC, id DESC);

-- +goose Down
DROP TABLE IF EXISTS whale_alerts;
