// Package docker provides Docker image building and publishing.
package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/oarkflow/releaser/internal/artifact"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// Builder builds Docker images.
type Builder struct {
	config  config.Docker
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewBuilder creates a new Docker builder.
func NewBuilder(cfg config.Docker, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *Builder {
	return &Builder{
		config:  cfg,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// Build builds Docker images.
func (b *Builder) Build(ctx context.Context) error {
	if b.config.Skip == "true" {
		log.Info("Skipping Docker build")
		return nil
	}

	if b.config.SkipBuild {
		log.Info("Skipping Docker image build (push only mode)")
		return nil
	}

	log.Info("Building Docker image")

	// Determine Dockerfile path
	dockerfile := b.config.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}

	// Check if Dockerfile exists
	if _, err := os.Stat(dockerfile); os.IsNotExist(err) {
		return fmt.Errorf("Dockerfile not found: %s", dockerfile)
	}

	// Prepare image tags
	imageTags, err := b.prepareTags()
	if err != nil {
		return fmt.Errorf("failed to prepare tags: %w", err)
	}

	if len(imageTags) == 0 {
		return fmt.Errorf("no image tags configured")
	}

	// Build command
	var args []string

	if b.config.Buildx {
		args = append(args, "buildx", "build")
		if len(b.config.BuildxPlatforms) > 0 {
			args = append(args, "--platform", strings.Join(b.config.BuildxPlatforms, ","))
		}
		if b.config.Push {
			args = append(args, "--push")
		}
	} else {
		args = append(args, "build")
	}

	// Add tags
	for _, tag := range imageTags {
		args = append(args, "-t", tag)
	}

	// Add Dockerfile
	args = append(args, "-f", dockerfile)

	// Add build args
	for _, arg := range b.config.BuildArgs {
		expandedArg, _ := b.tmplCtx.Apply(arg)
		args = append(args, "--build-arg", expandedArg)
	}

	// Add extra files if needed
	for _, file := range b.config.ExtraFiles {
		expandedFile, _ := b.tmplCtx.Apply(file)
		// Copy file to build context if needed
		if _, err := os.Stat(expandedFile); err == nil {
			destPath := filepath.Join(filepath.Dir(dockerfile), filepath.Base(expandedFile))
			if err := copyFile(expandedFile, destPath); err != nil {
				log.Warn("Failed to copy extra file", "file", expandedFile, "error", err)
			}
		}
	}

	// Build context
	buildContext := "."
	args = append(args, buildContext)

	log.Debug("Running docker command", "args", args)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	// Add artifact for each tag
	for _, tag := range imageTags {
		b.manager.Add(artifact.Artifact{
			Name: tag,
			Path: "",
			Type: artifact.TypeDockerImage,
			Extra: map[string]interface{}{
				"image": tag,
			},
		})
	}

	log.Info("Docker image built successfully", "tags", imageTags)
	return nil
}

// Push pushes Docker images.
func (b *Builder) Push(ctx context.Context) error {
	if b.config.Skip == "true" {
		log.Info("Skipping Docker push")
		return nil
	}

	if !b.config.Push {
		log.Debug("Docker push not enabled")
		return nil
	}

	// If using buildx with --push, images are already pushed
	if b.config.Buildx {
		log.Debug("Images already pushed via buildx")
		return nil
	}

	log.Info("Pushing Docker images")

	imageTags, err := b.prepareTags()
	if err != nil {
		return fmt.Errorf("failed to prepare tags: %w", err)
	}

	for _, tag := range imageTags {
		cmd := exec.CommandContext(ctx, "docker", "push", tag)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to push %s: %w", tag, err)
		}
		log.Info("Pushed Docker image", "tag", tag)
	}

	return nil
}

// prepareTags prepares image tags with template expansion.
func (b *Builder) prepareTags() ([]string, error) {
	var tags []string

	for _, tagTemplate := range b.config.ImageTemplates {
		tag, err := b.tmplCtx.Apply(tagTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to apply template to tag: %w", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// MultiBuilder builds multiple Docker configurations.
type MultiBuilder struct {
	configs []config.Docker
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewMultiBuilder creates a multi-config Docker builder.
func NewMultiBuilder(configs []config.Docker, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *MultiBuilder {
	return &MultiBuilder{
		configs: configs,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// BuildAll builds all Docker configurations.
func (m *MultiBuilder) BuildAll(ctx context.Context) error {
	for i, cfg := range m.configs {
		log.Info("Building Docker image", "index", i+1, "total", len(m.configs))
		builder := NewBuilder(cfg, m.tmplCtx, m.manager, m.distDir)
		if err := builder.Build(ctx); err != nil {
			return err
		}
	}
	return nil
}

// PushAll pushes all Docker images.
func (m *MultiBuilder) PushAll(ctx context.Context) error {
	for _, cfg := range m.configs {
		builder := NewBuilder(cfg, m.tmplCtx, m.manager, m.distDir)
		if err := builder.Push(ctx); err != nil {
			return err
		}
	}
	return nil
}
