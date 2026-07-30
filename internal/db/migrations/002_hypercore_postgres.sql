-- +goose Up
CREATE TABLE IF NOT EXISTS wallets (
    address TEXT PRIMARY KEY CHECK (address ~ '^0x[0-9a-f]{40}$'),
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_leaderboard_at TIMESTAMPTZ,
    last_refreshed_at TIMESTAMPTZ,
    last_successful_refresh_at TIMESTAMPTZ,
    last_failed_refresh_at TIMESTAMPTZ,
    refresh_attempts INTEGER NOT NULL DEFAULT 0,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    known_whale BOOLEAN NOT NULL DEFAULT FALSE,
    has_open_position BOOLEAN NOT NULL DEFAULT FALSE,
    max_position_value_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_position_value_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
    account_value DOUBLE PRECISION NOT NULL DEFAULT 0,
    leaderboard_rank INTEGER NOT NULL DEFAULT 0,
    leaderboard_account_value DOUBLE PRECISION NOT NULL DEFAULT 0,
    leaderboard_pnl DOUBLE PRECISION NOT NULL DEFAULT 0,
    leaderboard_roi DOUBLE PRECISION NOT NULL DEFAULT 0,
    manual_priority_boost DOUBLE PRECISION NOT NULL DEFAULT 0,
    next_refresh_at TIMESTAMPTZ,
    priority_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    tier SMALLINT NOT NULL DEFAULT 4,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS wallet_sources (
    wallet_address TEXT NOT NULL REFERENCES wallets(address) ON DELETE CASCADE,
    source TEXT NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (wallet_address, source)
);

CREATE INDEX IF NOT EXISTS wallets_listing_idx
    ON wallets (tier, priority_score DESC, last_refreshed_at DESC, address);

CREATE TABLE IF NOT EXISTS wallet_candidates (
    address TEXT PRIMARY KEY CHECK (address ~ '^0x[0-9a-f]{40}$'),
    status SMALLINT NOT NULL DEFAULT 0,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_leaderboard_at TIMESTAMPTZ NOT NULL,
    next_scan_at TIMESTAMPTZ,
    scan_attempts INTEGER NOT NULL DEFAULT 0,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    leaderboard_rank INTEGER NOT NULL DEFAULT 0,
    leaderboard_account_value DOUBLE PRECISION NOT NULL DEFAULT 0,
    leaderboard_pnl DOUBLE PRECISION NOT NULL DEFAULT 0,
    leaderboard_roi DOUBLE PRECISION NOT NULL DEFAULT 0,
    priority_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    rejected_at TIMESTAMPTZ,
    rejected_until TIMESTAMPTZ,
    real_account_value DOUBLE PRECISION NOT NULL DEFAULT 0,
    rejection_reason TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS candidates_status_priority_idx
    ON wallet_candidates (status, priority_score DESC, leaderboard_account_value DESC, address);

CREATE TABLE IF NOT EXISTS wallet_accounts_current (
    wallet_address TEXT PRIMARY KEY REFERENCES wallets(address) ON DELETE CASCADE,
    account_value DOUBLE PRECISION NOT NULL,
    margin_used DOUBLE PRECISION NOT NULL,
    withdrawable DOUBLE PRECISION NOT NULL,
    refreshed_at TIMESTAMPTZ NOT NULL,
    source_received_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS wallet_positions_current (
    wallet_address TEXT NOT NULL REFERENCES wallets(address) ON DELETE CASCADE,
    symbol TEXT NOT NULL,
    position_size DOUBLE PRECISION NOT NULL,
    entry_price DOUBLE PRECISION NOT NULL,
    mark_price DOUBLE PRECISION NOT NULL,
    liquidation_price DOUBLE PRECISION NOT NULL,
    leverage DOUBLE PRECISION NOT NULL,
    margin_balance DOUBLE PRECISION NOT NULL,
    position_value_usd DOUBLE PRECISION NOT NULL,
    unrealized_pnl DOUBLE PRECISION NOT NULL,
    refreshed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (wallet_address, symbol)
);

CREATE TABLE IF NOT EXISTS wallet_account_history (
    wallet_address TEXT NOT NULL REFERENCES wallets(address) ON DELETE CASCADE,
    account_value DOUBLE PRECISION NOT NULL,
    margin_used DOUBLE PRECISION NOT NULL,
    withdrawable DOUBLE PRECISION NOT NULL,
    refreshed_at TIMESTAMPTZ NOT NULL,
    source_received_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (wallet_address, refreshed_at)
);

CREATE TABLE IF NOT EXISTS wallet_position_history (
    wallet_address TEXT NOT NULL REFERENCES wallets(address) ON DELETE CASCADE,
    symbol TEXT NOT NULL,
    position_size DOUBLE PRECISION NOT NULL,
    entry_price DOUBLE PRECISION NOT NULL,
    mark_price DOUBLE PRECISION NOT NULL,
    liquidation_price DOUBLE PRECISION NOT NULL,
    leverage DOUBLE PRECISION NOT NULL,
    margin_balance DOUBLE PRECISION NOT NULL,
    position_value_usd DOUBLE PRECISION NOT NULL,
    unrealized_pnl DOUBLE PRECISION NOT NULL,
    refreshed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (wallet_address, symbol, refreshed_at)
);

CREATE INDEX IF NOT EXISTS positions_symbol_value_idx
    ON wallet_positions_current (symbol, position_value_usd DESC, wallet_address);

CREATE INDEX IF NOT EXISTS positions_whales_idx
    ON wallet_positions_current (position_value_usd DESC, wallet_address)
    WHERE position_value_usd >= 1000000;

CREATE INDEX IF NOT EXISTS account_history_time_idx
    ON wallet_account_history (refreshed_at DESC, wallet_address);

CREATE INDEX IF NOT EXISTS position_history_symbol_time_idx
    ON wallet_position_history (symbol, refreshed_at DESC, position_value_usd DESC);

CREATE INDEX IF NOT EXISTS position_history_wallet_time_idx
    ON wallet_position_history (wallet_address, refreshed_at DESC);

CREATE TABLE IF NOT EXISTS wallet_refresh_queue (
    wallet_address TEXT PRIMARY KEY CHECK (wallet_address ~ '^0x[0-9a-f]{40}$'),
    job_kind SMALLINT NOT NULL,
    next_refresh_at TIMESTAMPTZ NOT NULL,
    refresh_deadline_at TIMESTAMPTZ NOT NULL,
    priority_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    last_refreshed_at TIMESTAMPTZ,
    locked_until TIMESTAMPTZ,
    locked_by TEXT,
    attempts INTEGER NOT NULL DEFAULT 0,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS refresh_queue_due_idx
    ON wallet_refresh_queue (next_refresh_at, refresh_deadline_at, priority_score DESC);

CREATE INDEX IF NOT EXISTS refresh_queue_priority_idx
    ON wallet_refresh_queue (priority_score DESC, next_refresh_at);

CREATE INDEX IF NOT EXISTS refresh_queue_coverage_idx
    ON wallet_refresh_queue (refresh_deadline_at, last_refreshed_at ASC NULLS FIRST, priority_score DESC);

-- +goose Down
DROP TABLE IF EXISTS wallet_refresh_queue;
DROP TABLE IF EXISTS wallet_position_history;
DROP TABLE IF EXISTS wallet_account_history;
DROP TABLE IF EXISTS wallet_positions_current;
DROP TABLE IF EXISTS wallet_accounts_current;
DROP TABLE IF EXISTS wallet_candidates;
DROP TABLE IF EXISTS wallet_sources;
DROP TABLE IF EXISTS wallets;
