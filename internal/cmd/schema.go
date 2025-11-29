/*
Package cmd provides schema commands for Releaser.
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/oarkflow/releaser/internal/schema"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "JSON Schema utilities",
	Long: `Generate and validate JSON Schema for releaser configuration.

This command provides utilities for working with the JSON Schema
that defines the structure of releaser.yaml configuration files.`,
}

var schemaGenerateCmd = &cobra.Command{
	Use:   "generate [output]",
	Short: "Generate JSON Schema",
	Long: `Generate the JSON Schema for releaser configuration.

If no output file is specified, the schema is written to stdout.

Examples:
  releaser schema generate
  releaser schema generate releaser.schema.json
`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s := schema.GenerateSchema()
		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal schema: %w", err)
		}

		if len(args) > 0 {
			if err := os.WriteFile(args[0], data, 0644); err != nil {
				return fmt.Errorf("failed to write schema: %w", err)
			}
			log.Info("Schema written", "path", args[0])
		} else {
			fmt.Println(string(data))
		}

		return nil
	},
}

var schemaValidateCmd = &cobra.Command{
	Use:   "validate [config]",
	Short: "Validate configuration against schema",
	Long: `Validate a releaser configuration file against the JSON Schema.

Examples:
  releaser schema validate
  releaser schema validate .releaser.yaml
  releaser schema validate path/to/config.yaml
`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := ".releaser.yaml"
		if len(args) > 0 {
			configPath = args[0]
		}

		// Check if file exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("config file not found: %s", configPath)
		}

		result := schema.ValidateConfig(configPath)
		if !result.Valid {
			fmt.Printf("\nValidation failed with %d error(s):\n\n", len(result.Errors))
			for i, err := range result.Errors {
				fmt.Printf("  %d. %s: %s\n", i+1, err.Path, err.Message)
			}
			return fmt.Errorf("configuration is invalid")
		}

		log.Info("Configuration is valid", "path", configPath)
		return nil
	},
}

func init() {
	schemaCmd.AddCommand(schemaGenerateCmd)
	schemaCmd.AddCommand(schemaValidateCmd)
	rootCmd.AddCommand(schemaCmd)
}
