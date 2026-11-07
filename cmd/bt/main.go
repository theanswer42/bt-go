package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"bt-go/internal/app"
	"bt-go/internal/bt"
	"bt-go/internal/config"
	"bt-go/internal/database"
	"bt-go/internal/encryption"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var version string

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// newApp reads the config and creates a BTApp. The caller must defer app.Close().
// operation identifies the CLI command being run (e.g. "AddDirectory", "BackupAll").
func newApp(operation string) (*app.BTApp, error) {
	defaults, err := app.GetDefaults()
	if err != nil {
		return nil, fmt.Errorf("getting defaults: %w", err)
	}

	cfg, err := config.Load(defaults["config_path"])
	if err != nil {
		return nil, err
	}

	a, err := app.NewBTApp(cfg, operation)
	if err != nil {
		return nil, fmt.Errorf("initializing app: %w", err)
	}

	return a, nil
}

var rootCmd = &cobra.Command{
	Use:     "bt",
	Short:   "Personal backup tool",
	Version: version,
}

// config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration and generate encryption keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		fsVault, _ := cmd.Flags().GetString("fs-vault")

		defaults, err := app.GetDefaults()
		if err != nil {
			return fmt.Errorf("failed to get defaults: %w", err)
		}

		in := app.NewInitializer(termPrompter{}, func(c config.EncryptionConfig) (app.KeyGenerator, error) {
			return encryption.NewEncryptorFromConfig(c)
		}, database.CreateFromConfig)
		cfg, err := in.Init(app.InitParams{
			ConfigPath:  defaults["config_path"],
			BaseDir:     defaults["base_dir"],
			HostID:      uuid.New().String(),
			FSVaultRoot: fsVault,
		})
		if err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		fmt.Printf("Configuration initialized at %s\n", defaults["config_path"])
		fmt.Printf("Host ID:     %s\n", cfg.HostID)
		fmt.Printf("Base Dir:    %s\n", cfg.BaseDir)
		fmt.Printf("Public key:  %s\n", cfg.Encryption.PublicKeyPath)
		fmt.Printf("Private key: %s\n", cfg.Encryption.PrivateKeyPath)
		if len(cfg.Vaults) == 0 {
			fmt.Printf("\nNote: no vault is configured, so other bt commands will fail until you\n")
			fmt.Printf("add a [[vaults]] entry to %s.\n", defaults["config_path"])
			fmt.Printf("The file ends with these commented examples:\n")
			fmt.Print(config.VaultExamples())
		} else {
			fmt.Printf("Vault:       %s (%s) %s\n", cfg.Vaults[0].Name, cfg.Vaults[0].Type, cfg.Vaults[0].FSVaultRoot)
		}
		return nil
	},
}

// termPrompter reads passphrases from the terminal on stdin without echo.
type termPrompter struct{}

func (termPrompter) PromptPassphrase(prompt string) (string, error) {
	fmt.Print(prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	return string(b), err
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "View configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		defaults, err := app.GetDefaults()
		if err != nil {
			return fmt.Errorf("failed to get defaults: %w", err)
		}

		// Read without validating so an incomplete config can still be inspected.
		cfg, err := config.ReadFromFile(defaults["config_path"])
		if err != nil {
			return fmt.Errorf("failed to read config: %w", err)
		}

		fmt.Printf("Configuration from %s:\n\n", defaults["config_path"])
		if err := config.Format(os.Stdout, cfg); err != nil {
			return fmt.Errorf("printing config: %w", err)
		}
		if err := cfg.Validate(); err != nil {
			fmt.Printf("\nWarning: config is not usable yet: %v\n", err)
		}
		return nil
	},
}

var configVaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Manage vault",
}

var configVaultInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize vault",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Would initialize vault (create bucket structure, verify access)")
	},
}

// dir command
var dirCmd = &cobra.Command{
	Use:   "dir",
	Short: "Manage directories",
}

var dirInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Track current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		encrypted, _ := cmd.Flags().GetBool("encrypted")

		a, err := newApp("AddDirectory")
		if err != nil {
			return err
		}
		defer a.Close()

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		if err := a.AddDirectory(cwd, encrypted); err != nil {
			return fmt.Errorf("tracking directory: %w", err)
		}

		fmt.Printf("Tracking directory: %s\n", cwd)
		return nil
	},
}

var dirStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "View directory status",
	RunE: func(cmd *cobra.Command, args []string) error {
		recursive, _ := cmd.Flags().GetBool("recursive")

		a, err := newApp("GetStatus")
		if err != nil {
			return err
		}
		defer a.Close()

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		statuses, err := a.GetStatus(cwd, recursive)
		if err != nil {
			return err
		}

		if len(statuses) == 0 {
			fmt.Println("No files found.")
			return nil
		}

		for _, s := range statuses {
			var indicator string
			switch {
			case s.IsBackedUp && s.IsModifiedSince && s.IsStaged:
				indicator = "BMS"
			case s.IsBackedUp && s.IsModifiedSince:
				indicator = "BM "
			case s.IsBackedUp && s.IsStaged:
				indicator = "BS "
			case s.IsBackedUp:
				indicator = "B  "
			case s.IsStaged:
				indicator = "S  "
			default:
				indicator = "?  "
			}
			fmt.Printf("%s %s\n", indicator, s.RelativePath)
		}

		return nil
	},
}

// add command
var addCmd = &cobra.Command{
	Use:   "add [PATH]",
	Short: "Stage files for backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		recursive, _ := cmd.Flags().GetBool("recursive")

		a, err := newApp("StageFiles")
		if err != nil {
			return err
		}
		defer a.Close()

		target := "."
		if len(args) > 0 {
			target = args[0]
		}

		absTarget, err := filepath.Abs(target)
		if err != nil {
			return fmt.Errorf("resolving path: %w", err)
		}

		count, err := a.StageFiles(absTarget, recursive)
		if err != nil {
			return fmt.Errorf("staging: %w", err)
		}

		fmt.Printf("Staged %d file(s)\n", count)
		return nil
	},
}

// backup command
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Execute backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := newApp("BackupAll")
		if err != nil {
			return err
		}
		defer a.Close()

		count, err := a.BackupAll()
		if err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}

		fmt.Printf("Backed up %d file(s)\n", count)
		return nil
	},
}

// log command
var logCmd = &cobra.Command{
	Use:   "log FILENAME",
	Short: "View file history",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := newApp("GetFileHistory")
		if err != nil {
			return err
		}
		defer a.Close()

		absPath, err := filepath.Abs(args[0])
		if err != nil {
			return fmt.Errorf("resolving path: %w", err)
		}

		entries, err := a.GetFileHistory(absPath)
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			fmt.Println("No backup history.")
			return nil
		}

		for _, e := range entries {
			current := ""
			if e.IsCurrent {
				current = "  [current]"
			}
			fmt.Printf("%s  %s  %d  mtime:%s%s\n",
				e.ContentChecksum[:12],
				e.BackedUpAt.Format("2006-01-02 15:04:05"),
				e.Size,
				e.ModifiedAt.Format("2006-01-02 15:04:05"),
				current,
			)
		}
		return nil
	},
}

// history command
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View backup operation history",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")

		a, err := newApp("GetHistory")
		if err != nil {
			return err
		}
		defer a.Close()

		ops, err := a.GetHistory(limit)
		if err != nil {
			return err
		}

		if len(ops) == 0 {
			fmt.Println("No backup operations recorded.")
			return nil
		}

		for _, op := range ops {
			duration := ""
			if op.FinishedAt.Valid {
				d := op.FinishedAt.Time.Sub(op.StartedAt)
				duration = d.Truncate(time.Millisecond).String()
			}
			fmt.Printf("#%d  %-15s  %s  %-10s  %s\n",
				op.ID,
				op.Operation,
				op.StartedAt.Format("2006-01-02 15:04:05"),
				op.Status,
				duration,
			)
		}
		return nil
	},
}

// restore command
var restoreCmd = &cobra.Command{
	Use:   "restore FILENAME [CHECKSUM]",
	Short: "Restore a file or directory from backup",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := newApp("Restore")
		if err != nil {
			return err
		}
		defer a.Close()

		var checksum string
		if len(args) > 1 {
			checksum = args[1]
		}

		// Prompt for passphrase once if encryption keys are present.
		// The decryption context is reused for all files in this restore.
		var decryptCtx bt.DecryptionContext
		if a.EncryptionConfigured() {
			fmt.Print("Enter passphrase for decryption: ")
			passphrase, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				return fmt.Errorf("reading passphrase: %w", err)
			}
			decryptCtx, err = a.UnlockEncryption(string(passphrase))
			if err != nil {
				return fmt.Errorf("unlocking encryption: %w", err)
			}
		}

		paths, err := a.RestoreFiles(args[0], checksum, decryptCtx)
		if err != nil {
			return err
		}

		for _, p := range paths {
			fmt.Printf("Restored: %s\n", p)
		}
		return nil
	},
}

func init() {
	// config subcommands
	configCmd.AddCommand(configInitCmd)
	configInitCmd.Flags().String("fs-vault", "", "Configure a filesystem vault rooted at PATH")
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configVaultCmd)
	configVaultCmd.AddCommand(configVaultInitCmd)

	// dir subcommands
	dirCmd.AddCommand(dirInitCmd)
	dirInitCmd.Flags().Bool("encrypted", false, "Encrypt files in this directory on backup")
	dirCmd.AddCommand(dirStatusCmd)
	dirStatusCmd.Flags().BoolP("recursive", "r", false, "Recurse into subdirectories")

	// root commands
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(dirCmd)
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().BoolP("recursive", "r", false, "Recurse into subdirectories")
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(historyCmd)
	historyCmd.Flags().IntP("limit", "n", 50, "Maximum number of operations to show")
	rootCmd.AddCommand(restoreCmd)
}
