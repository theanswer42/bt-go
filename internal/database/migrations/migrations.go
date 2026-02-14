package migrations

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed files/*.sql
var migrationFiles embed.FS

// CheckDBMigrationStatus verifies that the database schema is up-to-date.
// Returns nil if the database is at the latest version.
// Returns an error describing any version mismatch or migration issues.
func CheckDBMigrationStatus(db *sql.DB) error {
	m, err := newMigrate(db)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	// Note: We don't close m here because it would close the db connection
	// The caller owns the db and is responsible for closing it

	version, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return fmt.Errorf("database has no schema version (needs migration)")
		}
		return fmt.Errorf("failed to get database version: %w", err)
	}

	if dirty {
		return fmt.Errorf("database is in dirty state at version %d (migration failed previously)", version)
	}

	// Get the latest version from migration files
	sourceDriver, err := iofs.New(migrationFiles, "files")
	if err != nil {
		return fmt.Errorf("failed to read migration files: %w", err)
	}
	defer sourceDriver.Close()

	// Find the latest version by checking what migrations are available
	latestVersion, err := getLatestVersion(sourceDriver)
	if err != nil {
		return fmt.Errorf("failed to determine latest version: %w", err)
	}

	if version < latestVersion {
		return fmt.Errorf("database is at version %d but latest is %d (%d migrations behind)",
			version, latestVersion, latestVersion-version)
	}

	if version > latestVersion {
		return fmt.Errorf("database version %d is ahead of binary version %d (binary needs update)",
			version, latestVersion)
	}

	// version == latestVersion
	return nil
}

// MigrateUp runs all pending migrations to bring database to latest version.
func MigrateUp(db *sql.DB) error {
	m, err := newMigrate(db)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	// Note: We don't close m here because it would close the db connection
	// The caller owns the db and is responsible for closing it

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			// Database is already at latest version - this is fine
			return nil
		}
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}

// newMigrate creates a new migrate instance for the given database.
func newMigrate(db *sql.DB) (*migrate.Migrate, error) {
	// Create source driver from embedded files
	sourceDriver, err := iofs.New(migrationFiles, "files")
	if err != nil {
		return nil, fmt.Errorf("failed to create source driver: %w", err)
	}

	// Create database driver (wraps *sql.DB with SQLite-specific migration logic)
	dbDriver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		sourceDriver.Close()
		return nil, fmt.Errorf("failed to create database driver: %w", err)
	}

	// Create migrate instance
	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite3", dbDriver)
	if err != nil {
		sourceDriver.Close()
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	return m, nil
}

// getLatestVersion returns the highest version number available in the source.
func getLatestVersion(src source.Driver) (uint, error) {
	// Read the first migration version
	version, err := src.First()
	if err != nil {
		return 0, err
	}

	// Keep reading next versions until we reach the end
	latestVersion := version
	for {
		nextVersion, err := src.Next(latestVersion)
		if err != nil {
			// Any error from Next() means we've reached the end
			// (no more migrations available)
			break
		}
		latestVersion = nextVersion
	}

	return latestVersion, nil
}
