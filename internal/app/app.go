package app

import (
	"fmt"
	"os"

	"bt-go/internal/bt"
	"bt-go/internal/config"
	"bt-go/internal/database"
	"bt-go/internal/fs"
	"bt-go/internal/staging"
	"bt-go/internal/vault"
)

// BTApp is the application layer between the CLI and BTService.
// It constructs all dependencies from config, exposes high-level operations
// that accept raw string paths, and manages the DB lifecycle on Close.
type BTApp struct {
	cfg     *config.Config
	db      bt.Database
	vault   bt.Vault
	staging bt.StagingArea
	fsmgr   bt.FilesystemManager
	service *bt.BTService
}

// NewBTApp creates a fully wired BTApp from the given config.
// The caller must call Close when done.
func NewBTApp(cfg *config.Config) (*BTApp, error) {
	fsmgr := fs.NewOSFilesystemManager()

	if len(cfg.Vaults) == 0 {
		return nil, fmt.Errorf("no vaults configured")
	}
	v, err := vault.NewVaultFromConfig(cfg.Vaults[0])
	if err != nil {
		return nil, fmt.Errorf("creating vault: %w", err)
	}

	sa, err := staging.NewStagingAreaFromConfig(cfg.Staging, fsmgr)
	if err != nil {
		return nil, fmt.Errorf("creating staging area: %w", err)
	}

	db, err := database.NewDatabaseFromConfig(cfg.Database, cfg.HostID)
	if err != nil {
		return nil, fmt.Errorf("creating database: %w", err)
	}

	if err := db.CheckMigrations(); err != nil {
		db.Close()
		return nil, fmt.Errorf("database schema out of date: %w", err)
	}

	svc := bt.NewBTService(db, sa, v, fsmgr)

	return &BTApp{
		cfg:     cfg,
		db:      db,
		vault:   v,
		staging: sa,
		fsmgr:   fsmgr,
		service: svc,
	}, nil
}

// AddDirectory resolves the given path and registers it for tracking.
func (a *BTApp) AddDirectory(rawPath string) error {
	p, err := a.fsmgr.Resolve(rawPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}
	return a.service.AddDirectory(p)
}

// StageFile resolves the given path and stages it for backup.
func (a *BTApp) StageFile(rawPath string) error {
	p, err := a.fsmgr.Resolve(rawPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}
	return a.service.StageFile(p)
}

// BackupAll processes all staged files and backs them up to the vault.
// Returns the number of files backed up.
func (a *BTApp) BackupAll() (int, error) {
	return a.service.BackupAll()
}

// Close backs up the database to the vault and closes all resources.
// It attempts all steps even if earlier ones fail, returning the first error.
func (a *BTApp) Close() error {
	var firstErr error

	// 1. Snapshot the DB to a temp file
	tmpFile, err := os.CreateTemp("", "bt-db-backup-*.db")
	if err != nil {
		firstErr = fmt.Errorf("creating temp file for db backup: %w", err)
	}

	var tmpPath string
	if tmpFile != nil {
		tmpPath = tmpFile.Name()
		tmpFile.Close()

		if err := a.db.BackupTo(tmpPath); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("backing up database: %w", err)
			}
			tmpPath = "" // skip vault upload
		}
	}

	// 2. Close the database
	if err := a.db.Close(); err != nil {
		if firstErr == nil {
			firstErr = fmt.Errorf("closing database: %w", err)
		}
	}

	// 3. Upload DB snapshot to vault
	if tmpPath != "" {
		if err := a.uploadMetadata(tmpPath); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	// 4. Clean up temp file
	if tmpPath != "" {
		os.Remove(tmpPath)
	}

	return firstErr
}

// uploadMetadata opens the temp DB file and uploads it to the vault as metadata.
func (a *BTApp) uploadMetadata(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening db backup for upload: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat db backup: %w", err)
	}

	if err := a.vault.PutMetadata(a.cfg.HostID, f, info.Size()); err != nil {
		return fmt.Errorf("uploading metadata to vault: %w", err)
	}

	return nil
}
