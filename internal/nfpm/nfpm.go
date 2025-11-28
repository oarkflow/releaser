// Package nfpm provides Linux package building.
package nfpm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/charmbracelet/log"
	"github.com/oarkflow/releaser/internal/artifact"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// Packager creates Linux packages.
type Packager struct {
	config  config.NFPM
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewPackager creates a new nfpm packager.
func NewPackager(cfg config.NFPM, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *Packager {
	return &Packager{
		config:  cfg,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// Build creates Linux packages for artifacts.
func (p *Packager) Build(ctx context.Context) error {
	if p.config.Skip == "true" {
		log.Info("Skipping nfpm packaging")
		return nil
	}

	log.Info("Building Linux packages with nfpm")

	// Get binary artifacts
	binaries := p.manager.Filter(artifact.ByType(artifact.TypeBinary))

	if len(binaries) == 0 {
		log.Warn("No binaries found for packaging")
		return nil
	}

	// Build packages for each format
	formats := p.config.Formats
	if len(formats) == 0 {
		formats = []string{"deb", "rpm"}
	}

	for _, binary := range binaries {
		// Only package linux binaries
		if binary.Goos != "" && binary.Goos != "linux" {
			continue
		}

		for _, format := range formats {
			if err := p.buildPackage(ctx, binary, format); err != nil {
				return fmt.Errorf("failed to build %s package for %s: %w", format, binary.Name, err)
			}
		}
	}

	log.Info("Linux packages built successfully")
	return nil
}

// buildPackage builds a single package using nfpm CLI or fpm.
func (p *Packager) buildPackage(ctx context.Context, binary artifact.Artifact, format string) error {
	log.Debug("Building package", "binary", binary.Name, "format", format)

	// Prepare package name
	pkgName := p.config.PackageName
	if pkgName == "" {
		pkgName = p.tmplCtx.Get("ProjectName")
	}

	// Prepare version
	version := p.tmplCtx.Get("Version")
	version = strings.TrimPrefix(version, "v")

	// Prepare architecture
	arch := binary.Goarch
	if arch == "" {
		arch = "amd64"
	}
	arch = normalizeArch(arch, format)

	// Generate nfpm config file
	nfpmConfigPath := filepath.Join(p.distDir, fmt.Sprintf("nfpm-%s-%s.yaml", binary.Name, format))
	if err := p.generateNfpmConfig(nfpmConfigPath, binary, pkgName, version, arch, format); err != nil {
		return fmt.Errorf("failed to generate nfpm config: %w", err)
	}

	// Generate output filename
	outputName := fmt.Sprintf("%s_%s_%s.%s", pkgName, version, arch, format)
	outputPath := filepath.Join(p.distDir, outputName)

	// Try nfpm first, then fpm
	if err := p.runNfpm(ctx, nfpmConfigPath, outputPath, format); err != nil {
		log.Debug("nfpm not available, trying fpm", "error", err)
		if err := p.runFpm(ctx, binary, pkgName, version, arch, outputPath, format); err != nil {
			return fmt.Errorf("failed to create package (neither nfpm nor fpm worked): %w", err)
		}
	}

	// Add artifact
	p.manager.Add(artifact.Artifact{
		Name:   outputName,
		Path:   outputPath,
		Type:   artifact.TypeLinuxPackage,
		Goos:   "linux",
		Goarch: binary.Goarch,
		Extra: map[string]interface{}{
			"format": format,
		},
	})

	log.Info("Package created", "name", outputName)
	return nil
}

// generateNfpmConfig generates an nfpm configuration file.
func (p *Packager) generateNfpmConfig(path string, binary artifact.Artifact, name, version, arch, format string) error {
	configTemplate := `name: "{{ .Name }}"
arch: "{{ .Arch }}"
platform: "linux"
version: "{{ .Version }}"
maintainer: "{{ .Maintainer }}"
description: "{{ .Description }}"
vendor: "{{ .Vendor }}"
homepage: "{{ .Homepage }}"
license: "{{ .License }}"

contents:
  - src: "{{ .BinaryPath }}"
    dst: "{{ .Bindir }}/{{ .BinaryName }}"
    type: file
    file_info:
      mode: 0755
{{ range .Contents }}
  - src: "{{ .Src }}"
    dst: "{{ .Dst }}"
{{ end }}
{{ if .Dependencies }}
depends:
{{ range .Dependencies }}
  - "{{ . }}"
{{ end }}
{{ end }}
`

	tmpl, err := template.New("nfpm").Parse(configTemplate)
	if err != nil {
		return err
	}

	bindir := p.config.Bindir
	if bindir == "" {
		bindir = "/usr/bin"
	}

	data := map[string]interface{}{
		"Name":         name,
		"Arch":         arch,
		"Version":      version,
		"Maintainer":   p.config.Maintainer,
		"Description":  p.config.Description,
		"Vendor":       p.config.Vendor,
		"Homepage":     p.config.Homepage,
		"License":      p.config.License,
		"BinaryPath":   binary.Path,
		"BinaryName":   binary.Name,
		"Bindir":       bindir,
		"Contents":     p.config.Contents,
		"Dependencies": p.config.Dependencies,
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

// runNfpm runs nfpm to create a package.
func (p *Packager) runNfpm(ctx context.Context, configPath, outputPath, format string) error {
	cmd := exec.CommandContext(ctx, "nfpm", "pkg",
		"--config", configPath,
		"--packager", format,
		"--target", outputPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runFpm runs fpm as a fallback.
func (p *Packager) runFpm(ctx context.Context, binary artifact.Artifact, name, version, arch, outputPath, format string) error {
	bindir := p.config.Bindir
	if bindir == "" {
		bindir = "/usr/bin"
	}

	args := []string{
		"-s", "dir",
		"-t", format,
		"-n", name,
		"-v", version,
		"-a", arch,
		"-p", outputPath,
		"--prefix", bindir,
	}

	if p.config.Description != "" {
		args = append(args, "--description", p.config.Description)
	}
	if p.config.Maintainer != "" {
		args = append(args, "-m", p.config.Maintainer)
	}
	if p.config.Homepage != "" {
		args = append(args, "--url", p.config.Homepage)
	}
	if p.config.License != "" {
		args = append(args, "--license", p.config.License)
	}
	if p.config.Vendor != "" {
		args = append(args, "--vendor", p.config.Vendor)
	}

	// Add dependencies
	for _, dep := range p.config.Dependencies {
		args = append(args, "-d", dep)
	}

	// Add binary
	args = append(args, binary.Path+"="+binary.Name)

	cmd := exec.CommandContext(ctx, "fpm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// normalizeArch normalizes Go architecture to package architecture.
func normalizeArch(arch, format string) string {
	switch format {
	case "deb":
		switch arch {
		case "amd64":
			return "amd64"
		case "386":
			return "i386"
		case "arm64":
			return "arm64"
		case "arm":
			return "armhf"
		default:
			return arch
		}
	case "rpm":
		switch arch {
		case "amd64":
			return "x86_64"
		case "386":
			return "i686"
		case "arm64":
			return "aarch64"
		case "arm":
			return "armhfp"
		default:
			return arch
		}
	case "apk":
		switch arch {
		case "amd64":
			return "x86_64"
		case "386":
			return "x86"
		case "arm64":
			return "aarch64"
		case "arm":
			return "armhf"
		default:
			return arch
		}
	default:
		return arch
	}
}

// MultiPackager builds packages for multiple configurations.
type MultiPackager struct {
	configs []config.NFPM
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewMultiPackager creates a multi-config packager.
func NewMultiPackager(configs []config.NFPM, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *MultiPackager {
	return &MultiPackager{
		configs: configs,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// BuildAll builds all package configurations.
func (m *MultiPackager) BuildAll(ctx context.Context) error {
	for i, cfg := range m.configs {
		log.Info("Building packages", "index", i+1, "total", len(m.configs))
		packager := NewPackager(cfg, m.tmplCtx, m.manager, m.distDir)
		if err := packager.Build(ctx); err != nil {
			return err
		}
	}
	return nil
}
