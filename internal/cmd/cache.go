/*
Package cmd provides cache management commands for Releaser.
*/
package cmd

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/oarkflow/releaser/internal/cache"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage build cache",
	Long: `Manage the build cache for incremental builds.

The cache stores compiled binaries and artifacts to speed up
subsequent builds when source files haven't changed.`,
}

var cacheCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clear the build cache",
	Long:  `Remove all cached artifacts from the build cache.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := cache.New(cache.DefaultOptions())
		if err != nil {
			return err
		}

		if err := c.Clear(); err != nil {
			return fmt.Errorf("failed to clear cache: %w", err)
		}

		log.Info("Cache cleared successfully")
		return nil
	},
}

var cachePruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove expired cache entries",
	Long:  `Remove expired entries from the build cache.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := cache.New(cache.DefaultOptions())
		if err != nil {
			return err
		}

		if err := c.Prune(); err != nil {
			return fmt.Errorf("failed to prune cache: %w", err)
		}

		log.Info("Cache pruned successfully")
		return nil
	},
}

var cacheStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show cache statistics",
	Long:  `Display statistics about the build cache.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := cache.New(cache.DefaultOptions())
		if err != nil {
			return err
		}

		stats := c.Stats()
		if stats == nil {
			fmt.Println("Cache is disabled")
			return nil
		}

		fmt.Printf("Cache Statistics:\n")
		fmt.Printf("  Directory: %v\n", stats["cache_dir"])
		fmt.Printf("  Entries:   %v\n", stats["entries"])
		fmt.Printf("  Expired:   %v\n", stats["expired"])
		fmt.Printf("  Size:      %v\n", stats["size_human"])

		return nil
	},
}

func init() {
	cacheCmd.AddCommand(cacheCleanCmd)
	cacheCmd.AddCommand(cachePruneCmd)
	cacheCmd.AddCommand(cacheStatsCmd)
	rootCmd.AddCommand(cacheCmd)
}
