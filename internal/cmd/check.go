package cmd

import (
	"fmt"
	"os"

	"github.com/oarkflow/releaser"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check configuration file",
	Long: `Check if the configuration file is valid.

This validates:
  - YAML syntax
  - Required fields
  - Template syntax
  - File references
  - Include statements`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := cfgFile
		if configPath == "" {
			configPath = ".releaser.yaml"
		}

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("config file not found: %s", configPath)
		}

		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("config validation failed: %w", err)
		}

		fmt.Printf("✓ Configuration file %s is valid\n", configPath)
		return nil
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new configuration file",
	Long: `Initialize a new .releaser.yaml configuration file.

This creates a basic configuration file that you can customize
for your project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := ".releaser.yaml"
		if cfgFile != "" {
			configPath = cfgFile
		}

		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("config file already exists: %s", configPath)
		}

		template := config.DefaultTemplate()
		if err := os.WriteFile(configPath, []byte(template), 0644); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		fmt.Printf("✓ Created %s\n", configPath)
		fmt.Println("\nEdit this file to customize your release configuration.")
		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print the version, commit, and build date of Releaser.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Releaser %s\n", releaser.Version)
		if releaser.GitCommit != "" {
			fmt.Printf("  Commit: %s\n", releaser.GitCommit)
		}
		if releaser.BuildDate != "" {
			fmt.Printf("  Built:  %s\n", releaser.BuildDate)
		}
	},
}
