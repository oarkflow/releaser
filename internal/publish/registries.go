// Package publish provides publishing to language-specific package registries.
package publish

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/oarkflow/releaser/internal/artifact"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// CratePublisher publishes Rust crates to crates.io or custom registries
type CratePublisher struct {
	config  config.Crate
	tmplCtx *tmpl.Context
}

// NewCratePublisher creates a new Crate publisher
func NewCratePublisher(cfg config.Crate, tmplCtx *tmpl.Context) *CratePublisher {
	return &CratePublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish publishes the crate
func (p *CratePublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	if p.config.SkipUpload == "true" {
		log.Info("Skipping crate publish")
		return nil
	}

	// Check for cargo
	if _, err := exec.LookPath("cargo"); err != nil {
		return fmt.Errorf("cargo not found in PATH")
	}

	log.Info("Publishing to crates.io")

	args := []string{"publish"}

	if p.config.Registry != "" {
		args = append(args, "--registry", p.config.Registry)
	}

	if p.config.AllowDirty {
		args = append(args, "--allow-dirty")
	}

	if p.config.DryRun {
		args = append(args, "--dry-run")
	}

	if p.config.NoVerify {
		args = append(args, "--no-verify")
	}

	if p.config.AllFeatures {
		args = append(args, "--all-features")
	}

	for _, feat := range p.config.Features {
		args = append(args, "--features", feat)
	}

	if p.config.Jobs > 0 {
		args = append(args, "-j", fmt.Sprintf("%d", p.config.Jobs))
	}

	if p.config.ManifestPath != "" {
		args = append(args, "--manifest-path", p.config.ManifestPath)
	}

	cmd := exec.CommandContext(ctx, "cargo", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set token if provided
	env := os.Environ()
	token := p.config.Token
	if token == "" {
		token = os.Getenv("CARGO_REGISTRY_TOKEN")
	}
	if token != "" {
		env = append(env, "CARGO_REGISTRY_TOKEN="+token)
	}
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cargo publish failed: %w", err)
	}

	log.Info("Crate published successfully")
	return nil
}

// PyPIPublisher publishes Python packages to PyPI
type PyPIPublisher struct {
	config  config.PyPI
	tmplCtx *tmpl.Context
}

// NewPyPIPublisher creates a new PyPI publisher
func NewPyPIPublisher(cfg config.PyPI, tmplCtx *tmpl.Context) *PyPIPublisher {
	return &PyPIPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish publishes to PyPI
func (p *PyPIPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	if p.config.SkipUpload == "true" {
		log.Info("Skipping PyPI publish")
		return nil
	}

	// Try twine first, then fall back to pip
	twine, err := exec.LookPath("twine")
	if err != nil {
		return fmt.Errorf("twine not found in PATH (install with: pip install twine)")
	}

	log.Info("Publishing to PyPI")

	repository := p.config.Repository
	if repository == "" {
		repository = "https://upload.pypi.org/legacy/"
	}

	// Find distribution files
	dists := p.config.Distributions
	if len(dists) == 0 {
		dists = []string{"dist/*"}
	}

	var distFiles []string
	for _, pattern := range dists {
		matches, _ := filepath.Glob(pattern)
		distFiles = append(distFiles, matches...)
	}

	if len(distFiles) == 0 {
		return fmt.Errorf("no distribution files found matching patterns: %v", dists)
	}

	args := []string{"upload", "--repository-url", repository}

	if p.config.SkipExisting {
		args = append(args, "--skip-existing")
	}

	args = append(args, distFiles...)

	cmd := exec.CommandContext(ctx, twine, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set credentials
	env := os.Environ()
	username := p.config.Username
	if username == "" {
		username = os.Getenv("TWINE_USERNAME")
	}
	if username == "" {
		username = "__token__"
	}

	password := p.config.Password
	if password == "" {
		password = os.Getenv("TWINE_PASSWORD")
		if password == "" {
			password = os.Getenv("PYPI_TOKEN")
		}
	}

	if username != "" {
		env = append(env, "TWINE_USERNAME="+username)
	}
	if password != "" {
		env = append(env, "TWINE_PASSWORD="+password)
	}
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("twine upload failed: %w", err)
	}

	log.Info("PyPI package published successfully")
	return nil
}

// MavenPublisher publishes Java packages to Maven Central
type MavenPublisher struct {
	config  config.Maven
	tmplCtx *tmpl.Context
}

// NewMavenPublisher creates a new Maven publisher
func NewMavenPublisher(cfg config.Maven, tmplCtx *tmpl.Context) *MavenPublisher {
	return &MavenPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish publishes to Maven Central
func (p *MavenPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	if p.config.SkipUpload == "true" {
		log.Info("Skipping Maven publish")
		return nil
	}

	// Try Maven first, then Gradle
	mvn, mvnErr := exec.LookPath("mvn")
	gradle, gradleErr := exec.LookPath("gradle")

	if mvnErr != nil && gradleErr != nil {
		return fmt.Errorf("neither mvn nor gradle found in PATH")
	}

	log.Info("Publishing to Maven repository")

	var cmd *exec.Cmd
	if mvnErr == nil {
		// Use Maven
		args := []string{"deploy", "-DskipTests"}
		if p.config.Repository != "" {
			args = append(args, "-DaltDeploymentRepository=release::"+p.config.Repository)
		}
		cmd = exec.CommandContext(ctx, mvn, args...)
	} else {
		// Use Gradle
		args := []string{"publish"}
		cmd = exec.CommandContext(ctx, gradle, args...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set credentials
	env := os.Environ()
	if p.config.Username != "" {
		env = append(env, "MAVEN_USERNAME="+p.config.Username)
	}
	if p.config.Password != "" {
		env = append(env, "MAVEN_PASSWORD="+p.config.Password)
	}
	if p.config.GPGPassphrase != "" {
		env = append(env, "GPG_PASSPHRASE="+p.config.GPGPassphrase)
	}
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("maven deploy failed: %w", err)
	}

	log.Info("Maven package published successfully")
	return nil
}

// NuGetPublisher publishes .NET packages to NuGet
type NuGetPublisher struct {
	config  config.NuGet
	tmplCtx *tmpl.Context
}

// NewNuGetPublisher creates a new NuGet publisher
func NewNuGetPublisher(cfg config.NuGet, tmplCtx *tmpl.Context) *NuGetPublisher {
	return &NuGetPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish publishes to NuGet
func (p *NuGetPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	if p.config.SkipUpload == "true" {
		log.Info("Skipping NuGet publish")
		return nil
	}

	dotnet, err := exec.LookPath("dotnet")
	if err != nil {
		return fmt.Errorf("dotnet not found in PATH")
	}

	log.Info("Publishing to NuGet")

	source := p.config.Source
	if source == "" {
		source = "https://api.nuget.org/v3/index.json"
	}

	apiKey := p.config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("NUGET_API_KEY")
	}

	// Find .nupkg files
	nupkgs, _ := filepath.Glob("**/*.nupkg")
	if len(nupkgs) == 0 {
		nupkgs, _ = filepath.Glob("*.nupkg")
	}

	for _, pkg := range nupkgs {
		args := []string{"nuget", "push", pkg, "--source", source}
		if apiKey != "" {
			args = append(args, "--api-key", apiKey)
		}

		cmd := exec.CommandContext(ctx, dotnet, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("nuget push failed for %s: %w", pkg, err)
		}
	}

	log.Info("NuGet package published successfully")
	return nil
}

// GemPublisher publishes Ruby gems
type GemPublisher struct {
	config  config.Gem
	tmplCtx *tmpl.Context
}

// NewGemPublisher creates a new Gem publisher
func NewGemPublisher(cfg config.Gem, tmplCtx *tmpl.Context) *GemPublisher {
	return &GemPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish publishes to RubyGems
func (p *GemPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	if p.config.SkipUpload == "true" {
		log.Info("Skipping Gem publish")
		return nil
	}

	gem, err := exec.LookPath("gem")
	if err != nil {
		return fmt.Errorf("gem not found in PATH")
	}

	log.Info("Publishing to RubyGems")

	// Build gem first if gemspec provided
	if p.config.Gemspec != "" {
		buildCmd := exec.CommandContext(ctx, gem, "build", p.config.Gemspec)
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("gem build failed: %w", err)
		}
	}

	// Find .gem files
	gems, _ := filepath.Glob("*.gem")
	if len(gems) == 0 {
		return fmt.Errorf("no .gem files found")
	}

	apiKey := p.config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("GEM_HOST_API_KEY")
	}

	for _, gemFile := range gems {
		args := []string{"push", gemFile}
		if p.config.Host != "" {
			args = append(args, "--host", p.config.Host)
		}
		if apiKey != "" {
			args = append(args, "--key", apiKey)
		}

		cmd := exec.CommandContext(ctx, gem, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("gem push failed for %s: %w", gemFile, err)
		}
	}

	log.Info("Gem published successfully")
	return nil
}

// HelmPublisher publishes Helm charts
type HelmPublisher struct {
	config  config.Helm
	tmplCtx *tmpl.Context
}

// NewHelmPublisher creates a new Helm publisher
func NewHelmPublisher(cfg config.Helm, tmplCtx *tmpl.Context) *HelmPublisher {
	return &HelmPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish publishes Helm chart
func (p *HelmPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	if p.config.SkipUpload == "true" {
		log.Info("Skipping Helm publish")
		return nil
	}

	helm, err := exec.LookPath("helm")
	if err != nil {
		return fmt.Errorf("helm not found in PATH")
	}

	log.Info("Publishing Helm chart")

	chartPath := p.config.ChartPath
	if chartPath == "" {
		chartPath = "chart"
	}

	// Package the chart
	version := p.tmplCtx.Get("Version")
	packageArgs := []string{"package", chartPath}
	if version != "" {
		packageArgs = append(packageArgs, "--version", version)
	}
	if p.config.AppVersion != "" {
		packageArgs = append(packageArgs, "--app-version", p.config.AppVersion)
	}

	packageCmd := exec.CommandContext(ctx, helm, packageArgs...)
	packageCmd.Stdout = os.Stdout
	packageCmd.Stderr = os.Stderr
	if err := packageCmd.Run(); err != nil {
		return fmt.Errorf("helm package failed: %w", err)
	}

	// Push to repository if configured
	if p.config.Repository != "" {
		// Find the packaged chart
		charts, _ := filepath.Glob("*.tgz")
		if len(charts) == 0 {
			return fmt.Errorf("no packaged charts found")
		}

		for _, chart := range charts {
			pushArgs := []string{"push", chart, p.config.Repository}

			pushCmd := exec.CommandContext(ctx, helm, pushArgs...)
			pushCmd.Stdout = os.Stdout
			pushCmd.Stderr = os.Stderr

			env := os.Environ()
			if p.config.Username != "" {
				env = append(env, "HELM_REPO_USERNAME="+p.config.Username)
			}
			if p.config.Password != "" {
				env = append(env, "HELM_REPO_PASSWORD="+p.config.Password)
			}
			pushCmd.Env = env

			if err := pushCmd.Run(); err != nil {
				return fmt.Errorf("helm push failed for %s: %w", chart, err)
			}
		}
	}

	log.Info("Helm chart published successfully")
	return nil
}
