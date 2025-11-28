// Package sbom provides Software Bill of Materials generation.
package sbom

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

// Generator generates SBOMs for artifacts.
type Generator struct {
	config  config.SBOM
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewGenerator creates a new SBOM generator.
func NewGenerator(cfg config.SBOM, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *Generator {
	return &Generator{
		config:  cfg,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// Run generates SBOMs for artifacts.
func (g *Generator) Run(ctx context.Context) error {
	if g.config.Skip == "true" {
		log.Info("Skipping SBOM generation")
		return nil
	}

	log.Info("Generating Software Bill of Materials")

	// Get artifacts to generate SBOMs for
	artifacts := g.manager.Filter(artifact.ByType(artifact.TypeBinary))

	if len(artifacts) == 0 {
		log.Warn("No artifacts found for SBOM generation")
		return nil
	}

	method := g.config.Method
	if method == "" {
		method = "syft" // Default to syft
	}

	format := g.config.Format
	if format == "" {
		format = "spdx-json"
	}

	for _, a := range artifacts {
		if err := g.generateSBOM(ctx, a, method, format); err != nil {
			return fmt.Errorf("failed to generate SBOM for %s: %w", a.Name, err)
		}
	}

	log.Info("SBOM generation complete")
	return nil
}

// generateSBOM generates SBOM for a single artifact.
func (g *Generator) generateSBOM(ctx context.Context, a artifact.Artifact, method, format string) error {
	log.Debug("Generating SBOM", "artifact", a.Name, "method", method, "format", format)

	// Determine output filename
	ext := formatExtension(format)
	sbomName := fmt.Sprintf("%s.sbom.%s", a.Name, ext)
	sbomPath := filepath.Join(g.distDir, sbomName)

	var err error
	switch method {
	case "syft":
		err = g.runSyft(ctx, a.Path, sbomPath, format)
	case "cyclonedx-gomod":
		err = g.runCycloneDXGoMod(ctx, sbomPath, format)
	case "trivy":
		err = g.runTrivy(ctx, a.Path, sbomPath, format)
	default:
		return fmt.Errorf("unsupported SBOM method: %s", method)
	}

	if err != nil {
		return err
	}

	// Add artifact
	g.manager.Add(artifact.Artifact{
		Name: sbomName,
		Path: sbomPath,
		Type: artifact.TypeSBOM,
		Extra: map[string]interface{}{
			"format": format,
			"method": method,
			"source": a.Name,
		},
	})

	log.Info("SBOM generated", "name", sbomName)
	return nil
}

// runSyft runs syft to generate SBOM.
func (g *Generator) runSyft(ctx context.Context, target, output, format string) error {
	args := []string{
		"scan",
		target,
		"-o", fmt.Sprintf("%s=%s", format, output),
	}

	cmd := exec.CommandContext(ctx, "syft", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// runCycloneDXGoMod runs cyclonedx-gomod for Go projects.
func (g *Generator) runCycloneDXGoMod(ctx context.Context, output, format string) error {
	args := []string{
		"mod",
		"-output", output,
	}

	if format == "json" {
		args = append(args, "-json")
	}

	cmd := exec.CommandContext(ctx, "cyclonedx-gomod", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// runTrivy runs trivy for SBOM generation.
func (g *Generator) runTrivy(ctx context.Context, target, output, format string) error {
	trivyFormat := "spdx-json"
	switch format {
	case "cyclonedx-json":
		trivyFormat = "cyclonedx"
	case "spdx-json":
		trivyFormat = "spdx-json"
	}

	args := []string{
		"sbom",
		"--format", trivyFormat,
		"--output", output,
		target,
	}

	cmd := exec.CommandContext(ctx, "trivy", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// formatExtension returns file extension for SBOM format.
func formatExtension(format string) string {
	switch format {
	case "spdx-json":
		return "spdx.json"
	case "spdx-tag-value":
		return "spdx"
	case "cyclonedx-json":
		return "cdx.json"
	case "cyclonedx-xml":
		return "cdx.xml"
	case "syft-json":
		return "syft.json"
	default:
		return "json"
	}
}

// MultiGenerator generates SBOMs for multiple configurations.
type MultiGenerator struct {
	configs []config.SBOM
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewMultiGenerator creates a multi-config SBOM generator.
func NewMultiGenerator(configs []config.SBOM, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *MultiGenerator {
	return &MultiGenerator{
		configs: configs,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// RunAll generates SBOMs for all configurations.
func (m *MultiGenerator) RunAll(ctx context.Context) error {
	for i, cfg := range m.configs {
		log.Info("Generating SBOM", "index", i+1, "total", len(m.configs))
		gen := NewGenerator(cfg, m.tmplCtx, m.manager, m.distDir)
		if err := gen.Run(ctx); err != nil {
			return err
		}
	}
	return nil
}
