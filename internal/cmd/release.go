package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oarkflow/releaser/internal/pipeline"
)

var (
	prepare bool
)

var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Create a full release",
	Long: `Create a full release by running the entire pipeline.

This includes:
  - Fetching git information
  - Running before hooks
  - Building artifacts
  - Creating archives and packages
  - Signing and notarizing
  - Publishing to registries
  - Announcing the release
  - Running after hooks

Use --prepare to prepare the release without publishing or announcing.
Use --single-target to build for a single architecture locally.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		opts := pipeline.ReleaseOptions{
			ConfigFile:   cfgFile,
			Prepare:      prepare,
			Snapshot:     snapshot,
			Nightly:      nightly,
			SingleTarget: singleTarget,
			SkipPublish:  skipPublish,
			SkipSign:     skipSign,
			SkipDocker:   skipDocker,
			SkipAnnounce: skipAnnounce,
			Clean:        clean,
			Parallelism:  parallelism,
			Timeout:      timeout,
		}

		p, err := pipeline.New(ctx, opts)
		if err != nil {
			return fmt.Errorf("failed to create pipeline: %w", err)
		}

		if err := p.Run(ctx); err != nil {
			return fmt.Errorf("release failed: %w", err)
		}

		return nil
	},
}

func init() {
	releaseCmd.Flags().BoolVar(&prepare, "prepare", false, "prepare release without publishing or announcing")
	releaseCmd.Flags().BoolVar(&snapshot, "snapshot", false, "create a snapshot release (no tag required)")
	releaseCmd.Flags().BoolVar(&nightly, "nightly", false, "create a nightly release")
	releaseCmd.Flags().StringVar(&singleTarget, "single-target", "", "build for a single target (e.g., linux_amd64)")
	releaseCmd.Flags().BoolVar(&skipPublish, "skip-publish", false, "skip publishing artifacts")
	releaseCmd.Flags().BoolVar(&skipSign, "skip-sign", false, "skip signing artifacts")
	releaseCmd.Flags().BoolVar(&skipDocker, "skip-docker", false, "skip Docker builds and publishing")
	releaseCmd.Flags().BoolVar(&skipAnnounce, "skip-announce", false, "skip announcing the release")
	releaseCmd.Flags().BoolVar(&clean, "clean", false, "remove dist folder before building")
}
