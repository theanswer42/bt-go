package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bt-go/internal/config"
)

// fakePrompter returns the queued answers in order.
type fakePrompter struct {
	answers []string
	err     error
	calls   int
}

func (f *fakePrompter) PromptPassphrase(string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if f.calls >= len(f.answers) {
		return "", errors.New("unexpected prompt")
	}
	a := f.answers[f.calls]
	f.calls++
	return a, nil
}

// fakeKeyGen writes placeholder key files, optionally failing after writing
// the public key to simulate a partial Setup.
type fakeKeyGen struct {
	cfg        config.EncryptionConfig
	fail       bool
	passphrase string
}

func (k *fakeKeyGen) Setup(passphrase string) error {
	k.passphrase = passphrase
	if err := os.WriteFile(k.cfg.PublicKeyPath, []byte("pub"), 0644); err != nil {
		return err
	}
	if k.fail {
		return errors.New("keygen exploded")
	}
	return os.WriteFile(k.cfg.PrivateKeyPath, []byte("priv"), 0600)
}

type initEnv struct {
	configPath string
	baseDir    string
}

func newInitEnv(t *testing.T) initEnv {
	t.Helper()
	root := t.TempDir()
	return initEnv{
		configPath: filepath.Join(root, "config", "bt.toml"),
		baseDir:    filepath.Join(root, "home"),
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("mode of %s = %o, want %o", path, got, want)
	}
}

func assertNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s should not exist (stat err = %v)", path, err)
	}
}

func TestInitializer_Init(t *testing.T) {
	tests := []struct {
		name        string
		answers     []string
		promptErr   error
		keygenFail  bool
		dbErr       error
		fsVault     string
		preexisting func(env initEnv) string // creates a file, returns its path
		wantErr     error                    // matched with errors.Is when set
		wantErrText string                   // substring, when set
		wantPrompts int
	}{
		{name: "happy path without vault", answers: []string{"pw", "pw"}, wantPrompts: 2},
		{name: "happy path with fs vault", answers: []string{"pw", "pw"}, fsVault: "/tmp/vault", wantPrompts: 2},
		{name: "empty passphrase", answers: []string{""}, wantErr: ErrEmptyPassphrase, wantPrompts: 1},
		{name: "mismatched confirmation", answers: []string{"pw", "typo"}, wantErr: ErrPassphraseMismatch, wantPrompts: 2},
		{name: "prompt error", promptErr: errors.New("no tty"), wantErrText: "no tty"},
		{name: "key generation fails", answers: []string{"pw", "pw"}, keygenFail: true, wantErrText: "keygen exploded", wantPrompts: 2},
		{name: "database creation fails", answers: []string{"pw", "pw"}, dbErr: errors.New("disk full"), wantErrText: "disk full", wantPrompts: 2},
		{
			name: "config already exists",
			preexisting: func(env initEnv) string {
				return env.configPath
			},
			wantErrText: "already exists",
		},
		{
			name: "public key already exists",
			preexisting: func(env initEnv) string {
				return filepath.Join(env.baseDir, "keys", "bt.pub")
			},
			wantErrText: "already exists",
		},
		{
			name: "private key already exists",
			preexisting: func(env initEnv) string {
				return filepath.Join(env.baseDir, "keys", "bt.key")
			},
			wantErrText: "already exists",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newInitEnv(t)

			var preexisting string
			if tt.preexisting != nil {
				preexisting = tt.preexisting(env)
				if err := os.MkdirAll(filepath.Dir(preexisting), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(preexisting, []byte("keep me"), 0600); err != nil {
					t.Fatal(err)
				}
			}

			prompter := &fakePrompter{answers: tt.answers, err: tt.promptErr}
			var keyGen *fakeKeyGen
			var dbCalls int
			in := NewInitializer(prompter, func(c config.EncryptionConfig) (KeyGenerator, error) {
				keyGen = &fakeKeyGen{cfg: c, fail: tt.keygenFail}
				return keyGen, nil
			}, func(c config.DatabaseConfig, hostID string) error {
				dbCalls++
				if c.DataDir != filepath.Join(env.baseDir, "data") || hostID != "host-1" {
					t.Errorf("createDB(%+v, %q): unexpected arguments", c, hostID)
				}
				return tt.dbErr
			})

			cfg, err := in.Init(InitParams{
				ConfigPath:  env.configPath,
				BaseDir:     env.baseDir,
				HostID:      "host-1",
				FSVaultRoot: tt.fsVault,
			})

			if prompter.calls != tt.wantPrompts {
				t.Errorf("prompts = %d, want %d", prompter.calls, tt.wantPrompts)
			}

			wantFail := tt.wantErr != nil || tt.wantErrText != ""
			if !wantFail {
				if err != nil {
					t.Fatalf("Init() error = %v", err)
				}
				assertMode(t, env.configPath, 0600)
				for _, d := range []string{"", "data", "staging", "log", "keys"} {
					assertMode(t, filepath.Join(env.baseDir, d), 0700)
				}
				if dbCalls != 1 {
					t.Errorf("createDB calls = %d, want 1", dbCalls)
				}
				if keyGen.passphrase != "pw" {
					t.Errorf("Setup passphrase = %q, want %q", keyGen.passphrase, "pw")
				}
				loaded, err := config.ReadFromFile(env.configPath)
				if err != nil {
					t.Fatalf("reading written config: %v", err)
				}
				if loaded.HostID != "host-1" || loaded.Staging.StagingDir != cfg.Staging.StagingDir {
					t.Errorf("written config = %+v, want %+v", loaded, cfg)
				}
				if tt.fsVault != "" {
					want := config.VaultConfig{Type: "filesystem", Name: config.DefaultFSVaultName, FSVaultRoot: tt.fsVault}
					if len(loaded.Vaults) != 1 || loaded.Vaults[0] != want {
						t.Errorf("vaults = %+v, want [%+v]", loaded.Vaults, want)
					}
					if err := loaded.Validate(); err != nil {
						t.Errorf("written config does not validate: %v", err)
					}
				} else if len(loaded.Vaults) != 0 {
					t.Errorf("vaults = %+v, want none", loaded.Vaults)
				}
				return
			}

			if err == nil {
				t.Fatal("Init() error = nil, want error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("Init() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErrText != "" && !strings.Contains(err.Error(), tt.wantErrText) {
				t.Errorf("Init() error = %v, want containing %q", err, tt.wantErrText)
			}
			if cfg != nil {
				t.Errorf("Init() cfg = %+v, want nil on error", cfg)
			}

			// Nothing init could have created may remain, but pre-existing files must.
			for _, p := range []string{env.configPath, filepath.Join(env.baseDir, "keys", "bt.pub"), filepath.Join(env.baseDir, "keys", "bt.key")} {
				if p == preexisting {
					data, err := os.ReadFile(p)
					if err != nil || string(data) != "keep me" {
						t.Errorf("pre-existing %s was changed (data=%q, err=%v)", p, data, err)
					}
					continue
				}
				assertNotExist(t, p)
			}
		})
	}
}

func TestInitializer_Init_RerunAfterFailure(t *testing.T) {
	env := newInitEnv(t)
	newKeyGen := func(c config.EncryptionConfig) (KeyGenerator, error) {
		return &fakeKeyGen{cfg: c}, nil
	}
	noDB := func(config.DatabaseConfig, string) error { return nil }
	params := InitParams{ConfigPath: env.configPath, BaseDir: env.baseDir, HostID: "h"}

	_, err := NewInitializer(&fakePrompter{answers: []string{"pw", "typo"}}, newKeyGen, noDB).Init(params)
	if !errors.Is(err, ErrPassphraseMismatch) {
		t.Fatalf("first Init() error = %v, want %v", err, ErrPassphraseMismatch)
	}

	if _, err := NewInitializer(&fakePrompter{answers: []string{"pw", "pw"}}, newKeyGen, noDB).Init(params); err != nil {
		t.Fatalf("second Init() error = %v", err)
	}
}

func TestInitializer_Init_EncryptorFactoryError(t *testing.T) {
	env := newInitEnv(t)
	prompter := &fakePrompter{}
	in := NewInitializer(prompter, func(config.EncryptionConfig) (KeyGenerator, error) {
		return nil, errors.New("bad encryption type")
	}, func(config.DatabaseConfig, string) error {
		t.Error("createDB called after factory error")
		return nil
	})
	_, err := in.Init(InitParams{ConfigPath: env.configPath, BaseDir: env.baseDir, HostID: "h"})
	if err == nil || !strings.Contains(err.Error(), "bad encryption type") {
		t.Fatalf("Init() error = %v, want factory error", err)
	}
	if prompter.calls != 0 {
		t.Errorf("prompted %d times before failing, want 0", prompter.calls)
	}
	assertNotExist(t, env.configPath)
}
