package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"bt-go/internal/bt"
	"bt-go/internal/config"
	"bt-go/internal/database"
	"bt-go/internal/database/sqlc"
	"bt-go/internal/encryption"
	"bt-go/internal/fs"
	"bt-go/internal/staging"
	"bt-go/internal/vault"
)

// BTApp is the application layer between the CLI and BTService.
// It constructs all dependencies from config, exposes high-level operations
// that accept raw string paths, and manages the DB lifecycle on Close.
type BTApp struct {
	cfg       *config.Config
	db        bt.Database
	vault     bt.Vault
	staging   bt.StagingArea
	fsmgr     bt.FilesystemManager
	encryptor bt.Encryptor
	service   *bt.BTService
	op        *BackupOperation
	logFile   *os.File
}

// NewBTApp creates a fully wired BTApp from the given config.
// operation identifies the CLI command being run (e.g. "AddDirectory", "BackupAll").
// The caller must call Close when done.
func NewBTApp(cfg *config.Config, operation string) (*BTApp, error) {
	fsmgr := fs.NewOSFilesystemManager(cfg.Filesystem.Ignore)

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

	// Check local DB version against remote vault version.
	remoteVersion, err := v.GetMetadataVersion(cfg.HostID, "db")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("checking remote metadata version: %w", err)
	}

	localMax, err := db.MaxBackupOperationID()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("checking local metadata version: %w", err)
	}

	if remoteVersion > localMax {
		db.Close()
		return nil, fmt.Errorf("local database is behind remote (local=%d, remote=%d): restore from vault or re-initialize", localMax, remoteVersion)
	}

	enc, err := encryption.NewEncryptorFromConfig(cfg.Encryption)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("creating encryptor: %w", err)
	}

	opID := time.Now().UTC().Format("20060102T150405Z")
	logger, logFile, err := newLogger(cfg.LogDir, opID)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("creating logger: %w", err)
	}

	svc := bt.NewBTService(db, sa, v, fsmgr, enc, &slogAdapter{l: logger}, bt.RealClock{}, bt.UUIDGenerator{})
	op := NewBackupOperation(operation, "")

	return &BTApp{
		cfg:       cfg,
		db:        db,
		vault:     v,
		staging:   sa,
		fsmgr:     fsmgr,
		encryptor: enc,
		service:   svc,
		op:        op,
		logFile:   logFile,
	}, nil
}

// persistOperation saves the backup operation to the database, giving it an auto-increment ID.
// This should only be called for DB-mutating commands.
func (a *BTApp) persistOperation() error {
	if a.op.Persisted() {
		return nil // already persisted
	}
	dbOp, err := a.db.CreateBackupOperation(a.op.Operation, a.op.Parameters)
	if err != nil {
		return fmt.Errorf("persisting backup operation: %w", err)
	}
	a.op.ID = dbOp.ID
	return nil
}

// AddDirectory resolves the given path and registers it for tracking.
// encrypted marks whether files in this directory should be encrypted on backup.
func (a *BTApp) AddDirectory(rawPath string, encrypted bool) error {
	if err := a.persistOperation(); err != nil {
		return err
	}
	p, err := a.fsmgr.Resolve(rawPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}
	return a.service.AddDirectory(p, encrypted)
}

// StageFiles resolves the given path and stages file(s) for backup.
// If the path is a directory, all discovered files are staged.
// When recursive is true, files in subdirectories are included.
// Returns the number of files staged.
func (a *BTApp) StageFiles(rawPath string, recursive bool) (int, error) {
	p, err := a.fsmgr.Resolve(rawPath)
	if err != nil {
		return 0, fmt.Errorf("resolving path: %w", err)
	}
	return a.service.StageFiles(p, recursive)
}

// GetStatus returns the backup status of files under the given path.
func (a *BTApp) GetStatus(rawPath string, recursive bool) ([]*bt.FileStatus, error) {
	p, err := a.fsmgr.Resolve(rawPath)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}
	return a.service.GetStatus(p, recursive)
}

// GetFileHistory resolves the given path and returns its backup history.
func (a *BTApp) GetFileHistory(rawPath string) ([]*bt.FileHistoryEntry, error) {
	p, err := a.fsmgr.Resolve(rawPath)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}
	return a.service.GetFileHistory(p)
}

// GetHistory returns the most recent backup operations.
func (a *BTApp) GetHistory(limit int) ([]*sqlc.BackupOperation, error) {
	return a.service.GetHistory(limit)
}

// EncryptionConfigured returns true if the encryption key files exist.
func (a *BTApp) EncryptionConfigured() bool {
	return a.encryptor.IsConfigured()
}

// UnlockEncryption decrypts the private key using the given passphrase and returns
// a DecryptionContext for use during the restore session.
func (a *BTApp) UnlockEncryption(passphrase string) (bt.DecryptionContext, error) {
	return a.encryptor.Unlock(passphrase)
}

// RestoreFiles resolves the given path and restores file(s) from the vault.
// The path may not exist on disk — resolution uses filepath.Abs only.
// If checksum is non-empty, restores a specific version (file only, not directory).
// decryptCtx must be non-nil when restoring encrypted files; pass nil for unencrypted restores.
// Returns the list of restored file paths.
func (a *BTApp) RestoreFiles(rawPath string, checksum string, decryptCtx bt.DecryptionContext) ([]string, error) {
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}
	return a.service.Restore(absPath, checksum, decryptCtx)
}

// BackupAll processes all staged files and backs them up to the vault.
// Returns the number of files backed up.
func (a *BTApp) BackupAll() (int, error) {
	if err := a.persistOperation(); err != nil {
		return 0, err
	}
	return a.service.BackupAll()
}

// Close finalizes the operation and closes all resources.
// For persisted operations: finishes the operation record, backs up the DB, and uploads to vault.
// For non-persisted operations: just closes the database.
func (a *BTApp) Close() error {
	var errs []error

	if a.op.Persisted() {
		// Finalize the operation record
		if err := a.db.FinishBackupOperation(a.op.ID, a.op.Status); err != nil {
			errs = append(errs, fmt.Errorf("finishing backup operation: %w", err))
		}

		// Snapshot the DB to a temp file
		tmpFile, err := os.CreateTemp("", "bt-db-backup-*.db")
		if err != nil {
			errs = append(errs, fmt.Errorf("creating temp file for db backup: %w", err))
		}

		var tmpPath string
		if tmpFile != nil {
			tmpPath = tmpFile.Name()
			tmpFile.Close()

			if err := a.db.BackupTo(tmpPath); err != nil {
				errs = append(errs, fmt.Errorf("backing up database: %w", err))
				tmpPath = "" // skip vault upload
			}
		}

		// Close the database
		if err := a.db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing database: %w", err))
		}

		// Upload DB snapshot to vault with version = operation ID
		if tmpPath != "" {
			if err := a.uploadMetadata(tmpPath, a.op.ID); err != nil {
				errs = append(errs, err)
			}
		}

		// Clean up temp file
		if tmpPath != "" {
			os.Remove(tmpPath)
		}

		// Upload encryption key files to vault (idempotent; version is always 1).
		if a.encryptor.IsConfigured() {
			if err := a.uploadKeyMetadata(); err != nil {
				errs = append(errs, err)
			}
		}
	} else {
		// Non-mutating operation: just close the database, no upload
		if err := a.db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing database: %w", err))
		}
	}

	if a.logFile != nil {
		a.logFile.Close()
	}

	return errors.Join(errs...)
}

// uploadMetadata opens the temp DB file, encrypts it if encryption is configured,
// and uploads it to the vault as metadata.
func (a *BTApp) uploadMetadata(path string, version int64) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening db backup for upload: %w", err)
	}
	defer f.Close()

	if a.encryptor.IsConfigured() {
		encTmp, err := os.CreateTemp("", "bt-meta-enc-*.tmp")
		if err != nil {
			return fmt.Errorf("creating encrypted temp file: %w", err)
		}
		encTmpPath := encTmp.Name()
		defer os.Remove(encTmpPath)

		if err := a.encryptor.Encrypt(f, encTmp); err != nil {
			encTmp.Close()
			return fmt.Errorf("encrypting db backup: %w", err)
		}
		info, err := encTmp.Stat()
		if err != nil {
			encTmp.Close()
			return fmt.Errorf("stat encrypted db temp file: %w", err)
		}
		if _, err := encTmp.Seek(0, io.SeekStart); err != nil {
			encTmp.Close()
			return fmt.Errorf("seeking encrypted db temp file: %w", err)
		}
		if err := a.vault.PutMetadata(a.cfg.HostID, "db", encTmp, info.Size(), version); err != nil {
			encTmp.Close()
			return fmt.Errorf("uploading metadata to vault: %w", err)
		}
		encTmp.Close()
		return nil
	}

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat db backup: %w", err)
	}

	if err := a.vault.PutMetadata(a.cfg.HostID, "db", f, info.Size(), version); err != nil {
		return fmt.Errorf("uploading metadata to vault: %w", err)
	}

	return nil
}

// uploadKeyMetadata uploads the public and private key files to the vault as metadata.
// Keys use a fixed version (1) since they are immutable after initial setup.
func (a *BTApp) uploadKeyMetadata() error {
	keys := []struct{ name, path string }{
		{"public_key", a.cfg.Encryption.PublicKeyPath},
		{"private_key", a.cfg.Encryption.PrivateKeyPath},
	}
	for _, k := range keys {
		f, err := os.Open(k.path)
		if err != nil {
			return fmt.Errorf("opening %s for upload: %w", k.name, err)
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return fmt.Errorf("stat %s: %w", k.name, err)
		}
		if err := a.vault.PutMetadata(a.cfg.HostID, k.name, f, info.Size(), 1); err != nil {
			f.Close()
			return fmt.Errorf("uploading %s to vault: %w", k.name, err)
		}
		f.Close()
	}
	return nil
}
