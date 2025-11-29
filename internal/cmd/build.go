package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oarkflow/releaser/internal/pipeline"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build artifacts only",
	Long: `Build artifacts without publishing or announcing.

This is useful for testing your build configuration locally
or in CI before creating an actual release.

This command builds binaries, creates archives, generates packages
(deb/rpm/apk), and creates checksums - everything except publish and announce.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		opts := pipeline.ReleaseOptions{
			ConfigFile:   cfgFile,
			Snapshot:     snapshot,
			SingleTarget: singleTarget,
			SkipPublish:  true,
			SkipAnnounce: true,
			SkipDocker:   skipDocker,
			SkipSign:     skipSign,
			Clean:        clean,
			Parallelism:  parallelism,
			Timeout:      timeout,
		}

		p, err := pipeline.New(ctx, opts)
		if err != nil {
			return fmt.Errorf("failed to create pipeline: %w", err)
		}

		if err := p.BuildAll(ctx); err != nil {
			return fmt.Errorf("build failed: %w", err)
		}

		return nil
	},
}

func init() {
	buildCmd.Flags().BoolVar(&snapshot, "snapshot", false, "create a snapshot build")
	buildCmd.Flags().StringVar(&singleTarget, "single-target", "", "build for a single target")
	buildCmd.Flags().BoolVar(&clean, "clean", false, "remove dist folder before building")
	buildCmd.Flags().BoolVar(&skipDocker, "skip-docker", false, "skip building Docker images")
	buildCmd.Flags().BoolVar(&skipSign, "skip-sign", false, "skip signing artifacts")
}
