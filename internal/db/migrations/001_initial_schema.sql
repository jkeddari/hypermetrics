-- +goose Up
-- Initial database schema for Goilerplate
-- Compatible with both PostgreSQL and SQLite

-- ============================================================================
-- USERS TABLE
-- Core user accounts with optional password (supports OAuth/magic link)
-- ============================================================================
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NULL,  -- Explicitly nullable for passwordless authentication
    pending_email TEXT NULL,
    email_verified_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_passwordless ON users(id) WHERE password_hash IS NULL;

-- ============================================================================
-- TOKENS TABLE
-- Verification tokens, password reset tokens, magic links
-- ============================================================================
CREATE TABLE IF NOT EXISTS tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL,
    token TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_tokens_token ON tokens(token);
CREATE INDEX IF NOT EXISTS idx_tokens_expires_at ON tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_tokens_user_id ON tokens(user_id);

-- ============================================================================
-- PROFILES TABLE
-- User profile information (name, bio, etc.)
-- ============================================================================
CREATE TABLE IF NOT EXISTS profiles (
    id TEXT PRIMARY KEY,
    user_id TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_profiles_user_id ON profiles(user_id);

-- ============================================================================
-- FILES TABLE
-- File uploads (avatars, documents, etc.)
-- user_id: Who owns/created the file (enables "all user files" queries)
-- owner_type + owner_id: Polymorphic relationship (what entity it belongs to)
-- ============================================================================
CREATE TABLE IF NOT EXISTS files (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    owner_type TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    type TEXT NOT NULL,
    filename TEXT NOT NULL,
    original_name TEXT,
    mime_type TEXT,
    size INTEGER,
    storage_path TEXT NOT NULL,
    public BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);
CREATE INDEX IF NOT EXISTS idx_files_owner ON files(owner_type, owner_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_files_owner_type ON files(owner_type, owner_id, type) WHERE type IN ('avatar');

-- ============================================================================
-- SUBSCRIPTIONS TABLE
-- Subscription plans and billing information
-- ============================================================================
CREATE TABLE IF NOT EXISTS subscriptions (
    id TEXT PRIMARY KEY,
    user_id TEXT UNIQUE NOT NULL,
    plan_id TEXT NOT NULL DEFAULT 'free',
    status TEXT NOT NULL DEFAULT 'active',
    provider_customer_id TEXT,
    provider_subscription_id TEXT,
    current_period_end TIMESTAMP,
    amount INTEGER,
    currency TEXT,
    interval TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON subscriptions(status);

-- ============================================================================
-- GOALS TABLE
-- User goals and progress tracking
-- ============================================================================
CREATE TABLE IF NOT EXISTS goals (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    current_step INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_goals_user_id ON goals(user_id);
CREATE INDEX IF NOT EXISTS idx_goals_status ON goals(status);
CREATE INDEX IF NOT EXISTS idx_goals_user_status ON goals(user_id, status);

-- ============================================================================
-- GOAL ENTRIES TABLE
-- Individual steps/entries for goals
-- ============================================================================
CREATE TABLE IF NOT EXISTS goal_entries (
    id TEXT PRIMARY KEY,
    goal_id TEXT NOT NULL,
    step INTEGER NOT NULL,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    note TEXT,
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (goal_id) REFERENCES goals(id) ON DELETE CASCADE,
    UNIQUE(goal_id, step)
);

CREATE INDEX IF NOT EXISTS idx_goal_entries_goal_id ON goal_entries(goal_id);
CREATE INDEX IF NOT EXISTS idx_goal_entries_completed ON goal_entries(goal_id, completed);
CREATE INDEX IF NOT EXISTS idx_goal_entries_step ON goal_entries(goal_id, step);

-- +goose Down
-- Drop all tables in reverse order (respecting foreign keys)
DROP INDEX IF EXISTS idx_goal_entries_step;
DROP INDEX IF EXISTS idx_goal_entries_completed;
DROP INDEX IF EXISTS idx_goal_entries_goal_id;
DROP TABLE IF EXISTS goal_entries;

DROP INDEX IF EXISTS idx_goals_user_status;
DROP INDEX IF EXISTS idx_goals_status;
DROP INDEX IF EXISTS idx_goals_user_id;
DROP TABLE IF EXISTS goals;

DROP INDEX IF EXISTS idx_subscriptions_status;
DROP INDEX IF EXISTS idx_subscriptions_user_id;
DROP TABLE IF EXISTS subscriptions;

DROP INDEX IF EXISTS idx_files_owner_type;
DROP INDEX IF EXISTS idx_files_owner;
DROP INDEX IF EXISTS idx_files_user_id;
DROP TABLE IF EXISTS files;

DROP INDEX IF EXISTS idx_profiles_user_id;
DROP TABLE IF EXISTS profiles;

DROP INDEX IF EXISTS idx_tokens_user_id;
DROP INDEX IF EXISTS idx_tokens_expires_at;
DROP INDEX IF EXISTS idx_tokens_token;
DROP TABLE IF EXISTS tokens;

DROP INDEX IF EXISTS idx_users_passwordless;
DROP INDEX IF EXISTS idx_users_email;
DROP TABLE IF EXISTS users;
