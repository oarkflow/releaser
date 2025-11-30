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

// DockerSigner signs Docker images using cosign or other tools
type DockerSigner struct {
	configs []config.DockerSign
	tmplCtx *tmpl.Context
	manager *artifact.Manager
}

// NewDockerSigner creates a new Docker signer
func NewDockerSigner(configs []config.DockerSign, tmplCtx *tmpl.Context, manager *artifact.Manager) *DockerSigner {
	return &DockerSigner{
		configs: configs,
		tmplCtx: tmplCtx,
		manager: manager,
	}
}

// SignAll signs all Docker images according to configuration
func (s *DockerSigner) SignAll(ctx context.Context) error {
	if len(s.configs) == 0 {
		log.Debug("No Docker signing configurations")
		return nil
	}

	log.Info("Signing Docker images")

	for _, cfg := range s.configs {
		if err := s.signWithConfig(ctx, cfg); err != nil {
			return fmt.Errorf("docker signing failed: %w", err)
		}
	}

	return nil
}

// signWithConfig signs images with a specific configuration
func (s *DockerSigner) signWithConfig(ctx context.Context, cfg config.DockerSign) error {
	// Get images to sign
	var images []string

	if len(cfg.Images) > 0 {
		// Use explicitly specified images
		for _, img := range cfg.Images {
			expanded, err := s.tmplCtx.Apply(img)
			if err != nil {
				return fmt.Errorf("failed to expand image template: %w", err)
			}
			images = append(images, expanded)
		}
	} else {
		// Get Docker images from artifacts
		dockerImages := s.manager.Filter(func(a artifact.Artifact) bool {
			return a.Type == artifact.TypeDockerImage
		})
		for _, img := range dockerImages {
			if imgName, ok := img.Extra["image"].(string); ok {
				images = append(images, imgName)
			}
		}
	}

	if len(images) == 0 {
		log.Debug("No Docker images to sign")
		return nil
	}

	// Use cosign if configured
	if cfg.Cosign != nil {
		return s.signWithCosign(ctx, cfg, images)
	}

	// Use custom command if specified
	if cfg.Cmd != "" {
		return s.signWithCustomCmd(ctx, cfg, images)
	}

	// Default to cosign keyless if no configuration specified
	return s.signWithCosign(ctx, cfg, images)
}

// signWithCosign signs images using cosign
func (s *DockerSigner) signWithCosign(ctx context.Context, cfg config.DockerSign, images []string) error {
	// Check if cosign is available
	if _, err := exec.LookPath("cosign"); err != nil {
		log.Warn("Skipping Docker signing: cosign not found in PATH")
		return nil
	}

	for _, image := range images {
		log.Info("Signing Docker image with cosign", "image", image)

		args := []string{"sign"}

		if cfg.Cosign != nil {
			if cfg.Cosign.Keyless {
				// Keyless signing using OIDC/Sigstore
				args = append(args, "--yes")
				if cfg.Cosign.OIDCIssuer != "" {
					args = append(args, "--oidc-issuer", cfg.Cosign.OIDCIssuer)
				}
				if cfg.Cosign.OIDCClientID != "" {
					args = append(args, "--oidc-client-id", cfg.Cosign.OIDCClientID)
				}
				if cfg.Cosign.FulcioURL != "" {
					args = append(args, "--fulcio-url", cfg.Cosign.FulcioURL)
				}
				if cfg.Cosign.RekorURL != "" {
					args = append(args, "--rekor-url", cfg.Cosign.RekorURL)
				}
			} else if cfg.Cosign.KeyRef != "" {
				// Key-based signing
				args = append(args, "--key", cfg.Cosign.KeyRef)
				if cfg.Cosign.Certificate != "" {
					args = append(args, "--certificate", cfg.Cosign.Certificate)
				}
				if cfg.Cosign.CertificateChain != "" {
					args = append(args, "--certificate-chain", cfg.Cosign.CertificateChain)
				}
			}

			if cfg.Cosign.Recursive {
				args = append(args, "--recursive")
			}

			// Add annotations
			for k, v := range cfg.Cosign.Annotations {
				args = append(args, "-a", fmt.Sprintf("%s=%s", k, v))
			}
		} else {
			// Default to keyless signing
			args = append(args, "--yes")
		}

		args = append(args, image)

		// Prepare environment
		env := os.Environ()
		for _, e := range cfg.Env {
			expanded, _ := s.tmplCtx.Apply(e)
			env = append(env, expanded)
		}

		cmd := exec.CommandContext(ctx, "cosign", args...)
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("cosign sign failed for %s: %w", image, err)
		}

		log.Info("Docker image signed successfully", "image", image)
	}

	return nil
}

// signWithCustomCmd signs images using a custom command
func (s *DockerSigner) signWithCustomCmd(ctx context.Context, cfg config.DockerSign, images []string) error {
	for _, image := range images {
		log.Info("Signing Docker image", "image", image, "cmd", cfg.Cmd)

		// Expand arguments
		args := make([]string, len(cfg.Args))
		for i, arg := range cfg.Args {
			expanded := strings.ReplaceAll(arg, "${image}", image)
			expanded, _ = s.tmplCtx.Apply(expanded)
			args[i] = expanded
		}

		// Prepare environment
		env := os.Environ()
		for _, e := range cfg.Env {
			expanded, _ := s.tmplCtx.Apply(e)
			env = append(env, expanded)
		}

		cmd := exec.CommandContext(ctx, cfg.Cmd, args...)
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("docker signing command failed for %s: %w", image, err)
		}

		log.Info("Docker image signed successfully", "image", image)
	}

	return nil
}
