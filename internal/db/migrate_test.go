package db

import "testing"

func TestMigrationVersion(t *testing.T) {
	version, err := migrationVersion("002_hypercore_postgres.sql")
	if err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("expected version 2, got %d", version)
	}
}

func TestMigrationUpSQLExcludesDownSection(t *testing.T) {
	sql := migrationUpSQL("-- +goose Up\nCREATE TABLE example (id int);\n-- +goose Down\nDROP TABLE example;")
	if sql != "CREATE TABLE example (id int);" {
		t.Fatalf("unexpected up migration: %q", sql)
	}
}
