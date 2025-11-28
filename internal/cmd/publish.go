package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oarkflow/releaser/internal/pipeline"
)

var publishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish prepared artifacts",
	Long: `Publish artifacts that were prepared with 'releaser release --prepare'.

This command reads the prepared artifacts from the dist folder
and publishes them to the configured registries and repositories.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		opts := pipeline.ReleaseOptions{
			ConfigFile:  cfgFile,
			Parallelism: parallelism,
			Timeout:     timeout,
		}

		p, err := pipeline.New(ctx, opts)
		if err != nil {
			return fmt.Errorf("failed to create pipeline: %w", err)
		}

		if err := p.Publish(ctx); err != nil {
			return fmt.Errorf("publish failed: %w", err)
		}

		return nil
	},
}

var announceCmd = &cobra.Command{
	Use:   "announce",
	Short: "Announce a prepared release",
	Long: `Announce a release that was prepared with 'releaser release --prepare'.

This sends notifications to configured channels like:
  - Slack
  - Discord
  - Twitter/X
  - Mastodon
  - Email
  - Webhooks`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		opts := pipeline.ReleaseOptions{
			ConfigFile:  cfgFile,
			Parallelism: parallelism,
			Timeout:     timeout,
		}

		p, err := pipeline.New(ctx, opts)
		if err != nil {
			return fmt.Errorf("failed to create pipeline: %w", err)
		}

		if err := p.Announce(ctx); err != nil {
			return fmt.Errorf("announce failed: %w", err)
		}

		return nil
	},
}

var continueCmd = &cobra.Command{
	Use:   "continue",
	Short: "Continue from a prepared release",
	Long: `Continue a release that was prepared with 'releaser release --prepare'.

This will both publish and announce the release in sequence.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		opts := pipeline.ReleaseOptions{
			ConfigFile:  cfgFile,
			Parallelism: parallelism,
			Timeout:     timeout,
		}

		p, err := pipeline.New(ctx, opts)
		if err != nil {
			return fmt.Errorf("failed to create pipeline: %w", err)
		}

		if err := p.Continue(ctx); err != nil {
			return fmt.Errorf("continue failed: %w", err)
		}

		return nil
	},
}
