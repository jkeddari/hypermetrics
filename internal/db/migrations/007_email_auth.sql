-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS auth_tokens (
    token_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('email_verification', 'magic_link')),
    plan_id TEXT CHECK (plan_id IN ('builder', 'pro')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_auth_tokens_user_type ON auth_tokens(user_id, type);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_expiry ON auth_tokens(expires_at);

-- Existing accounts predate verification and remain usable.
UPDATE users SET email_verified_at = created_at WHERE email_verified_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS auth_tokens;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified_at;
