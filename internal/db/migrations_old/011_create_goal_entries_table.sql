-- +goose Up
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

-- Index for goal lookups
CREATE INDEX IF NOT EXISTS idx_goal_entries_goal_id ON goal_entries(goal_id);

-- Index for completed queries
CREATE INDEX IF NOT EXISTS idx_goal_entries_completed ON goal_entries(goal_id, completed);

-- Index for step ordering
CREATE INDEX IF NOT EXISTS idx_goal_entries_step ON goal_entries(goal_id, step);

-- +goose Down
DROP INDEX IF EXISTS idx_goal_entries_step;
DROP INDEX IF EXISTS idx_goal_entries_completed;
DROP INDEX IF EXISTS idx_goal_entries_goal_id;
DROP TABLE IF EXISTS goal_entries;
