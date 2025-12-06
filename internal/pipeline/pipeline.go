/*
Package pipeline provides the release pipeline orchestration for Releaser.
*/
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/charmbracelet/log"

	"github.com/oarkflow/releaser/internal/announce"
	"github.com/oarkflow/releaser/internal/archive"
	"github.com/oarkflow/releaser/internal/artifact"
	"github.com/oarkflow/releaser/internal/builder"
	"github.com/oarkflow/releaser/internal/cache"
	"github.com/oarkflow/releaser/internal/checksum"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/deps"
	"github.com/oarkflow/releaser/internal/docker"
	"github.com/oarkflow/releaser/internal/git"
	"github.com/oarkflow/releaser/internal/nfpm"
	"github.com/oarkflow/releaser/internal/packaging"
	"github.com/oarkflow/releaser/internal/publish"
	"github.com/oarkflow/releaser/internal/sign"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// ReleaseOptions contains options for the release pipeline
type ReleaseOptions struct {
	ConfigFile   string
	Prepare      bool
	Snapshot     bool
	Nightly      bool
	SingleTarget string
	SkipPublish  bool
	SkipSign     bool
	SkipDocker   bool
	SkipAnnounce bool
	SkipCache    bool
	Clean        bool
	Parallelism  int
	Timeout      string
	Silent       bool
}

// Pipeline orchestrates the release process
type Pipeline struct {
	config      *config.Config
	options     ReleaseOptions
	artifacts   *artifact.Manager
	gitInfo     *git.Info
	templateCtx *tmpl.Context
	buildCache  *cache.BuildCache
	distDir     string
	startTime   time.Time
	mu          sync.Mutex
}

// New creates a new release pipeline
func New(ctx context.Context, opts ReleaseOptions) (*Pipeline, error) {
	// Load configuration
	cfgPath := opts.ConfigFile
	if cfgPath == "" {
		cfgPath = findConfigFile()
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Get git information
	gitInfo, err := git.GetInfo(ctx)
	if err != nil && !opts.Snapshot {
		return nil, fmt.Errorf("failed to get git info: %w", err)
	}

	// Create template context
	templateCtx := tmpl.New(cfg, gitInfo, opts.Snapshot, opts.Nightly)

	// Create artifact manager
	artifacts := artifact.NewManager()

	// Determine dist directory
	distDir := cfg.Dist
	if !filepath.IsAbs(distDir) {
		cwd, _ := os.Getwd()
		distDir = filepath.Join(cwd, distDir)
	}

	// Initialize build cache if not skipped
	var buildCache *cache.BuildCache
	if !opts.SkipCache {
		cacheOpts := cache.DefaultOptions()
		buildCache, _ = cache.NewBuildCache(cacheOpts)
		if buildCache != nil {
			log.Debug("Build cache initialized", "dir", cacheOpts.Dir)
		}
	}

	return &Pipeline{
		config:      cfg,
		options:     opts,
		artifacts:   artifacts,
		gitInfo:     gitInfo,
		templateCtx: templateCtx,
		buildCache:  buildCache,
		distDir:     distDir,
		startTime:   time.Now(),
	}, nil
}

// Run executes the full release pipeline
func (p *Pipeline) Run(ctx context.Context) error {
	log.Info("Starting release pipeline", "project", p.config.ProjectName)

	// Run before hooks
	if err := p.runHooks(ctx, p.config.Before, "before"); err != nil {
		return err
	}

	// Build all artifacts (binaries, archives, packages, checksums, docker)
	if err := p.BuildAll(ctx); err != nil {
		return err
	}

	// If prepare mode, stop here
	if p.options.Prepare {
		if err := p.saveState(); err != nil {
			return err
		}
		log.Info("Release prepared. Use 'releaser publish' and 'releaser announce' to continue.")
		return nil
	}

	// Publish artifacts
	if !p.options.SkipPublish {
		if err := p.Publish(ctx); err != nil {
			return err
		}
	}

	// Announce release
	if !p.options.SkipAnnounce {
		if err := p.Announce(ctx); err != nil {
			return err
		}
	}

	// Run after hooks
	if err := p.runHooks(ctx, p.config.After, "after"); err != nil {
		return err
	}

	elapsed := time.Since(p.startTime)
	log.Info("Release completed successfully", "duration", elapsed.Round(time.Second))

	return nil
}

// BuildAll builds all artifacts including archives, packages, checksums, and docker images
func (p *Pipeline) BuildAll(ctx context.Context) error {
	var allErrors []error

	if p.config.CleanupDistDirs {
		defer func() {
			if err := p.cleanupDistDirs(); err != nil {
				log.Warn("Failed to remove dist folders", "error", err)
			}
		}()
	}

	// Clean dist directory if requested
	if p.options.Clean {
		if err := p.clean(); err != nil {
			allErrors = append(allErrors, fmt.Errorf("clean failed: %w", err))
		}
	}

	// Create dist directory
	if err := os.MkdirAll(p.distDir, 0755); err != nil {
		allErrors = append(allErrors, fmt.Errorf("failed to create dist directory: %w", err))
		// If we can't create dist dir, we can't continue
		return fmt.Errorf("setup failed: %v", allErrors)
	}

	// Build artifacts
	if err := p.Build(ctx); err != nil {
		allErrors = append(allErrors, err)
	}

	// Clean up temporary object files
	_ = os.Remove("-" + ".o")

	// Create archives
	if err := p.archive(ctx); err != nil {
		allErrors = append(allErrors, err)
	}

	// Create packages (nfpm, snapcraft, etc.)
	if err := p.packages(ctx); err != nil {
		allErrors = append(allErrors, err)
	}

	// Create platform-specific packages (macOS, Windows)
	if err := p.platformPackages(ctx); err != nil {
		allErrors = append(allErrors, err)
	}

	// Create checksums
	if err := p.checksum(ctx); err != nil {
		allErrors = append(allErrors, err)
	}

	// Sign artifacts
	if !p.options.SkipSign {
		if err := p.sign(ctx); err != nil {
			allErrors = append(allErrors, err)
		}
	}

	// Build Docker images
	if !p.options.SkipDocker {
		if err := p.docker(ctx); err != nil {
			allErrors = append(allErrors, err)
		}

		if err := p.dockerExports(); err != nil {
			allErrors = append(allErrors, err)
		}
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("build pipeline completed with %d errors: %v", len(allErrors), allErrors)
	}

	log.Info("Build pipeline completed successfully")
	return nil
}

// Build builds all configured artifacts
func (p *Pipeline) Build(ctx context.Context) error {
	log.Info("Building artifacts")

	if len(p.config.Builds) == 0 {
		log.Warn("No builds configured")
		return nil
	}

	// Check and install required dependencies
	if err := p.ensureBuildDependencies(); err != nil {
		return fmt.Errorf("dependency check failed: %w", err)
	}

	// Create a build context with overall timeout to prevent deadlocks
	var buildCtx context.Context
	var cancel context.CancelFunc
	if p.options.Timeout != "" {
		timeout, err := time.ParseDuration(p.options.Timeout)
		if err != nil {
			// Default timeout if parsing fails
			timeout = 2 * time.Hour
		}
		buildCtx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		// Default 2 hour timeout for builds
		buildCtx, cancel = context.WithTimeout(ctx, 2*time.Hour)
	}
	defer cancel()

	// Filter targets if single-target mode
	targets := p.getTargets()
	if p.options.SingleTarget != "" {
		filtered := []BuildTarget{}
		for _, t := range targets {
			if t.String() == p.options.SingleTarget {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("target %s not found", p.options.SingleTarget)
		}
		targets = filtered
	}

	// Build each target
	sem := make(chan struct{}, p.options.Parallelism)
	errCh := make(chan error, len(targets))
	var wg sync.WaitGroup

	// Use the build context with timeout for all operations
	ctx = buildCtx

	for _, build := range p.config.Builds {
		if build.Skip {
			continue
		}

		for _, target := range targets {
			if !p.shouldBuild(build, target) {
				continue
			}

			wg.Add(1)
			go func(b config.Build, t BuildTarget) {
				defer wg.Done()

				// Create a timeout context for this build to prevent deadlock
				buildCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
				defer cancel()

				// Acquire semaphore with context awareness to prevent deadlock
				select {
				case sem <- struct{}{}:
					// Successfully acquired semaphore slot
				case <-buildCtx.Done():
					// Context cancelled while waiting for semaphore
					errCh <- fmt.Errorf("build %s for %s cancelled while waiting for resources: %w", b.ID, t.String(), buildCtx.Err())
					return
				}

				// Ensure semaphore is released
				defer func() { <-sem }()

				if err := p.buildTarget(buildCtx, b, t); err != nil {
					if p.options.Silent {
						log.Error(fmt.Sprintf("Build failed for %s %s: %s", b.ID, t.String(), err.Error()))
					} else {
						select {
						case errCh <- fmt.Errorf("build %s for %s failed: %w", b.ID, t.String(), err):
						case <-ctx.Done():
						}
					}
				}
			}(build, target)
		}
	}

	wg.Wait()
	close(errCh)

	// Collect errors with timeout to prevent deadlock
	var errs []error
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use select with timeout to prevent hanging on channel read
	for {
		select {
		case err, ok := <-errCh:
			if !ok {
				// Channel closed, we're done
				goto collect_done
			}
			errs = append(errs, err)
		case <-timeoutCtx.Done():
			log.Warn("Timeout waiting for error channel, some errors may not be collected")
			goto collect_done
		}
	}

collect_done:

	if len(errs) > 0 {
		if p.options.Silent {
			log.Warn("Some builds failed", "count", len(errs))
		} else {
			return fmt.Errorf("build failed with %d errors: %v", len(errs), errs)
		}
	}

	log.Info("Build completed", "artifacts", p.artifacts.Count())
	return nil
}

// Publish publishes all artifacts
func (p *Pipeline) Publish(ctx context.Context) error {
	log.Info("Publishing artifacts")

	// Load state if continuing from prepare
	if err := p.loadState(); err != nil {
		log.Debug("No saved state found, using current artifacts")
	}

	// Publish to release platforms
	if err := p.publishRelease(ctx); err != nil {
		return err
	}

	// Publish Docker images
	if !p.options.SkipDocker {
		if err := p.publishDocker(ctx); err != nil {
			return err
		}
	}

	// Publish to package managers
	if err := p.publishPackages(ctx); err != nil {
		return err
	}

	log.Info("Publishing completed")
	return nil
}

// Announce announces the release
func (p *Pipeline) Announce(ctx context.Context) error {
	log.Info("Announcing release")

	// Load state if continuing from prepare
	if err := p.loadState(); err != nil {
		log.Debug("No saved state found")
	}

	// Run announcements
	if err := p.runAnnouncements(ctx); err != nil {
		return err
	}

	log.Info("Announcement completed")
	return nil
}

// Continue continues from a prepared release
func (p *Pipeline) Continue(ctx context.Context) error {
	if err := p.Publish(ctx); err != nil {
		return err
	}
	return p.Announce(ctx)
}

// BuildTarget represents a build target (OS/arch combination)
type BuildTarget struct {
	OS    string
	Arch  string
	Arm   string
	Amd64 string
	Mips  string
}

// String returns the target as a string
func (t BuildTarget) String() string {
	s := t.OS + "_" + t.Arch
	if t.Arm != "" {
		s += "_" + t.Arm
	}
	if t.Amd64 != "" {
		s += "_" + t.Amd64
	}
	return s
}

// findConfigFile looks for a configuration file
func findConfigFile() string {
	candidates := []string{
		".releaser.yaml",
		".releaser.yml",
		"releaser.yaml",
		"releaser.yml",
		".goreleaser.yaml",
		".goreleaser.yml",
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	return ".releaser.yaml"
}

// clean removes the dist directory
func (p *Pipeline) clean() error {
	log.Info("Cleaning dist directory", "path", p.distDir)
	return os.RemoveAll(p.distDir)
}

// cleanupDistDirs removes any directories left inside the dist folder, leaving only final artifacts.
func (p *Pipeline) cleanupDistDirs() error {
	entries, err := os.ReadDir(p.distDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := os.RemoveAll(filepath.Join(p.distDir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// runHooks runs before/after hooks
func (p *Pipeline) runHooks(ctx context.Context, hooks config.Hooks, phase string) error {
	// Run simple commands
	for _, cmd := range hooks.Commands {
		log.Debug("Running hook", "phase", phase, "cmd", cmd)
		if err := p.runCommand(ctx, cmd, ""); err != nil {
			return fmt.Errorf("hook %s failed: %w", cmd, err)
		}
	}

	// Run hooks with options
	for _, hook := range hooks.Hooks {
		log.Debug("Running hook", "phase", phase, "cmd", hook.Cmd)
		if err := p.runCommand(ctx, hook.Cmd, hook.Dir); err != nil {
			return fmt.Errorf("hook %s failed: %w", hook.Cmd, err)
		}
	}

	return nil
}

// runCommand runs a shell command
func (p *Pipeline) runCommand(ctx context.Context, cmd, dir string) error {
	// Template the command
	cmd, err := p.templateCtx.Apply(cmd)
	if err != nil {
		return err
	}

	log.Debug("Running command", "cmd", cmd, "dir", dir)

	// Determine working directory
	workDir := dir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Get shell from environment
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	// Run command through shell
	execCmd := exec.CommandContext(ctx, shell, "-c", cmd)
	execCmd.Dir = workDir
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Env = os.Environ()

	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// getTargets returns all build targets
func (p *Pipeline) getTargets() []BuildTarget {
	var targets []BuildTarget

	// Default targets if none specified
	defaultGoos := []string{"linux", "darwin", "windows"}
	defaultGoarch := []string{"amd64", "arm64"}

	for _, goos := range defaultGoos {
		for _, goarch := range defaultGoarch {
			targets = append(targets, BuildTarget{
				OS:   goos,
				Arch: goarch,
			})
		}
	}

	return targets
}

// shouldBuild checks if a target should be built
func (p *Pipeline) shouldBuild(build config.Build, target BuildTarget) bool {
	// Check OS filter
	if len(build.Goos) > 0 {
		found := false
		for _, goos := range build.Goos {
			if goos == target.OS {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check arch filter
	if len(build.Goarch) > 0 {
		found := false
		for _, goarch := range build.Goarch {
			if goarch == target.Arch {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check ignore rules
	for _, ignore := range build.Ignore {
		if ignore.Goos == target.OS && ignore.Goarch == target.Arch {
			return false
		}
	}

	return true
}

// buildTarget builds a single target
func (p *Pipeline) buildTarget(ctx context.Context, build config.Build, target BuildTarget) error {
	log.Info("Starting build", "build", build.ID, "target", target.String(), "builder", build.Builder)

	// Create output directory
	outputDir := filepath.Join(p.distDir, build.ID+"_"+target.String())
	log.Debug("Creating output directory", "path", outputDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Determine binary name
	binary := build.Binary
	if binary == "" {
		binary = p.config.ProjectName
		log.Debug("Using project name as binary name", "name", binary)
	}
	if target.OS == "windows" {
		binary += ".exe"
		log.Debug("Adding Windows extension", "binary", binary)
	}

	// Template the binary name
	log.Debug("Templating binary name", "template", binary)
	binary, err := p.templateCtx.Apply(binary)
	if err != nil {
		return fmt.Errorf("failed to template binary name: %w", err)
	}
	log.Debug("Final binary name", "name", binary)

	outputPath := filepath.Join(outputDir, binary)
	log.Debug("Output path", "path", outputPath)

	// Check build cache
	cacheKey := ""
	if p.buildCache != nil && !p.options.SkipCache {
		log.Debug("Checking build cache")

		// Generate cache key from build config, target, and source hash
		sourcePatterns := []string{"*.go", "go.mod", "go.sum"}
		if build.Builder == "rust" {
			sourcePatterns = []string{"*.rs", "Cargo.toml", "Cargo.lock"}
		}
		log.Debug("Generating cache key", "patterns", sourcePatterns)
		cacheKey = p.buildCache.BuildKey(target.OS, target.Arch, binary, sourcePatterns)

		// Check if we have a cached binary
		if cachedPath, found := p.buildCache.GetBinary(cacheKey); found {
			log.Info("Cache hit - using cached binary", "target", target.String(), "cache_key", cacheKey)
			if err := copyFile(cachedPath, outputPath); err == nil {
				// Register artifact
				p.mu.Lock()
				p.artifacts.Add(artifact.Artifact{
					Name:    binary,
					Path:    outputPath,
					Type:    artifact.TypeBinary,
					Goos:    target.OS,
					Goarch:  target.Arch,
					Goarm:   target.Arm,
					BuildID: build.ID,
					Extra: map[string]interface{}{
						"cached": true,
					},
				})
				p.mu.Unlock()
				log.Info("Build completed using cache", "build", build.ID, "target", target.String())
				return nil
			} else {
				log.Warn("Failed to copy cached binary, proceeding with fresh build", "error", err)
			}
		} else {
			log.Debug("Cache miss - no cached binary found", "cache_key", cacheKey)
		}
	} else {
		log.Debug("Build cache disabled or not available")
	}

	log.Info("Building from source", "build", build.ID, "target", target.String(), "builder", build.Builder)

	// Build based on builder type
	var buildErr error
	switch build.Builder {
	case "", "go":
		log.Debug("Using Go builder")
		buildErr = p.buildGo(ctx, build, target, outputPath)
	case "rust":
		log.Debug("Using Rust builder")
		buildErr = p.buildRust(ctx, build, target, outputPath)
	case "prebuilt":
		log.Debug("Using prebuilt builder")
		buildErr = p.copyPrebuilt(ctx, build, target, outputPath)
	default:
		buildErr = fmt.Errorf("unknown builder: %s", build.Builder)
	}

	if buildErr != nil {
		log.Error("Build failed", "build", build.ID, "target", target.String(), "error", buildErr)
		return fmt.Errorf("build failed: %w", buildErr)
	}

	// Cache the built binary
	if p.buildCache != nil && cacheKey != "" && !p.options.SkipCache {
		log.Debug("Caching built binary")
		if err := p.buildCache.PutBinary(cacheKey, outputPath, target.OS, target.Arch); err != nil {
			log.Warn("Failed to cache binary", "error", err)
		} else {
			log.Debug("Binary cached successfully", "cache_key", cacheKey)
		}
	}

	// Register artifact
	p.mu.Lock()
	p.artifacts.Add(artifact.Artifact{
		Name:    binary,
		Path:    outputPath,
		Type:    artifact.TypeBinary,
		Goos:    target.OS,
		Goarch:  target.Arch,
		Goarm:   target.Arm,
		BuildID: build.ID,
	})
	p.mu.Unlock()

	log.Info("Build completed successfully", "build", build.ID, "target", target.String(), "output", outputPath)
	return nil
}

// buildGo builds a Go binary
func (p *Pipeline) buildGo(ctx context.Context, build config.Build, target BuildTarget, output string) error {
	log.Debug("Building Go binary", "output", output)

	goBuilder := builder.NewGoBuilder()
	builderTarget := builder.Target{
		OS:    target.OS,
		Arch:  target.Arch,
		Arm:   target.Arm,
		Amd64: target.Amd64,
		Mips:  target.Mips,
	}

	return goBuilder.Build(ctx, build, builderTarget, output, p.templateCtx)
}

// buildRust builds a Rust binary
func (p *Pipeline) buildRust(ctx context.Context, build config.Build, target BuildTarget, output string) error {
	log.Debug("Building Rust binary", "output", output)

	rustBuilder := builder.NewRustBuilder()
	builderTarget := builder.Target{
		OS:    target.OS,
		Arch:  target.Arch,
		Arm:   target.Arm,
		Amd64: target.Amd64,
		Mips:  target.Mips,
	}

	return rustBuilder.Build(ctx, build, builderTarget, output, p.templateCtx)
}

// copyPrebuilt copies a prebuilt binary
func (p *Pipeline) copyPrebuilt(ctx context.Context, build config.Build, target BuildTarget, output string) error {
	log.Debug("Copying prebuilt binary", "output", output)

	prebuiltBuilder := builder.NewPrebuiltBuilder()
	builderTarget := builder.Target{
		OS:    target.OS,
		Arch:  target.Arch,
		Arm:   target.Arm,
		Amd64: target.Amd64,
		Mips:  target.Mips,
	}

	return prebuiltBuilder.Build(ctx, build, builderTarget, output, p.templateCtx)
}

// archive creates archives from built artifacts
func (p *Pipeline) archive(_ context.Context) error {
	log.Info("Creating archives")

	if len(p.config.Archives) == 0 {
		log.Debug("No archive configurations found")
		return nil
	}

	// Get binary artifacts
	binaries := p.artifacts.Filter(artifact.ByType(artifact.TypeBinary))
	if len(binaries) == 0 {
		log.Warn("No binaries to archive")
		return nil
	}

	// Create archive creator
	creator := archive.NewCreator(p.distDir, p.templateCtx)

	// Group binaries by target
	targetBinaries := make(map[string][]artifact.Artifact)
	for _, bin := range binaries {
		key := fmt.Sprintf("%s_%s", bin.Goos, bin.Goarch)
		targetBinaries[key] = append(targetBinaries[key], bin)
	}

	// Create archive for each configuration and target
	for _, archiveCfg := range p.config.Archives {
		for _, bins := range targetBinaries {
			arch, err := creator.Create(archiveCfg, bins)
			if err != nil {
				return fmt.Errorf("failed to create archive: %w", err)
			}
			if arch != nil {
				p.artifacts.Add(*arch)
			}
		}
	}

	return nil
}

// packages creates system packages
func (p *Pipeline) packages(ctx context.Context) error {
	log.Info("Creating packages")

	if len(p.config.NFPMs) == 0 {
		log.Debug("No package configurations found")
		return nil
	}

	// Create nfpm packager with full config for GUI app support
	packager := nfpm.NewMultiPackagerWithConfig(p.config.NFPMs, p.config, p.templateCtx, p.artifacts, p.distDir)
	return packager.BuildAll(ctx)
}

// platformPackages creates platform-specific packages (macOS App Bundle/DMG, Windows MSI/NSIS)
func (p *Pipeline) platformPackages(ctx context.Context) error {
	log.Info("Creating platform-specific packages")

	// Build macOS Universal Binaries first (before App Bundles)
	if len(p.config.UniversalBinaries) > 0 {
		if err := packaging.BuildAllUniversalBinaries(ctx, p.config.UniversalBinaries, p.templateCtx, p.artifacts, p.distDir); err != nil {
			return fmt.Errorf("failed to build Universal Binaries: %w", err)
		}
	}

	// Build macOS App Bundles
	if len(p.config.AppBundles) > 0 {
		if err := packaging.BuildAllAppBundles(ctx, p.config.AppBundles, p.templateCtx, p.artifacts, p.distDir); err != nil {
			return fmt.Errorf("failed to build App Bundles: %w", err)
		}
	}

	// Build macOS DMG images
	if len(p.config.DMGs) > 0 {
		if err := packaging.BuildAllDMGs(ctx, p.config.DMGs, p.templateCtx, p.artifacts, p.distDir); err != nil {
			return fmt.Errorf("failed to build DMGs: %w", err)
		}
	}

	// Build macOS PKG installers
	if len(p.config.PKGs) > 0 {
		if err := packaging.BuildAllPKGs(ctx, p.config.PKGs, p.templateCtx, p.artifacts, p.distDir); err != nil {
			return fmt.Errorf("failed to build PKGs: %w", err)
		}
	}

	// Build Windows MSI installers
	if len(p.config.MSIs) > 0 {
		if err := packaging.BuildAllMSIs(ctx, p.config.MSIs, p.templateCtx, p.artifacts, p.distDir); err != nil {
			return fmt.Errorf("failed to build MSIs: %w", err)
		}
	}

	// Build Windows NSIS installers
	if len(p.config.NSISs) > 0 {
		if err := packaging.BuildAllNSIS(ctx, p.config.NSISs, p.templateCtx, p.artifacts, p.distDir); err != nil {
			return fmt.Errorf("failed to build NSIS installers: %w", err)
		}
	}

	// Build Linux Flatpak packages
	if len(p.config.Flatpaks) > 0 {
		if err := packaging.BuildAllFlatpaks(ctx, p.config, p.templateCtx, p.artifacts, p.distDir); err != nil {
			return fmt.Errorf("failed to build Flatpaks: %w", err)
		}
	}

	// Build Linux AppImage packages
	if len(p.config.AppImages) > 0 {
		if err := packaging.BuildAllAppImages(ctx, p.config, p.templateCtx, p.artifacts, p.distDir); err != nil {
			return fmt.Errorf("failed to build AppImages: %w", err)
		}
	}

	// Build Linux Snap packages
	if len(p.config.Snapcrafts) > 0 {
		if err := packaging.BuildAllSnaps(ctx, p.config, p.templateCtx, p.artifacts, p.distDir); err != nil {
			return fmt.Errorf("failed to build Snaps: %w", err)
		}
	}

	return nil
}

// checksum creates checksums
func (p *Pipeline) checksum(_ context.Context) error {
	log.Info("Creating checksums")

	generator := checksum.NewGenerator(p.config.Checksum, p.distDir, p.artifacts)
	return generator.Run()
}

// sign signs artifacts
func (p *Pipeline) sign(ctx context.Context) error {
	log.Info("Signing artifacts")

	if len(p.config.Signs) == 0 {
		log.Debug("No signing configurations found")
		return nil
	}

	signer := sign.NewSigner(p.distDir, p.templateCtx)

	for _, signCfg := range p.config.Signs {
		allArtifacts := p.artifacts.List()
		signed, err := signer.Sign(ctx, signCfg, allArtifacts)
		if err != nil {
			return fmt.Errorf("signing failed: %w", err)
		}
		for _, sig := range signed {
			p.artifacts.Add(*sig)
		}
	}

	return nil
}

// docker builds Docker images
func (p *Pipeline) docker(ctx context.Context) error {
	log.Info("Building Docker images")

	if len(p.config.Dockers) == 0 {
		log.Debug("No Docker configurations found")
		return nil
	}

	dockerBuilder := docker.NewMultiBuilder(p.config.Dockers, p.templateCtx, p.artifacts, p.distDir)
	return dockerBuilder.BuildAll(ctx)
}

// dockerExports exports built Docker images into tar/tar.gz artifacts.
func (p *Pipeline) dockerExports() error {
	if len(p.config.DockerExports) == 0 {
		return nil
	}

	log.Info("Exporting Docker images")

	exports := make([]config.DockerExportConfig, 0, len(p.config.DockerExports))
	for _, exp := range p.config.DockerExports {
		image, err := p.templateCtx.Apply(exp.Image)
		if err != nil {
			return fmt.Errorf("failed to template docker export image for %s: %w", exp.ID, err)
		}

		output, err := p.templateCtx.Apply(exp.Output)
		if err != nil {
			return fmt.Errorf("failed to template docker export output for %s: %w", exp.ID, err)
		}

		format := exp.Format
		if format != "" {
			format, err = p.templateCtx.Apply(format)
			if err != nil {
				return fmt.Errorf("failed to template docker export format for %s: %w", exp.ID, err)
			}
		}

		exports = append(exports, config.DockerExportConfig{
			ID:     exp.ID,
			Image:  image,
			Format: format,
			Output: output,
		})
	}

	return docker.ExportAll(exports)
}

// publishRelease publishes to release platforms
func (p *Pipeline) publishRelease(ctx context.Context) error {
	log.Info("Publishing release")

	allArtifacts := p.artifacts.List()
	if len(allArtifacts) == 0 {
		log.Warn("No artifacts to publish")
		return nil
	}

	// Publish to GitHub
	if p.config.Release.GitHub.Owner != "" {
		publisher := publish.NewGitHubPublisher(p.config.Release, p.templateCtx)
		if err := publisher.Publish(ctx, allArtifacts); err != nil {
			return fmt.Errorf("GitHub publish failed: %w", err)
		}
	}

	// Publish to Homebrew
	for _, brewCfg := range p.config.Brews {
		publisher := publish.NewHomebrewPublisher(brewCfg, p.templateCtx)
		if err := publisher.Publish(ctx, allArtifacts); err != nil {
			return fmt.Errorf("Homebrew publish failed: %w", err)
		}
	}

	return nil
}

// publishDocker publishes Docker images
func (p *Pipeline) publishDocker(ctx context.Context) error {
	log.Info("Publishing Docker images")

	if len(p.config.Dockers) == 0 {
		return nil
	}

	dockerBuilder := docker.NewMultiBuilder(p.config.Dockers, p.templateCtx, p.artifacts, p.distDir)
	if err := dockerBuilder.PushAll(ctx); err != nil {
		return err
	}

	// Sign Docker images if configured
	if len(p.config.DockerSigns) > 0 {
		signer := docker.NewDockerSigner(p.config.DockerSigns, p.templateCtx, p.artifacts)
		if err := signer.SignAll(ctx); err != nil {
			return fmt.Errorf("docker signing failed: %w", err)
		}
	}

	return nil
}

// publishPackages publishes to package managers
func (p *Pipeline) publishPackages(ctx context.Context) error {
	log.Info("Publishing packages")

	allArtifacts := p.artifacts.List()

	// Publish to NPM
	for _, npmCfg := range p.config.NPMs {
		publisher := publish.NewNPMPublisher(npmCfg, p.templateCtx)
		if err := publisher.Publish(ctx, allArtifacts); err != nil {
			return fmt.Errorf("NPM publish failed: %w", err)
		}
	}

	// Publish to CloudSmith
	for _, cloudsmithCfg := range p.config.CloudSmiths {
		publisher := publish.NewCloudSmithPublisher(cloudsmithCfg, p.templateCtx)
		if err := publisher.Publish(ctx, allArtifacts); err != nil {
			return fmt.Errorf("CloudSmith publish failed: %w", err)
		}
	}

	// Publish to Fury
	for _, furyCfg := range p.config.Furies {
		publisher := publish.NewFuryPublisher(furyCfg, p.templateCtx)
		if err := publisher.Publish(ctx, allArtifacts); err != nil {
			return fmt.Errorf("Fury publish failed: %w", err)
		}
	}

	// Publish to Scoop
	for _, scoopCfg := range p.config.Scoops {
		publisher := publish.NewScoopPublisher(scoopCfg, p.templateCtx)
		if err := publisher.Publish(ctx, allArtifacts); err != nil {
			return fmt.Errorf("Scoop publish failed: %w", err)
		}
	}

	// Publish to AUR
	for _, aurCfg := range p.config.AURs {
		publisher := publish.NewAURPublisher(aurCfg, p.templateCtx, p.artifacts)
		if err := publisher.Publish(ctx, allArtifacts); err != nil {
			return fmt.Errorf("AUR publish failed: %w", err)
		}
	}

	// Publish to Chocolatey
	for _, chocoCfg := range p.config.Chocolateys {
		publisher := publish.NewChocolateyPublisher(chocoCfg, p.templateCtx, p.artifacts)
		if err := publisher.Publish(ctx, allArtifacts); err != nil {
			return fmt.Errorf("Chocolatey publish failed: %w", err)
		}
	}

	// Publish to Winget
	for _, wingetCfg := range p.config.Wingets {
		publisher := publish.NewWingetPublisher(wingetCfg, p.templateCtx, p.artifacts)
		if err := publisher.Publish(ctx, allArtifacts); err != nil {
			return fmt.Errorf("Winget publish failed: %w", err)
		}
	}

	// Publish to crates.io
	for _, crateCfg := range p.config.Crates {
		publisher := publish.NewCratePublisher(crateCfg, p.templateCtx)
		if err := publisher.Publish(ctx, allArtifacts); err != nil {
			return fmt.Errorf("Crate publish failed: %w", err)
		}
	}

	// Publish to PyPI
	for _, pypiCfg := range p.config.PyPIs {
		publisher := publish.NewPyPIPublisher(pypiCfg, p.templateCtx)
		if err := publisher.Publish(ctx, allArtifacts); err != nil {
			return fmt.Errorf("PyPI publish failed: %w", err)
		}
	}

	// Publish to Maven Central
	for _, mavenCfg := range p.config.Mavens {
		publisher := publish.NewMavenPublisher(mavenCfg, p.templateCtx)
		if err := publisher.Publish(ctx, allArtifacts); err != nil {
			return fmt.Errorf("Maven publish failed: %w", err)
		}
	}

	// Publish to NuGet
	for _, nugetCfg := range p.config.NuGets {
		publisher := publish.NewNuGetPublisher(nugetCfg, p.templateCtx)
		if err := publisher.Publish(ctx, allArtifacts); err != nil {
			return fmt.Errorf("NuGet publish failed: %w", err)
		}
	}

	// Publish to RubyGems
	for _, gemCfg := range p.config.Gems {
		publisher := publish.NewGemPublisher(gemCfg, p.templateCtx)
		if err := publisher.Publish(ctx, allArtifacts); err != nil {
			return fmt.Errorf("Gem publish failed: %w", err)
		}
	}

	// Publish Helm charts
	for _, helmCfg := range p.config.Helms {
		publisher := publish.NewHelmPublisher(helmCfg, p.templateCtx)
		if err := publisher.Publish(ctx, allArtifacts); err != nil {
			return fmt.Errorf("Helm publish failed: %w", err)
		}
	}

	return nil
}

// runAnnouncements runs all configured announcements
func (p *Pipeline) runAnnouncements(ctx context.Context) error {
	log.Info("Running announcements")

	announcer := announce.NewAnnouncer(p.config.Announce, p.templateCtx)
	return announcer.Run(ctx)
}

// StateFile represents the saved pipeline state
type StateFile struct {
	Version   string              `json:"version"`
	Tag       string              `json:"tag"`
	Artifacts []artifact.Artifact `json:"artifacts"`
	Timestamp time.Time           `json:"timestamp"`
}

// saveState saves the pipeline state for later continuation
func (p *Pipeline) saveState() error {
	log.Debug("Saving pipeline state")

	state := StateFile{
		Version:   p.templateCtx.Get("Version"),
		Tag:       p.templateCtx.Get("Tag"),
		Artifacts: p.artifacts.List(),
		Timestamp: time.Now(),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	statePath := filepath.Join(p.distDir, ".releaser-state.json")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	log.Info("Pipeline state saved", "path", statePath)
	return nil
}

// loadState loads the pipeline state from a previous prepare
func (p *Pipeline) loadState() error {
	statePath := filepath.Join(p.distDir, ".releaser-state.json")

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no saved state found")
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var state StateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Restore artifacts
	for _, a := range state.Artifacts {
		p.artifacts.Add(a)
	}

	log.Info("Pipeline state loaded", "artifacts", len(state.Artifacts), "timestamp", state.Timestamp)
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}

// ensureBuildDependencies checks and installs required build tools
func (p *Pipeline) ensureBuildDependencies() error {
	log.Info("Checking build dependencies")

	// Collect all targets that need cross-compilation
	var crossTargets []string
	needsCGO := false
	needsPackaging := len(p.config.NFPMs) > 0
	needsDocker := len(p.config.Dockers) > 0 && !p.options.SkipDocker
	needsSigning := !p.options.SkipSign && len(p.config.Signs) > 0
	needsSBOM := len(p.config.SBOMs) > 0

	for _, build := range p.config.Builds {
		if build.Skip {
			continue
		}

		// Check if this build requires CGO
		if build.Cgo.Enabled {
			needsCGO = true

			// Collect targets for CGO cross-compilation
			for _, goos := range build.Goos {
				for _, goarch := range build.Goarch {
					target := goos + "_" + goarch
					crossTargets = append(crossTargets, target)
				}
			}
		}
	}

	// Ensure Fyne GUI dependencies if needed
	for _, build := range p.config.Builds {
		if build.Type == "gui" && build.Cgo.Enabled {
			if err := deps.DetectAndInstallForFyne(); err != nil {
				log.Warn("Could not install Fyne dependencies", "error", err)
			}
			break
		}
	}

	// Ensure cross-compilers for CGO builds
	if needsCGO && len(crossTargets) > 0 {
		if err := deps.EnsureCrossCompilers(crossTargets); err != nil {
			log.Warn("Could not install all cross-compilers", "error", err)
		}
	}

	// Ensure packaging tools
	if needsPackaging {
		if err := deps.CheckAndInstall("nfpm"); err != nil {
			return fmt.Errorf("nfpm required for packaging: %w", err)
		}
	}

	// Ensure Docker
	if needsDocker {
		if err := deps.CheckAndInstall("docker"); err != nil {
			log.Warn("Docker not available, skipping docker builds", "error", err)
		}
	}

	// Ensure signing tools
	if needsSigning {
		// Check which signing method is configured
		for _, signCfg := range p.config.Signs {
			switch signCfg.Cmd {
			case "cosign":
				if err := deps.CheckAndInstall("cosign"); err != nil {
					log.Warn("Cosign not available", "error", err)
				}
			case "gpg", "":
				if err := deps.CheckAndInstall("gpg"); err != nil {
					log.Warn("GPG not available", "error", err)
				}
			}
		}
	}

	// Ensure SBOM tools
	if needsSBOM {
		if err := deps.CheckAndInstall("syft"); err != nil {
			log.Warn("Syft not available, SBOM generation may be skipped", "error", err)
		}
	}

	// Check for UPX if compression is enabled in upx config
	if len(p.config.UPXs) > 0 {
		if err := deps.CheckAndInstall("upx"); err != nil {
			log.Warn("UPX not available, binary compression disabled", "error", err)
		}
	}

	log.Info("Dependency check completed")
	return nil
}
