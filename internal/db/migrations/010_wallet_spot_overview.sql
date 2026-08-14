-- +goose Up
CREATE TABLE IF NOT EXISTS spot_tokens (
    token_index INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    sz_decimals SMALLINT NOT NULL,
    wei_decimals SMALLINT NOT NULL,
    token_id TEXT NOT NULL,
    is_canonical BOOLEAN NOT NULL DEFAULT FALSE,
    evm_contract TEXT NOT NULL DEFAULT '',
    full_name TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS spot_markets (
    market_index INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    base_token_index INTEGER NOT NULL,
    quote_token_index INTEGER NOT NULL,
    is_canonical BOOLEAN NOT NULL DEFAULT FALSE,
    mark_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    mid_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    previous_day_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    day_notional_volume DOUBLE PRECISION NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS spot_markets_base_token_idx
    ON spot_markets (base_token_index, quote_token_index);

CREATE TABLE IF NOT EXISTS wallet_spot_balances_current (
    wallet_address TEXT NOT NULL REFERENCES wallets(address) ON DELETE CASCADE,
    token_index INTEGER NOT NULL,
    coin TEXT NOT NULL,
    hold DOUBLE PRECISION NOT NULL,
    total DOUBLE PRECISION NOT NULL,
    entry_ntl DOUBLE PRECISION NOT NULL,
    mark_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    value_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
    unrealized_pnl DOUBLE PRECISION NOT NULL DEFAULT 0,
    refreshed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (wallet_address, token_index)
);

CREATE TABLE IF NOT EXISTS wallet_spot_balance_history (
    wallet_address TEXT NOT NULL REFERENCES wallets(address) ON DELETE CASCADE,
    token_index INTEGER NOT NULL,
    coin TEXT NOT NULL,
    hold DOUBLE PRECISION NOT NULL,
    total DOUBLE PRECISION NOT NULL,
    entry_ntl DOUBLE PRECISION NOT NULL,
    mark_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    value_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
    unrealized_pnl DOUBLE PRECISION NOT NULL DEFAULT 0,
    refreshed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (wallet_address, token_index, refreshed_at)
);

CREATE INDEX IF NOT EXISTS wallet_spot_balance_history_time_idx
    ON wallet_spot_balance_history (wallet_address, refreshed_at DESC);

CREATE TABLE IF NOT EXISTS wallet_open_orders_current (
    wallet_address TEXT NOT NULL REFERENCES wallets(address) ON DELETE CASCADE,
    oid BIGINT NOT NULL,
    client_oid TEXT NOT NULL DEFAULT '',
    dex TEXT NOT NULL DEFAULT '',
    coin TEXT NOT NULL,
    market_type TEXT NOT NULL,
    side TEXT NOT NULL,
    order_type TEXT NOT NULL,
    limit_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    size DOUBLE PRECISION NOT NULL DEFAULT 0,
    original_size DOUBLE PRECISION NOT NULL DEFAULT 0,
    reduce_only BOOLEAN NOT NULL DEFAULT FALSE,
    is_trigger BOOLEAN NOT NULL DEFAULT FALSE,
    is_position_tpsl BOOLEAN NOT NULL DEFAULT FALSE,
    trigger_condition TEXT NOT NULL DEFAULT '',
    trigger_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    order_timestamp BIGINT NOT NULL DEFAULT 0,
    refreshed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (wallet_address, dex, oid)
);

CREATE INDEX IF NOT EXISTS wallet_open_orders_coin_idx
    ON wallet_open_orders_current (coin, market_type, wallet_address);

-- +goose Down
DROP TABLE IF EXISTS wallet_open_orders_current;
DROP TABLE IF EXISTS wallet_spot_balance_history;
DROP TABLE IF EXISTS wallet_spot_balances_current;
DROP TABLE IF EXISTS spot_markets;
DROP TABLE IF EXISTS spot_tokens;
