package db

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

func InitDev() (*sqlx.DB, error) {
	// SQLite: create data directory.
	connection := "./data/acme.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	dir := filepath.Dir(connection)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	db, err := sqlx.Connect("sqlite", connection)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Connection pool configuration (good defaults for all drivers)
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// InitProduction connects to Cloud SQL using the Cloud SQL Connector
func InitProduction(supabasePWD string) (*sqlx.DB, error) {
	slog.Info("connecting to Supabase SQL")

	connStr := fmt.Sprintf("postgresql://postgres:%s@db.ymqjkrwmpkxharauatgy.supabase.co:5432/postgres", supabasePWD)
	// Open connection with sqlx
	db, err := sqlx.Connect("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Connection pool configuration optimized for serverless
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(30 * time.Second)

	slog.Info("database connected via Supabase SQL")

	// Test connection
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func Close(db *sqlx.DB) error {
	if db != nil {
		return db.Close()
	}
	return nil
}
