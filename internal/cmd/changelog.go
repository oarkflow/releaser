package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/oarkflow/releaser/internal/changelog"
)

var (
	changelogOutput string
	changelogFormat string
	changelogSince  string
	changelogUntil  string
	changelogAI     bool
)

var changelogCmd = &cobra.Command{
	Use:   "changelog",
	Short: "Generate or preview changelog",
	Long: `Generate or preview the changelog for the next release.

This is useful for testing your changelog configuration
and seeing what the release notes will look like.

The changelog can be enhanced with AI to:
  - Improve formatting and readability
  - Group commits by type
  - Generate summaries
  - Fix typos and grammar`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		opts := changelog.Options{
			ConfigFile: cfgFile,
			Since:      changelogSince,
			Until:      changelogUntil,
			UseAI:      changelogAI,
			Format:     changelogFormat,
		}

		gen, err := changelog.New(opts)
		if err != nil {
			return fmt.Errorf("failed to create changelog generator: %w", err)
		}

		log, err := gen.Generate(ctx)
		if err != nil {
			return fmt.Errorf("failed to generate changelog: %w", err)
		}

		if changelogOutput != "" {
			if err := os.WriteFile(changelogOutput, []byte(log), 0644); err != nil {
				return fmt.Errorf("failed to write changelog: %w", err)
			}
			fmt.Printf("Changelog written to %s\n", changelogOutput)
		} else {
			fmt.Println(log)
		}

		return nil
	},
}

func init() {
	changelogCmd.Flags().StringVarP(&changelogOutput, "output", "o", "", "write changelog to file")
	changelogCmd.Flags().StringVar(&changelogFormat, "format", "markdown", "output format (markdown, json, yaml)")
	changelogCmd.Flags().StringVar(&changelogSince, "since", "", "generate changelog since this ref")
	changelogCmd.Flags().StringVar(&changelogUntil, "until", "", "generate changelog until this ref")
	changelogCmd.Flags().BoolVar(&changelogAI, "ai", false, "enhance changelog with AI")
}
