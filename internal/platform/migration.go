package platform

import (
	"database/sql"
	"fmt"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func init() {
	if err := goose.SetDialect("postgres"); err != nil {
		slog.Error("Failed to set goose dialect to postgres", "error", err)
	}
}

// openMigrationDB opens an sql.DB connection for goose using pgx stdlib driver.
func openMigrationDB(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database for migrations: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migration database ping failed: %w", err)
	}
	return db, nil
}

// MigrateUp applies all available pending database migrations.
func MigrateUp(databaseURL, migrationsDir string) error {
	db, err := openMigrationDB(databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("failed to apply migrations up: %w", err)
	}
	return nil
}

// MigrateDown rolls back the most recently applied database migration.
func MigrateDown(databaseURL, migrationsDir string) error {
	db, err := openMigrationDB(databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := goose.Down(db, migrationsDir); err != nil {
		return fmt.Errorf("failed to roll back migration: %w", err)
	}
	return nil
}

// MigrateReset rolls back all applied migrations.
func MigrateReset(databaseURL, migrationsDir string) error {
	db, err := openMigrationDB(databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := goose.Reset(db, migrationsDir); err != nil {
		return fmt.Errorf("failed to reset migrations: %w", err)
	}
	return nil
}

// MigrateStatus prints the current migration status.
func MigrateStatus(databaseURL, migrationsDir string) error {
	db, err := openMigrationDB(databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := goose.Status(db, migrationsDir); err != nil {
		return fmt.Errorf("failed to get migration status: %w", err)
	}
	return nil
}

