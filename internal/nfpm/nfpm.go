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
	"github.com/oarkflow/releaser/internal/assets"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// Packager creates Linux packages.
type Packager struct {
	config     config.NFPM
	allConfigs *config.Config
	tmplCtx    *tmpl.Context
	manager    *artifact.Manager
	distDir    string
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

// NewPackagerWithConfig creates a new nfpm packager with full config access.
func NewPackagerWithConfig(cfg config.NFPM, allConfigs *config.Config, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *Packager {
	return &Packager{
		config:     cfg,
		allConfigs: allConfigs,
		tmplCtx:    tmplCtx,
		manager:    manager,
		distDir:    distDir,
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
	allBinaries := p.manager.Filter(artifact.ByType(artifact.TypeBinary))

	if len(allBinaries) == 0 {
		log.Warn("No binaries found for packaging")
		return nil
	}

	// Filter and group linux binaries by architecture
	archBinaries := make(map[string][]artifact.Artifact)
	for _, binary := range allBinaries {
		// Only package linux binaries
		if binary.Goos != "" && binary.Goos != "linux" {
			continue
		}
		arch := binary.Goarch
		if arch == "" {
			arch = "amd64"
		}
		archBinaries[arch] = append(archBinaries[arch], binary)
	}

	if len(archBinaries) == 0 {
		log.Warn("No Linux binaries found for packaging")
		return nil
	}

	// Build packages for each format
	formats := p.config.Formats
	if len(formats) == 0 {
		formats = []string{"deb", "rpm"}
	}

	// Build a single package per architecture containing ALL binaries
	for arch, binaries := range archBinaries {
		for _, format := range formats {
			if err := p.buildPackageWithBinaries(ctx, binaries, arch, format); err != nil {
				return fmt.Errorf("failed to build %s package for %s: %w", format, arch, err)
			}
		}
	}

	log.Info("Linux packages built successfully")
	return nil
}

// buildPackageWithBinaries builds a single package containing multiple binaries.
func (p *Packager) buildPackageWithBinaries(ctx context.Context, binaries []artifact.Artifact, arch, format string) error {
	log.Debug("Building package", "arch", arch, "format", format, "binaries", len(binaries))

	// Prepare package name
	pkgName := p.config.PackageName
	if pkgName == "" {
		pkgName = p.tmplCtx.Get("ProjectName")
	}

	// Prepare version
	version := p.tmplCtx.Get("Version")
	version = strings.TrimPrefix(version, "v")

	// Normalize architecture for package format
	normalizedArch := normalizeArch(arch, format)

	// Generate nfpm config file
	nfpmConfigPath := filepath.Join(p.distDir, fmt.Sprintf("nfpm-%s-%s.yaml", normalizedArch, format))
	if err := p.generateNfpmConfigMulti(nfpmConfigPath, binaries, pkgName, version, normalizedArch, format); err != nil {
		return fmt.Errorf("failed to generate nfpm config: %w", err)
	}

	// Generate output filename
	outputName := fmt.Sprintf("%s_%s_%s.%s", pkgName, version, normalizedArch, format)
	outputPath := filepath.Join(p.distDir, outputName)

	// Try nfpm first, then fpm
	if err := p.runNfpm(ctx, nfpmConfigPath, outputPath, format); err != nil {
		log.Debug("nfpm not available, trying fpm", "error", err)
		if err := p.runFpmMulti(ctx, binaries, pkgName, version, normalizedArch, outputPath, format); err != nil {
			return fmt.Errorf("failed to create package (neither nfpm nor fpm worked): %w", err)
		}
	}

	// Add artifact
	p.manager.Add(artifact.Artifact{
		Name:   outputName,
		Path:   outputPath,
		Type:   artifact.TypeLinuxPackage,
		Goos:   "linux",
		Goarch: arch,
		Extra: map[string]interface{}{
			"format": format,
		},
	})

	log.Info("Package created", "name", outputName, "binaries", len(binaries))
	return nil
}

// buildPackage builds a single package using nfpm CLI or fpm.
func (p *Packager) buildPackage(ctx context.Context, binary artifact.Artifact, format string) error {
	return p.buildPackageWithBinaries(ctx, []artifact.Artifact{binary}, binary.Goarch, format)
}

// generateNfpmConfigMulti generates an nfpm configuration file for multiple binaries.
func (p *Packager) generateNfpmConfigMulti(path string, binaries []artifact.Artifact, name, version, arch, format string) error {
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
{{ range .Binaries }}
  - src: "{{ .Path }}"
    dst: "{{ $.Bindir }}/{{ .Name }}"
    type: file
    file_info:
      mode: 0755
{{ end }}
{{ range .GUIEntries }}
  - src: "{{ .DesktopFile }}"
    dst: "/usr/share/applications/{{ .AppID }}.desktop"
    type: file
{{ if .IconPath }}
  - src: "{{ .IconPath }}"
    dst: "/usr/share/icons/hicolor/256x256/apps/{{ .AppID }}.png"
    type: file
{{ end }}
{{ end }}
{{ range .Contents }}
  - src: "{{ .Src }}"
    dst: "{{ .Dst }}"
{{ if .Type }}
    type: {{ .Type }}
{{ end }}
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

	// Collect GUI entries for all GUI binaries
	type guiEntry struct {
		AppID       string
		DesktopFile string
		IconPath    string
	}
	var guiEntries []guiEntry

	for _, binary := range binaries {
		// Check if this is a GUI application
		isGUI := false
		var guiConfig *config.GUIConfig
		appID := binary.Name

		if p.allConfigs != nil {
			for _, build := range p.allConfigs.Builds {
				if build.ID == binary.BuildID && build.Type == "gui" {
					isGUI = true
					guiConfig = build.GUI
					break
				}
			}
		}

		if isGUI {
			iconPath := resolveGUIIconPath(guiConfig, p.distDir, appID)

			desktopFile := filepath.Join(p.distDir, appID+".desktop")
			if err := p.generateDesktopFile(desktopFile, binary, guiConfig, bindir); err != nil {
				log.Warn("Failed to generate desktop file", "binary", binary.Name, "error", err)
			} else {
				guiEntries = append(guiEntries, guiEntry{
					AppID:       appID,
					DesktopFile: desktopFile,
					IconPath:    iconPath,
				})
			}
		}
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
		"Binaries":     binaries,
		"Bindir":       bindir,
		"Contents":     p.config.Contents,
		"Dependencies": p.config.Dependencies,
		"GUIEntries":   guiEntries,
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
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
{{ if .IsGUI }}
  - src: "{{ .DesktopFile }}"
    dst: "/usr/share/applications/{{ .AppID }}.desktop"
    type: file
{{ if .IconPath }}
  - src: "{{ .IconPath }}"
    dst: "/usr/share/icons/hicolor/256x256/apps/{{ .AppID }}.png"
    type: file
{{ end }}
{{ end }}
{{ range .Contents }}
  - src: "{{ .Src }}"
    dst: "{{ .Dst }}"
{{ if .Type }}
    type: {{ .Type }}
{{ end }}
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

	// Check if this is a GUI application
	isGUI := false
	var guiConfig *config.GUIConfig
	appID := name
	desktopFile := ""
	iconPath := ""

	if p.allConfigs != nil {
		for _, build := range p.allConfigs.Builds {
			if build.ID == binary.BuildID && build.Type == "gui" {
				isGUI = true
				guiConfig = build.GUI
				break
			}
		}
	}

	// Generate desktop file for GUI apps
	if isGUI {
		iconPath = resolveGUIIconPath(guiConfig, p.distDir, appID)
		desktopFile = filepath.Join(p.distDir, appID+".desktop")
		if err := p.generateDesktopFile(desktopFile, binary, guiConfig, bindir); err != nil {
			log.Warn("Failed to generate desktop file", "error", err)
			isGUI = false // Fall back to non-GUI mode
		}
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
		"IsGUI":        isGUI,
		"AppID":        appID,
		"DesktopFile":  desktopFile,
		"IconPath":     iconPath,
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

// generateDesktopFile creates a .desktop file for GUI applications
func (p *Packager) generateDesktopFile(path string, binary artifact.Artifact, guiConfig *config.GUIConfig, bindir string) error {
	desktopTemplate := `[Desktop Entry]
Type=Application
Name={{ .Name }}
{{ if .GenericName }}GenericName={{ .GenericName }}
{{ end }}{{ if .Comment }}Comment={{ .Comment }}
{{ end }}Exec={{ .Exec }}
{{ if .Icon }}Icon={{ .Icon }}
{{ end }}Terminal={{ .Terminal }}
{{ if .Categories }}Categories={{ .Categories }}
{{ end }}{{ if .Keywords }}Keywords={{ .Keywords }}
{{ end }}{{ if .MimeTypes }}MimeType={{ .MimeTypes }}
{{ end }}StartupNotify={{ .StartupNotify }}
`

	tmpl, err := template.New("desktop").Parse(desktopTemplate)
	if err != nil {
		return err
	}

	name := binary.Name
	comment := p.config.Description
	categories := "Utility;"
	keywords := ""
	genericName := ""
	icon := binary.Name
	terminal := "false"
	startupNotify := "true"
	mimeTypes := ""

	if guiConfig != nil {
		if guiConfig.Name != "" {
			name = guiConfig.Name
		}
		if guiConfig.Comment != "" {
			comment = guiConfig.Comment
		}
		if guiConfig.GenericName != "" {
			genericName = guiConfig.GenericName
		}
		if len(guiConfig.Categories) > 0 {
			categories = strings.Join(guiConfig.Categories, ";") + ";"
		}
		if len(guiConfig.Keywords) > 0 {
			keywords = strings.Join(guiConfig.Keywords, ";") + ";"
		}
		if guiConfig.Terminal {
			terminal = "true"
		}
		if !guiConfig.StartupNotify {
			startupNotify = "false"
		}
		if len(guiConfig.MimeTypes) > 0 {
			mimeTypes = strings.Join(guiConfig.MimeTypes, ";") + ";"
		}
	}

	data := map[string]interface{}{
		"Name":          name,
		"GenericName":   genericName,
		"Comment":       comment,
		"Exec":          filepath.Join(bindir, binary.Name),
		"Icon":          icon,
		"Terminal":      terminal,
		"Categories":    categories,
		"Keywords":      keywords,
		"MimeTypes":     mimeTypes,
		"StartupNotify": startupNotify,
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

// resolveGUIIconPath returns a usable icon path, generating one that reflects
// the application name when the build configuration omits a specific file.
func resolveGUIIconPath(guiConfig *config.GUIConfig, distDir, fallbackName string) string {
	if guiConfig != nil {
		if guiConfig.Icon != "" {
			return guiConfig.Icon
		}
		if fallbackName == "" && guiConfig.Name != "" {
			fallbackName = guiConfig.Name
		}
	}
	icons, err := assets.EnsureAppIcon(fallbackName, distDir)
	if err != nil {
		log.Debug("Falling back without icon", "error", err)
		return ""
	}
	return icons.PNG
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

// runFpmMulti runs fpm as a fallback for multiple binaries.
func (p *Packager) runFpmMulti(ctx context.Context, binaries []artifact.Artifact, name, version, arch, outputPath, format string) error {
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

	// Add all binaries
	for _, binary := range binaries {
		args = append(args, binary.Path+"="+binary.Name)
	}

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
	configs    []config.NFPM
	allConfigs *config.Config
	tmplCtx    *tmpl.Context
	manager    *artifact.Manager
	distDir    string
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

// NewMultiPackagerWithConfig creates a multi-config packager with full config access.
func NewMultiPackagerWithConfig(configs []config.NFPM, allConfigs *config.Config, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *MultiPackager {
	return &MultiPackager{
		configs:    configs,
		allConfigs: allConfigs,
		tmplCtx:    tmplCtx,
		manager:    manager,
		distDir:    distDir,
	}
}

// BuildAll builds all package configurations.
func (m *MultiPackager) BuildAll(ctx context.Context) error {
	for i, cfg := range m.configs {
		log.Info("Building packages", "index", i+1, "total", len(m.configs))
		packager := NewPackagerWithConfig(cfg, m.allConfigs, m.tmplCtx, m.manager, m.distDir)
		if err := packager.Build(ctx); err != nil {
			return err
		}
	}
	return nil
}
