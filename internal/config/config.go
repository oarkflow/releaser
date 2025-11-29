/*
Package config provides configuration loading and validation for Releaser.
*/
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"dario.cat/mergo"
	"gopkg.in/yaml.v3"
)

// Config represents the complete Releaser configuration
type Config struct {
	// Version of the configuration schema
	Version int `yaml:"version"`

	// ProjectName is the name of the project
	ProjectName string `yaml:"project_name"`

	// Dist is the output directory for artifacts
	Dist string `yaml:"dist,omitempty"`

	// Global defaults
	Defaults Defaults `yaml:"defaults,omitempty"`

	// Environment variables (list of KEY=VALUE strings)
	Env []string `yaml:"env,omitempty"`

	// Custom template variables
	Variables map[string]interface{} `yaml:"variables,omitempty"`

	// Include other configuration files
	Includes []string `yaml:"includes,omitempty"`

	// Before hooks run at the start of the release
	Before Hooks `yaml:"before,omitempty"`

	// After hooks run at the end of the release
	After Hooks `yaml:"after,omitempty"`

	// Git configuration
	Git GitConfig `yaml:"git,omitempty"`

	// Builds configuration
	Builds []Build `yaml:"builds,omitempty"`

	// Archives configuration
	Archives []Archive `yaml:"archives,omitempty"`

	// NFPMs (Linux packages) configuration
	NFPMs []NFPM `yaml:"nfpms,omitempty"`

	// Snapcrafts configuration
	Snapcrafts []Snapcraft `yaml:"snapcrafts,omitempty"`

	// Docker images configuration
	Dockers []Docker `yaml:"dockers,omitempty"`

	// Docker manifests configuration
	DockerManifests []DockerManifest `yaml:"docker_manifests,omitempty"`

	// Homebrew taps configuration
	Brews []Brew `yaml:"brews,omitempty"`

	// Scoop buckets configuration
	Scoops []Scoop `yaml:"scoops,omitempty"`

	// NPM packages configuration
	NPMs []NPM `yaml:"npms,omitempty"`

	// Chocolatey packages configuration
	Chocolateys []Chocolatey `yaml:"chocolateys,omitempty"`

	// macOS App Bundles configuration
	AppBundles []AppBundle `yaml:"app_bundles,omitempty"`

	// macOS DMGs configuration
	DMGs []DMG `yaml:"dmgs,omitempty"`

	// Windows MSI installers configuration
	MSIs []MSI `yaml:"msis,omitempty"`

	// Windows NSIS installers configuration
	NSISs []NSIS `yaml:"nsiss,omitempty"`

	// Signs configuration
	Signs []Sign `yaml:"signs,omitempty"`

	// Docker signs configuration
	DockerSigns []DockerSign `yaml:"docker_signs,omitempty"`

	// Checksum configuration
	Checksum Checksum `yaml:"checksum,omitempty"`

	// Changelog configuration
	Changelog Changelog `yaml:"changelog,omitempty"`

	// Release configuration
	Release Release `yaml:"release,omitempty"`

	// Announce configuration
	Announce Announce `yaml:"announce,omitempty"`

	// Blobs (S3, GCS, Azure) configuration
	Blobs []Blob `yaml:"blobs,omitempty"`

	// Uploads (custom HTTP) configuration
	Uploads []Upload `yaml:"uploads,omitempty"`

	// Publishers configuration
	Publishers []Publisher `yaml:"publishers,omitempty"`

	// Source archive configuration
	Source Source `yaml:"source,omitempty"`

	// SBOMs configuration
	SBOMs []SBOM `yaml:"sboms,omitempty"`

	// Milestones configuration
	Milestones []Milestone `yaml:"milestones,omitempty"`

	// Universal binaries (macOS) configuration
	UniversalBinaries []UniversalBinary `yaml:"universal_binaries,omitempty"`

	// UPX compression configuration
	UPXs []UPX `yaml:"upxs,omitempty"`

	// Winget configuration
	Wingets []Winget `yaml:"wingets,omitempty"`

	// AUR configuration
	AURs []AUR `yaml:"aurs,omitempty"`

	// Krew configuration
	Krews []Krew `yaml:"krews,omitempty"`

	// Ko configuration (for Go container images)
	Kos []Ko `yaml:"kos,omitempty"`

	// Nix configuration
	Nixes []Nix `yaml:"nixes,omitempty"`

	// Fury.io configuration
	Furies []Fury `yaml:"furies,omitempty"`

	// CloudSmith configuration
	CloudSmiths []CloudSmith `yaml:"cloudsmiths,omitempty"`

	// Template files configuration
	TemplateFiles []TemplateFile `yaml:"template_files,omitempty"`

	// Metadata configuration
	Metadata Metadata `yaml:"metadata,omitempty"`

	// Monorepo configuration
	Monorepo Monorepo `yaml:"monorepo,omitempty"`

	// Nightly configuration
	Nightly Nightly `yaml:"nightly,omitempty"`

	// Split configuration for distributed builds
	Split Split `yaml:"split,omitempty"`

	// Prebuilt binaries configuration
	Prebuilt []Prebuilt `yaml:"prebuilt,omitempty"`

	// Flatpak configuration
	Flatpaks []Flatpak `yaml:"flatpaks,omitempty"`

	// AppImage configuration
	AppImages []AppImage `yaml:"appimages,omitempty"`

	// Crates.io configuration
	Crates []Crate `yaml:"crates,omitempty"`

	// PyPI configuration
	PyPIs []PyPI `yaml:"pypis,omitempty"`

	// Maven Central configuration
	Mavens []Maven `yaml:"mavens,omitempty"`

	// NuGet configuration
	NuGets []NuGet `yaml:"nugets,omitempty"`

	// Gem configuration
	Gems []Gem `yaml:"gems,omitempty"`

	// Helm configuration
	Helms []Helm `yaml:"helms,omitempty"`

	// Cosign signing configuration
	Cosigns []Cosign `yaml:"cosigns,omitempty"`

	// Kubernetes manifests configuration
	Kubernetes []Kubernetes `yaml:"kubernetes,omitempty"`

	// Docker Compose configuration
	DockerComposes []DockerCompose `yaml:"docker_composes,omitempty"`
}

// Defaults contains global default values
type Defaults struct {
	// Homepage URL for the project
	Homepage string `yaml:"homepage,omitempty"`

	// Description of the project
	Description string `yaml:"description,omitempty"`

	// License of the project
	License string `yaml:"license,omitempty"`

	// Maintainer of the project
	Maintainer string `yaml:"maintainer,omitempty"`

	// Vendor of the project
	Vendor string `yaml:"vendor,omitempty"`
}

// Hooks represents before/after hooks
type Hooks struct {
	// Commands to run
	Commands []string `yaml:"commands,omitempty"`

	// Hooks with more options
	Hooks []Hook `yaml:"hooks,omitempty"`

	// Before hooks
	Before []Hook `yaml:"before,omitempty"`

	// After hooks
	After []Hook `yaml:"after,omitempty"`
}

// Hook represents a single hook command
type Hook struct {
	// Command to run
	Cmd string `yaml:"cmd"`

	// Directory to run the command in
	Dir string `yaml:"dir,omitempty"`

	// Environment variables
	Env map[string]string `yaml:"env,omitempty"`

	// Output handling
	Output string `yaml:"output,omitempty"`

	// If condition
	If string `yaml:"if,omitempty"`

	// FailFast stops on error
	FailFast bool `yaml:"fail_fast,omitempty"`

	// Shell runs command in shell
	Shell bool `yaml:"shell,omitempty"`
}

// GitConfig contains git-related configuration
type GitConfig struct {
	// TagSort for sorting tags
	TagSort string `yaml:"tag_sort,omitempty"`

	// PrereleaseSuffix for identifying prereleases
	PrereleaseSuffix string `yaml:"prerelease_suffix,omitempty"`

	// IgnoreTags for filtering tags
	IgnoreTags []string `yaml:"ignore_tags,omitempty"`
}

// Build represents a build configuration
type Build struct {
	// ID of the build
	ID string `yaml:"id,omitempty"`

	// Builder to use (go, rust, node, python, prebuilt)
	Builder string `yaml:"builder,omitempty"`

	// Dir is the working directory
	Dir string `yaml:"dir,omitempty"`

	// Main package or entry point
	Main string `yaml:"main,omitempty"`

	// Binary name
	Binary string `yaml:"binary,omitempty"`

	// Flags for the builder
	Flags []string `yaml:"flags,omitempty"`

	// Ldflags for Go builds
	Ldflags []string `yaml:"ldflags,omitempty"`

	// Tags for Go builds
	Tags []string `yaml:"tags,omitempty"`

	// Env for build environment
	Env []string `yaml:"env,omitempty"`

	// Goos target operating systems
	Goos []string `yaml:"goos,omitempty"`

	// Goarch target architectures
	Goarch []string `yaml:"goarch,omitempty"`

	// Goarm versions for ARM builds
	Goarm []string `yaml:"goarm,omitempty"`

	// Goamd64 versions for AMD64 builds
	Goamd64 []string `yaml:"goamd64,omitempty"`

	// Gomips versions for MIPS builds
	Gomips []string `yaml:"gomips,omitempty"`

	// Ignore certain OS/arch combinations
	Ignore []BuildIgnore `yaml:"ignore,omitempty"`

	// Targets to build (alternative to goos/goarch)
	Targets []string `yaml:"targets,omitempty"`

	// Mod for Go modules mode
	Mod string `yaml:"mod,omitempty"`

	// Asmflags for Go builds
	Asmflags []string `yaml:"asmflags,omitempty"`

	// Gcflags for Go builds
	Gcflags []string `yaml:"gcflags,omitempty"`

	// Buildmode for Go builds
	Buildmode string `yaml:"buildmode,omitempty"`

	// ModTimestamp for reproducible builds
	ModTimestamp string `yaml:"mod_timestamp,omitempty"`

	// Skip build
	Skip bool `yaml:"skip,omitempty"`

	// NoUniqueDistDir disables unique dist directories
	NoUniqueDistDir bool `yaml:"no_unique_dist_dir,omitempty"`

	// Hooks for this build
	Hooks BuildHooks `yaml:"hooks,omitempty"`

	// GoBinary to use
	GoBinary string `yaml:"gobinary,omitempty"`

	// Command to run (for custom builders)
	Command string `yaml:"command,omitempty"`

	// NoMainCheck disables main package check
	NoMainCheck bool `yaml:"no_main_check,omitempty"`

	// Overrides for specific targets
	Overrides []BuildOverride `yaml:"overrides,omitempty"`
}

// BuildIgnore represents an ignore rule for builds
type BuildIgnore struct {
	Goos   string `yaml:"goos,omitempty"`
	Goarch string `yaml:"goarch,omitempty"`
	Goarm  string `yaml:"goarm,omitempty"`
	Gomips string `yaml:"gomips,omitempty"`
}

// BuildHooks for pre/post build hooks
type BuildHooks struct {
	Pre  string `yaml:"pre,omitempty"`
	Post string `yaml:"post,omitempty"`
}

// BuildOverride for target-specific overrides
type BuildOverride struct {
	Goos    string   `yaml:"goos,omitempty"`
	Goarch  string   `yaml:"goarch,omitempty"`
	Goarm   string   `yaml:"goarm,omitempty"`
	Goamd64 string   `yaml:"goamd64,omitempty"`
	Env     []string `yaml:"env,omitempty"`
	Flags   []string `yaml:"flags,omitempty"`
	Ldflags []string `yaml:"ldflags,omitempty"`
	Tags    []string `yaml:"tags,omitempty"`
}

// Archive represents archive configuration
type Archive struct {
	ID                        string                  `yaml:"id,omitempty"`
	Builds                    []string                `yaml:"builds,omitempty"`
	Format                    string                  `yaml:"format,omitempty"`
	FormatOverrides           []ArchiveFormatOverride `yaml:"format_overrides,omitempty"`
	NameTemplate              string                  `yaml:"name_template,omitempty"`
	WrapInDirectory           string                  `yaml:"wrap_in_directory,omitempty"`
	StripParentDir            bool                    `yaml:"strip_parent_dir,omitempty"`
	Files                     []ArchiveFile           `yaml:"files,omitempty"`
	Meta                      bool                    `yaml:"meta,omitempty"`
	AllowDifferentBinaryCount bool                    `yaml:"allow_different_binary_count,omitempty"`
	Hooks                     ArchiveHooks            `yaml:"hooks,omitempty"`
	If                        string                  `yaml:"if,omitempty"`
}

// ArchiveFormatOverride for OS-specific formats
type ArchiveFormatOverride struct {
	Goos   string `yaml:"goos"`
	Format string `yaml:"format"`
}

// ArchiveFile represents a file to include in archive
type ArchiveFile struct {
	Src         string          `yaml:"src"`
	Dst         string          `yaml:"dst,omitempty"`
	StripParent bool            `yaml:"strip_parent,omitempty"`
	Info        ArchiveFileInfo `yaml:"info,omitempty"`
}

// UnmarshalYAML allows ArchiveFile to be specified as either a string or object
func (f *ArchiveFile) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		// Simple string format: just the source path
		f.Src = value.Value
		return nil
	}

	// Object format with src, dst, etc.
	type rawArchiveFile ArchiveFile
	return value.Decode((*rawArchiveFile)(f))
}

// ArchiveFileInfo for file metadata
type ArchiveFileInfo struct {
	Owner string      `yaml:"owner,omitempty"`
	Group string      `yaml:"group,omitempty"`
	Mode  os.FileMode `yaml:"mode,omitempty"`
	MTime string      `yaml:"mtime,omitempty"`
}

// ArchiveHooks for archive before/after hooks
type ArchiveHooks struct {
	Before []Hook `yaml:"before,omitempty"`
	After  []Hook `yaml:"after,omitempty"`
}

// Load loads configuration from a file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	data = []byte(os.ExpandEnv(string(data)))

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if cfg.Dist == "" {
		cfg.Dist = "dist"
	}

	// Process includes
	baseDir := filepath.Dir(path)
	for _, include := range cfg.Includes {
		includePath := include
		if !filepath.IsAbs(includePath) {
			includePath = filepath.Join(baseDir, include)
		}

		// Support glob patterns
		matches, err := filepath.Glob(includePath)
		if err != nil {
			return nil, fmt.Errorf("invalid include pattern %s: %w", include, err)
		}

		for _, match := range matches {
			includeCfg, err := Load(match)
			if err != nil {
				return nil, fmt.Errorf("failed to load include %s: %w", match, err)
			}

			if err := mergo.Merge(&cfg, includeCfg, mergo.WithAppendSlice); err != nil {
				return nil, fmt.Errorf("failed to merge include %s: %w", match, err)
			}
		}
	}

	return &cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.ProjectName == "" {
		return fmt.Errorf("project_name is required")
	}

	// Validate builds
	buildIDs := make(map[string]bool)
	for i, build := range c.Builds {
		if build.ID == "" {
			c.Builds[i].ID = fmt.Sprintf("build%d", i)
		}
		if buildIDs[c.Builds[i].ID] {
			return fmt.Errorf("duplicate build ID: %s", c.Builds[i].ID)
		}
		buildIDs[c.Builds[i].ID] = true
	}

	// Validate archives
	for i, archive := range c.Archives {
		if archive.ID == "" {
			c.Archives[i].ID = fmt.Sprintf("archive%d", i)
		}
	}

	// Validate templates in configuration
	if err := c.validateTemplates(); err != nil {
		return err
	}

	return nil
}

// validateTemplates validates all template strings in the configuration
func (c *Config) validateTemplates() error {
	templateRe := regexp.MustCompile(`\{\{.*?\}\}`)

	// Helper to validate a template string
	validateTemplate := func(name, tmpl string) error {
		if !templateRe.MatchString(tmpl) {
			return nil
		}
		_, err := template.New(name).Parse(tmpl)
		if err != nil {
			return fmt.Errorf("invalid template in %s: %w", name, err)
		}
		return nil
	}

	// Validate common template fields
	for i, build := range c.Builds {
		if err := validateTemplate(fmt.Sprintf("builds[%d].binary", i), build.Binary); err != nil {
			return err
		}
	}

	for i, archive := range c.Archives {
		if err := validateTemplate(fmt.Sprintf("archives[%d].name_template", i), archive.NameTemplate); err != nil {
			return err
		}
	}

	return nil
}

// ApplyTemplate applies template variables to a string
func (c *Config) ApplyTemplate(tmpl string, data map[string]interface{}) (string, error) {
	t, err := template.New("").Funcs(templateFuncs()).Parse(tmpl)
	if err != nil {
		return "", err
	}

	// Merge config variables with provided data
	merged := make(map[string]interface{})
	for k, v := range c.Variables {
		merged[k] = v
	}
	for k, v := range data {
		merged[k] = v
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, merged); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// templateFuncs returns common template functions
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"replace":    strings.ReplaceAll,
		"tolower":    strings.ToLower,
		"toupper":    strings.ToUpper,
		"title":      strings.Title,
		"trim":       strings.TrimSpace,
		"trimprefix": strings.TrimPrefix,
		"trimsuffix": strings.TrimSuffix,
		"split":      strings.Split,
		"join":       strings.Join,
		"contains":   strings.Contains,
		"hasprefix":  strings.HasPrefix,
		"hassuffix":  strings.HasSuffix,
		"env":        os.Getenv,
		"default": func(def, val interface{}) interface{} {
			if val == nil || val == "" {
				return def
			}
			return val
		},
	}
}

// DefaultTemplate returns the default configuration template
func DefaultTemplate() string {
	return `# Releaser configuration file
# See https://github.com/oarkflow/releaser for documentation

version: 2

project_name: myproject

# Global defaults
defaults:
  homepage: https://github.com/user/myproject
  description: My awesome project
  license: MIT
  maintainer: user@example.com

# Environment variables
env:
  - CGO_ENABLED=0

# Custom template variables
variables:
  owner: user
  repo: myproject

# Before hooks
before:
  hooks:
    - cmd: go mod tidy
    - cmd: go generate ./...

# Build configuration
builds:
  - id: default
    main: ./cmd/myproject
    binary: myproject
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64

# Archive configuration
archives:
  - id: default
    format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - LICENSE*
      - README*
      - CHANGELOG*

# Checksum configuration
checksum:
  name_template: "checksums.txt"
  algorithm: sha256

# Changelog configuration
changelog:
  use: github
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
  groups:
    - title: Features
      regexp: "^feat:"
    - title: Bug Fixes
      regexp: "^fix:"
    - title: Documentation
      regexp: "^docs:"

# Release configuration
release:
  github:
    owner: "{{ .Env.GITHUB_OWNER }}"
    name: "{{ .Env.GITHUB_REPO }}"
  draft: false
  prerelease: auto
  name_template: "{{ .Tag }}"
`
}
