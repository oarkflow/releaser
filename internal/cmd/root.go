/*
Package cmd provides the CLI commands for Releaser.
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

var (
	cfgFile      string
	verbose      bool
	debug        bool
	parallelism  int
	timeout      string
	skipPublish  bool
	skipSign     bool
	skipDocker   bool
	skipAnnounce bool
	clean        bool
	snapshot     bool
	nightly      bool
	singleTarget string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "releaser",
	Short: "A robust release automation tool",
	Long: `Releaser is a powerful release automation tool that supports
multiple programming languages and extensive customization options.

It can build artifacts, create packages, sign binaries, publish to
various registries, and generate release notes with AI enhancement.

Example:
  releaser release              # Run full release pipeline
  releaser release --prepare    # Prepare without publishing
  releaser build --snapshot     # Build snapshot version
  releaser changelog            # Preview changelog`,
	SilenceUsage: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is .releaser.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug output")
	rootCmd.PersistentFlags().IntVarP(&parallelism, "parallelism", "p", 4, "number of parallel tasks")
	rootCmd.PersistentFlags().StringVar(&timeout, "timeout", "60m", "timeout for the entire release")

	// Add subcommands
	rootCmd.AddCommand(releaseCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(publishCmd)
	rootCmd.AddCommand(announceCmd)
	rootCmd.AddCommand(continueCmd)
	rootCmd.AddCommand(changelogCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(versionCmd)
}

func initConfig() {
	if debug {
		log.SetLevel(log.DebugLevel)
	} else if verbose {
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(log.WarnLevel)
	}

	if cfgFile != "" {
		// Use config file from the flag
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Config file not found: %s\n", cfgFile)
			os.Exit(1)
		}
	}
}
