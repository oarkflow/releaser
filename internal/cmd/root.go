/*
Package cmd provides the CLI commands for Releaser.
*/
package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/oarkflow/releaser/internal/deps"
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
	autoInstall  bool
	skipInstall  bool
	silent       bool
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
	rootCmd.PersistentFlags().IntVarP(&parallelism, "parallelism", "p", runtime.NumCPU(), "number of parallel tasks")
	rootCmd.PersistentFlags().StringVar(&timeout, "timeout", "60m", "timeout for the entire release")
	rootCmd.PersistentFlags().BoolVar(&autoInstall, "auto-install", false, "automatically install missing dependencies without prompting")
	rootCmd.PersistentFlags().BoolVar(&skipInstall, "skip-install", false, "skip dependency installation prompts")

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

	// Configure dependency installation behavior
	if autoInstall {
		deps.AutoInstall = true
		deps.PromptForInstall = false
	} else if skipInstall {
		deps.AutoInstall = false
		deps.PromptForInstall = false
	}

	if cfgFile != "" {
		// Use config file from the flag
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Config file not found: %s\n", cfgFile)
			os.Exit(1)
		}
	}
}
