package db

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
)

// dialectMap maps database drivers to Goose dialect names
var dialectMap = map[string]string{
	"sqlite": "sqlite3",
	"pgx":    "postgres",
	"mysql":  "mysql",
}

// getDialect returns the Goose dialect for the given driver
func getDialect(driver string) string {
	dialect, ok := dialectMap[driver]
	if ok {
		return dialect
	}
	return driver // fallback to driver name
}

// setupGoose configures Goose with the correct dialect and filesystem
func setupGoose(driver string) error {
	// Set dialect based on driver
	err := goose.SetDialect(getDialect(driver))
	if err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	// Get migrations subdirectory from embed.FS
	migrationsDir, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to get migrations directory: %w", err)
	}

	// Set base filesystem for migrations
	goose.SetBaseFS(migrationsDir)
	return nil
}

func RunMigrations(db *sql.DB, driver string) error {
	err := setupGoose(driver)
	if err != nil {
		return err
	}

	err = goose.Up(db, ".")
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	slog.Info("migrations completed successfully")
	return nil
}

func MigrateDown(db *sql.DB, driver string) error {
	err := setupGoose(driver)
	if err != nil {
		return err
	}

	err = goose.Down(db, ".")
	if err != nil {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	slog.Info("rolled back one migration")
	return nil
}
