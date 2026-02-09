package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
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
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Would initialize configuration at ~/.config/bt.toml")
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "View configuration",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Would display current configuration settings")
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
	Run: func(cmd *cobra.Command, args []string) {
		cwd, _ := os.Getwd()
		fmt.Printf("Would track directory: %s\n", cwd)
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
	Run: func(cmd *cobra.Command, args []string) {
		target := "."
		if len(args) > 0 {
			target = args[0]
		}
		cwd, _ := os.Getwd()
		fmt.Printf("Would stage files from: %s (in directory: %s)\n", target, cwd)
	},
}

// backup command
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Execute backup",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Would process all staged operations and back up to vault")
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
