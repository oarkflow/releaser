/*
Package releaser provides a robust release automation tool similar to GoReleaser
that supports multiple programming languages and extensive customization options.

Releaser is designed to automate the entire release process including:
  - Building artifacts for multiple platforms and architectures
  - Creating archives, packages, and installers
  - Signing and notarizing binaries
  - Publishing to various registries and package managers
  - Generating and enhancing release notes with AI
  - Managing changelogs with advanced filtering

# Configuration

Releaser uses a YAML configuration file (.releaser.yaml) to define the release pipeline.
The configuration supports:
  - Template variables using Go's text/template syntax
  - Include statements for reusing configuration
  - Global defaults for common settings
  - Conditional artifact filtering

# Usage

Basic usage:

	releaser release              # Run the full release pipeline
	releaser release --prepare    # Prepare release without publishing
	releaser publish              # Publish prepared artifacts
	releaser announce             # Announce the release
	releaser continue             # Continue from a prepared release
	releaser changelog            # Preview changelog
	releaser build                # Build artifacts only

For more information, see the documentation at https://github.com/oarkflow/releaser
*/
package releaser

// Version is the current version of Releaser
const Version = "1.0.0"

// BuildDate is set at build time
var BuildDate string

// GitCommit is set at build time
var GitCommit string
