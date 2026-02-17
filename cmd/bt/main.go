package main

import (
	"fmt"
	"os"
	"path/filepath"

	"bt-go/internal/app"
	"bt-go/internal/config"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// newApp reads the config and creates a BTApp. The caller must defer app.Close().
func newApp() (*app.BTApp, error) {
	defaults, err := app.GetDefaults()
	if err != nil {
		return nil, fmt.Errorf("getting defaults: %w", err)
	}

	cfg, err := config.ReadFromFile(defaults["config_path"])
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	a, err := app.NewBTApp(cfg)
	if err != nil {
		return nil, fmt.Errorf("initializing app: %w", err)
	}

	return a, nil
}

var rootCmd = &cobra.Command{
	Use:   "bt",
	Short: "Personal backup tool",
}

// config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get application defaults
		defaults, err := app.GetDefaults()
		if err != nil {
			return fmt.Errorf("failed to get defaults: %w", err)
		}

		// Generate a new host ID
		hostID := uuid.New().String()

		// Create config with defaults
		cfg := config.NewConfig(hostID, defaults["base_dir"])

		// Initialize config file
		if err := config.Init(defaults["config_path"], cfg); err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		fmt.Printf("Configuration initialized at %s\n", defaults["config_path"])
		fmt.Printf("Host ID: %s\n", hostID)
		fmt.Printf("Base Dir: %s\n", defaults["base_dir"])
		return nil
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "View configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get application defaults
		defaults, err := app.GetDefaults()
		if err != nil {
			return fmt.Errorf("failed to get defaults: %w", err)
		}

		// Read config
		cfg, err := config.ReadFromFile(defaults["config_path"])
		if err != nil {
			return fmt.Errorf("failed to read config: %w", err)
		}

		// Display config
		fmt.Printf("Configuration from %s:\n\n", defaults["config_path"])
		fmt.Printf("Host ID:  %s\n", cfg.HostID)
		fmt.Printf("Base Dir: %s\n", cfg.BaseDir)
		fmt.Printf("Log Dir:  %s\n", cfg.LogDir)
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
		a, err := newApp()
		if err != nil {
			return err
		}
		defer a.Close()

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		if err := a.AddDirectory(cwd); err != nil {
			return fmt.Errorf("tracking directory: %w", err)
		}

		fmt.Printf("Tracking directory: %s\n", cwd)
		return nil
	},
}

var dirStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "View directory status",
	Run: func(cmd *cobra.Command, args []string) {
		cwd, _ := os.Getwd()
		fmt.Printf("Would show status for directory: %s\n", cwd)
	},
}

// add command
var addCmd = &cobra.Command{
	Use:   "add [FILENAME]",
	Short: "Stage files for backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := newApp()
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

		if err := a.StageFile(absTarget); err != nil {
			return fmt.Errorf("staging file: %w", err)
		}

		fmt.Printf("Staged: %s\n", absTarget)
		return nil
	},
}

// backup command
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Execute backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := newApp()
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
	Run: func(cmd *cobra.Command, args []string) {
		filename := args[0]
		fmt.Printf("Would show version history for: %s\n", filename)
	},
}

// restore command
var restoreCmd = &cobra.Command{
	Use:   "restore FILENAME",
	Short: "Restore a file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filename := args[0]
		fmt.Printf("Would restore file: %s\n", filename)
	},
}

func init() {
	// config subcommands
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configVaultCmd)
	configVaultCmd.AddCommand(configVaultInitCmd)

	// dir subcommands
	dirCmd.AddCommand(dirInitCmd)
	dirCmd.AddCommand(dirStatusCmd)

	// root commands
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(dirCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(restoreCmd)
}
