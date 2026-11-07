package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"bt-go/internal/config"
)

// PassphrasePrompter reads a passphrase from the user, displaying prompt first.
type PassphrasePrompter interface {
	PromptPassphrase(prompt string) (string, error)
}

// KeyGenerator generates the encryption key pair. bt.Encryptor satisfies it.
type KeyGenerator interface {
	Setup(passphrase string) error
}

// KeyGeneratorFactory builds the KeyGenerator for the given encryption config.
type KeyGeneratorFactory func(cfg config.EncryptionConfig) (KeyGenerator, error)

// DatabaseCreator creates the host's empty, fully migrated database. It must
// clean up after itself if it fails. database.CreateFromConfig satisfies it.
type DatabaseCreator func(cfg config.DatabaseConfig, hostID string) error

// InitParams describes a `bt config init` run.
type InitParams struct {
	ConfigPath string
	BaseDir    string
	HostID     string
	// FSVaultRoot, when non-empty, adds a filesystem vault rooted there.
	FSVaultRoot string
}

// Initializer performs `bt config init`: it prompts for a passphrase, writes
// the config file, creates the data directories, generates the key pair and
// creates the host database. It is all-or-nothing with respect to the config,
// key and database files.
type Initializer struct {
	prompter  PassphrasePrompter
	newKeyGen KeyGeneratorFactory
	createDB  DatabaseCreator
}

// NewInitializer creates an Initializer.
func NewInitializer(prompter PassphrasePrompter, newKeyGen KeyGeneratorFactory, createDB DatabaseCreator) *Initializer {
	return &Initializer{prompter: prompter, newKeyGen: newKeyGen, createDB: createDB}
}

// Sentinel errors returned by Initializer.Init for expected user mistakes.
var (
	ErrEmptyPassphrase    = errors.New("passphrase must not be empty")
	ErrPassphraseMismatch = errors.New("passphrases do not match")
)

// Init runs the initialization and returns the config that was written.
//
// Order of operations:
//  1. Refuse if the config file or either key file already exists.
//  2. Prompt for and validate the passphrase (nothing is written before this).
//  3. Create base_dir and its data, staging, log and keys subdirectories (0700).
//  4. Write the config file (0600).
//  5. Generate the key pair.
//  6. Create the host database with its schema, so `bt dir init` works next.
//
// If step 5 or 6 fails, the config file and key files written by this run are
// removed (the DatabaseCreator cleans up its own files), so init can simply be
// re-run. The empty directories from step 3 are left in place.
func (in *Initializer) Init(p InitParams) (*config.Config, error) {
	cfg := config.NewConfig(p.HostID, p.BaseDir)
	if p.FSVaultRoot != "" {
		v, err := config.NewFilesystemVaultConfig(p.FSVaultRoot)
		if err != nil {
			return nil, err
		}
		cfg.Vaults = append(cfg.Vaults, v)
	}

	for _, path := range []string{p.ConfigPath, cfg.Encryption.PublicKeyPath, cfg.Encryption.PrivateKeyPath} {
		exists, err := fileExists(path)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, fmt.Errorf("%s already exists; refusing to overwrite", path)
		}
	}

	keyGen, err := in.newKeyGen(cfg.Encryption)
	if err != nil {
		return nil, fmt.Errorf("creating encryptor: %w", err)
	}

	passphrase, err := in.readPassphrase()
	if err != nil {
		return nil, err
	}

	for _, dir := range []string{cfg.BaseDir, cfg.Database.DataDir, cfg.Staging.StagingDir, cfg.LogDir, filepath.Dir(cfg.Encryption.PrivateKeyPath)} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	if err := config.Init(p.ConfigPath, cfg); err != nil {
		return nil, fmt.Errorf("writing config: %w", err)
	}

	// rollback removes what this run wrote. The existence check above
	// guarantees none of these existed before, so this cannot lose user data.
	// Removal is best effort; the original error is what the caller needs.
	rollback := func() {
		for _, path := range []string{p.ConfigPath, cfg.Encryption.PublicKeyPath, cfg.Encryption.PrivateKeyPath} {
			_ = os.Remove(path)
		}
	}

	if err := keyGen.Setup(passphrase); err != nil {
		rollback()
		return nil, fmt.Errorf("generating encryption keys: %w", err)
	}
	if err := in.createDB(cfg.Database, cfg.HostID); err != nil {
		rollback()
		return nil, fmt.Errorf("creating database: %w", err)
	}
	return cfg, nil
}

// readPassphrase prompts twice and checks the passphrase is non-empty and confirmed.
func (in *Initializer) readPassphrase() (string, error) {
	passphrase, err := in.prompter.PromptPassphrase("Enter passphrase for encryption key: ")
	if err != nil {
		return "", fmt.Errorf("reading passphrase: %w", err)
	}
	if passphrase == "" {
		return "", ErrEmptyPassphrase
	}
	confirm, err := in.prompter.PromptPassphrase("Confirm passphrase: ")
	if err != nil {
		return "", fmt.Errorf("reading passphrase confirmation: %w", err)
	}
	if passphrase != confirm {
		return "", ErrPassphraseMismatch
	}
	return passphrase, nil
}

// fileExists reports whether path exists. Errors other than "not exist" are returned.
func fileExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	default:
		return false, fmt.Errorf("checking %s: %w", path, err)
	}
}
