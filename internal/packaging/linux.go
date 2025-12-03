// Package packaging provides Flatpak and AppImage creation for Linux.
package packaging

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

// FlatpakConfig represents Flatpak build configuration
type FlatpakConfig struct {
	ID             string   `yaml:"id,omitempty"`
	AppID          string   `yaml:"app_id"`
	Runtime        string   `yaml:"runtime,omitempty"`
	RuntimeVersion string   `yaml:"runtime_version,omitempty"`
	SDK            string   `yaml:"sdk,omitempty"`
	Command        string   `yaml:"command,omitempty"`
	Permissions    []string `yaml:"permissions,omitempty"`
	Manifest       string   `yaml:"manifest,omitempty"`
	Repo           string   `yaml:"repo,omitempty"`
	Branch         string   `yaml:"branch,omitempty"`
}

// FlatpakBuilder creates Flatpak packages
type FlatpakBuilder struct {
	config  FlatpakConfig
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewFlatpakBuilder creates a new Flatpak builder
func NewFlatpakBuilder(cfg FlatpakConfig, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *FlatpakBuilder {
	return &FlatpakBuilder{
		config:  cfg,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// Build creates a Flatpak package
func (b *FlatpakBuilder) Build(ctx context.Context) error {
	// Check if flatpak-builder is available
	if _, err := exec.LookPath("flatpak-builder"); err != nil {
		log.Warn("Skipping Flatpak: flatpak-builder not found")
		return nil
	}

	log.Info("Building Flatpak package")

	// Get Linux binaries
	binaries := b.manager.Filter(func(a artifact.Artifact) bool {
		return a.Type == artifact.TypeBinary && a.Goos == "linux" && a.Goarch == "amd64"
	})

	if len(binaries) == 0 {
		log.Warn("No Linux amd64 binaries found for Flatpak")
		return nil
	}

	for _, binary := range binaries {
		if err := b.createFlatpak(ctx, binary); err != nil {
			return fmt.Errorf("failed to create Flatpak for %s: %w", binary.Name, err)
		}
	}

	return nil
}

// createFlatpak creates a single Flatpak package
func (b *FlatpakBuilder) createFlatpak(ctx context.Context, binary artifact.Artifact) error {
	appID := b.config.AppID
	if appID == "" {
		appID = fmt.Sprintf("com.%s.%s", b.tmplCtx.Get("ProjectName"), b.tmplCtx.Get("ProjectName"))
	}

	version := b.tmplCtx.Get("Version")

	// Create build directory
	buildDir := filepath.Join(b.distDir, "flatpak-build")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return err
	}

	// Generate manifest if not provided
	manifestPath := b.config.Manifest
	if manifestPath == "" {
		manifestPath = filepath.Join(b.distDir, appID+".json")
		if err := b.generateManifest(manifestPath, binary, appID); err != nil {
			return fmt.Errorf("failed to generate manifest: %w", err)
		}
	}

	// Create repo directory
	repoDir := b.config.Repo
	if repoDir == "" {
		repoDir = filepath.Join(b.distDir, "flatpak-repo")
	}

	// Build flatpak
	args := []string{
		"--force-clean",
		"--repo=" + repoDir,
	}

	if b.config.Branch != "" {
		args = append(args, "--default-branch="+b.config.Branch)
	}

	args = append(args, buildDir, manifestPath)

	cmd := exec.CommandContext(ctx, "flatpak-builder", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = b.distDir

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("flatpak-builder failed: %w", err)
	}

	// Create bundle
	bundleName := fmt.Sprintf("%s_%s.flatpak", appID, version)
	bundlePath := filepath.Join(b.distDir, bundleName)

	branch := b.config.Branch
	if branch == "" {
		branch = "master"
	}

	bundleCmd := exec.CommandContext(ctx, "flatpak", "build-bundle",
		repoDir, bundlePath, appID, branch)
	bundleCmd.Stdout = os.Stdout
	bundleCmd.Stderr = os.Stderr

	if err := bundleCmd.Run(); err != nil {
		return fmt.Errorf("flatpak build-bundle failed: %w", err)
	}

	// Add artifact
	b.manager.Add(artifact.Artifact{
		Name:   bundleName,
		Path:   bundlePath,
		Type:   artifact.TypeLinuxPackage,
		Goos:   "linux",
		Goarch: binary.Goarch,
		Extra: map[string]interface{}{
			"format": "flatpak",
		},
	})

	log.Info("Flatpak created", "name", bundleName)
	return nil
}

// generateManifest generates a Flatpak manifest
func (b *FlatpakBuilder) generateManifest(path string, binary artifact.Artifact, appID string) error {
	runtime := b.config.Runtime
	if runtime == "" {
		runtime = "org.freedesktop.Platform"
	}

	runtimeVersion := b.config.RuntimeVersion
	if runtimeVersion == "" {
		runtimeVersion = "23.08"
	}

	sdk := b.config.SDK
	if sdk == "" {
		sdk = "org.freedesktop.Sdk"
	}

	command := b.config.Command
	if command == "" {
		command = binary.Name
	}

	permissions := b.config.Permissions
	if len(permissions) == 0 {
		permissions = []string{"--share=network", "--share=ipc", "--socket=x11"}
	}

	manifest := fmt.Sprintf(`{
    "app-id": "%s",
    "runtime": "%s",
    "runtime-version": "%s",
    "sdk": "%s",
    "command": "%s",
    "finish-args": [
        %s
    ],
    "modules": [
        {
            "name": "%s",
            "buildsystem": "simple",
            "build-commands": [
                "install -D %s /app/bin/%s"
            ],
            "sources": [
                {
                    "type": "file",
                    "path": "%s"
                }
            ]
        }
    ]
}`,
		appID,
		runtime,
		runtimeVersion,
		sdk,
		command,
		formatPermissions(permissions),
		b.tmplCtx.Get("ProjectName"),
		binary.Name,
		binary.Name,
		binary.Path,
	)

	return os.WriteFile(path, []byte(manifest), 0644)
}

// formatPermissions formats permissions for JSON
func formatPermissions(perms []string) string {
	quoted := make([]string, len(perms))
	for i, p := range perms {
		quoted[i] = fmt.Sprintf(`"%s"`, p)
	}
	return strings.Join(quoted, ",\n        ")
}

// AppImageConfig represents AppImage build configuration
type AppImageConfig struct {
	ID           string             `yaml:"id,omitempty"`
	Name         string             `yaml:"name,omitempty"`
	Icon         string             `yaml:"icon,omitempty"`
	Desktop      string             `yaml:"desktop,omitempty"`
	Categories   []string           `yaml:"categories,omitempty"`
	ExtraFiles   []config.ExtraFile `yaml:"extra_files,omitempty"`
	Architecture string             `yaml:"architecture,omitempty"`
}

// AppImageBuilder creates AppImage packages
type AppImageBuilder struct {
	config  AppImageConfig
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewAppImageBuilder creates a new AppImage builder
func NewAppImageBuilder(cfg AppImageConfig, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *AppImageBuilder {
	return &AppImageBuilder{
		config:  cfg,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// Build creates an AppImage
func (b *AppImageBuilder) Build(ctx context.Context) error {
	// Check if appimagetool is available
	if _, err := exec.LookPath("appimagetool"); err != nil {
		log.Warn("Skipping AppImage: appimagetool not found (install from https://github.com/AppImage/AppImageKit)")
		return nil
	}

	log.Info("Building AppImage")

	// Get Linux binaries
	binaries := b.manager.Filter(func(a artifact.Artifact) bool {
		return a.Type == artifact.TypeBinary && a.Goos == "linux"
	})

	if len(binaries) == 0 {
		log.Warn("No Linux binaries found for AppImage")
		return nil
	}

	for _, binary := range binaries {
		if err := b.createAppImage(ctx, binary); err != nil {
			return fmt.Errorf("failed to create AppImage for %s: %w", binary.Name, err)
		}
	}

	return nil
}

// createAppImage creates a single AppImage
func (b *AppImageBuilder) createAppImage(ctx context.Context, binary artifact.Artifact) error {
	name := b.config.Name
	if name == "" {
		name = b.tmplCtx.Get("ProjectName")
	}

	version := b.tmplCtx.Get("Version")
	arch := binary.Goarch
	if arch == "amd64" {
		arch = "x86_64"
	} else if arch == "386" {
		arch = "i686"
	} else if arch == "arm64" {
		arch = "aarch64"
	}

	// Create AppDir structure
	appDir := filepath.Join(b.distDir, name+".AppDir")
	usrBinDir := filepath.Join(appDir, "usr", "bin")
	usrShareDir := filepath.Join(appDir, "usr", "share", "applications")
	usrIconDir := filepath.Join(appDir, "usr", "share", "icons", "hicolor", "256x256", "apps")

	for _, dir := range []string{usrBinDir, usrShareDir, usrIconDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Copy binary
	if err := copyFile(binary.Path, filepath.Join(usrBinDir, binary.Name)); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}
	if err := os.Chmod(filepath.Join(usrBinDir, binary.Name), 0755); err != nil {
		return err
	}

	// Create desktop file
	desktopFile := b.config.Desktop
	if desktopFile == "" {
		desktopContent := b.generateDesktopFile(name, binary.Name)
		desktopPath := filepath.Join(usrShareDir, name+".desktop")
		if err := os.WriteFile(desktopPath, []byte(desktopContent), 0644); err != nil {
			return err
		}
		// Also link to AppDir root
		if err := os.Symlink(filepath.Join("usr", "share", "applications", name+".desktop"),
			filepath.Join(appDir, name+".desktop")); err != nil {
			return err
		}
	}

	iconPath := b.config.Icon
	if iconPath == "" {
		if iconSet, err := assets.EnsureAppIcon(name, b.distDir); err == nil {
			iconPath = iconSet.PNG
		} else {
			log.Warn("Failed to generate default icon", "error", err)
		}
	}
	if iconPath != "" {
		ext := filepath.Ext(iconPath)
		if ext == "" {
			ext = ".png"
		}
		iconDest := filepath.Join(usrIconDir, name+ext)
		if err := copyFile(iconPath, iconDest); err != nil {
			log.Warn("Failed to copy icon", "error", err)
		} else {
			_ = os.Symlink(filepath.Join("usr", "share", "icons", "hicolor", "256x256", "apps", name+ext),
				filepath.Join(appDir, name+ext))
		}
	}

	// Create AppRun script
	appRunPath := filepath.Join(appDir, "AppRun")
	appRunContent := fmt.Sprintf(`#!/bin/bash
SELF=$(readlink -f "$0")
HERE=${SELF%%/*}
export PATH="${HERE}/usr/bin/:${PATH}"
exec "%s" "$@"
`, binary.Name)
	if err := os.WriteFile(appRunPath, []byte(appRunContent), 0755); err != nil {
		return err
	}

	// Run appimagetool
	appImageName := fmt.Sprintf("%s-%s-%s.AppImage", name, version, arch)
	appImagePath := filepath.Join(b.distDir, appImageName)

	cmd := exec.CommandContext(ctx, "appimagetool", appDir, appImagePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "ARCH="+arch)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("appimagetool failed: %w", err)
	}

	// Clean up AppDir
	os.RemoveAll(appDir)

	// Add artifact
	b.manager.Add(artifact.Artifact{
		Name:   appImageName,
		Path:   appImagePath,
		Type:   artifact.TypeLinuxPackage,
		Goos:   "linux",
		Goarch: binary.Goarch,
		Extra: map[string]interface{}{
			"format": "appimage",
		},
	})

	log.Info("AppImage created", "name", appImageName)
	return nil
}

// generateDesktopFile generates a .desktop file
func (b *AppImageBuilder) generateDesktopFile(name, executable string) string {
	categories := b.config.Categories
	if len(categories) == 0 {
		categories = []string{"Utility"}
	}

	return fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%s
Icon=%s
Categories=%s;
Terminal=false
`,
		name,
		executable,
		name,
		strings.Join(categories, ";"),
	)
}

// SnapBuilder creates Snap packages
type SnapBuilder struct {
	config  config.Snapcraft
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewSnapBuilder creates a new Snap builder
func NewSnapBuilder(cfg config.Snapcraft, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *SnapBuilder {
	return &SnapBuilder{
		config:  cfg,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// Build creates a Snap package
func (b *SnapBuilder) Build(ctx context.Context) error {
	if b.config.Skip == "true" {
		log.Info("Skipping Snap build")
		return nil
	}

	// Check if snapcraft is available
	if _, err := exec.LookPath("snapcraft"); err != nil {
		log.Warn("Skipping Snap: snapcraft not found")
		return nil
	}

	log.Info("Building Snap package")

	// Create snap directory
	snapDir := filepath.Join(b.distDir, "snap")
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		return err
	}

	// Generate snapcraft.yaml
	snapcraftPath := filepath.Join(snapDir, "snapcraft.yaml")
	if err := b.generateSnapcraft(snapcraftPath); err != nil {
		return fmt.Errorf("failed to generate snapcraft.yaml: %w", err)
	}

	// Run snapcraft
	cmd := exec.CommandContext(ctx, "snapcraft", "--destructive-mode")
	cmd.Dir = b.distDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("snapcraft failed: %w", err)
	}

	// Find and register snap artifact
	snapFiles, _ := filepath.Glob(filepath.Join(b.distDir, "*.snap"))
	for _, snapFile := range snapFiles {
		b.manager.Add(artifact.Artifact{
			Name:   filepath.Base(snapFile),
			Path:   snapFile,
			Type:   artifact.TypeLinuxPackage,
			Goos:   "linux",
			Goarch: "amd64",
			Extra: map[string]interface{}{
				"format": "snap",
			},
		})
		log.Info("Snap created", "name", filepath.Base(snapFile))
	}

	return nil
}

// generateSnapcraft generates snapcraft.yaml
func (b *SnapBuilder) generateSnapcraft(path string) error {
	tmplText := `name: {{ .Name }}
version: '{{ .Version }}'
summary: {{ .Summary }}
description: |
  {{ .Description }}
grade: {{ .Grade }}
confinement: {{ .Confinement }}
base: {{ .Base }}

apps:
{{ range $name, $app := .Apps }}
  {{ $name }}:
    command: {{ $app.Command }}
    plugs:
{{ range $app.Plugs }}
      - {{ . }}
{{ end }}
{{ if $app.Daemon }}
    daemon: {{ $app.Daemon }}
{{ end }}
{{ end }}

parts:
  {{ .Name }}:
    plugin: dump
    source: .
    stage-packages:
      - libc6
`

	t, err := template.New("snapcraft").Parse(tmplText)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	name := b.config.Name
	if name == "" {
		name = b.tmplCtx.Get("ProjectName")
	}

	grade := b.config.Grade
	if grade == "" {
		grade = "stable"
	}

	confinement := b.config.Confinement
	if confinement == "" {
		confinement = "strict"
	}

	base := b.config.Base
	if base == "" {
		base = "core22"
	}

	data := map[string]interface{}{
		"Name":        name,
		"Version":     strings.TrimPrefix(b.tmplCtx.Get("Version"), "v"),
		"Summary":     b.config.Summary,
		"Description": b.config.Description,
		"Grade":       grade,
		"Confinement": confinement,
		"Base":        base,
		"Apps":        b.config.Apps,
	}

	return t.Execute(f, data)
}

// BuildAllFlatpaks builds all configured Flatpak packages
func BuildAllFlatpaks(ctx context.Context, cfg *config.Config, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) error {
	if len(cfg.Flatpaks) == 0 {
		return nil
	}

	log.Info("Building Flatpak packages", "count", len(cfg.Flatpaks))

	for _, flatpakCfg := range cfg.Flatpaks {
		if flatpakCfg.Skip == "true" {
			log.Info("Skipping Flatpak", "id", flatpakCfg.ID)
			continue
		}

		builder := NewFlatpakBuilder(FlatpakConfig{
			ID:             flatpakCfg.ID,
			AppID:          flatpakCfg.AppID,
			Runtime:        flatpakCfg.Runtime,
			RuntimeVersion: flatpakCfg.RuntimeVersion,
			SDK:            flatpakCfg.SDK,
			Command:        flatpakCfg.Command,
			Permissions:    flatpakCfg.FinishArgs,
		}, tmplCtx, manager, distDir)

		if err := builder.Build(ctx); err != nil {
			return fmt.Errorf("failed to build Flatpak %s: %w", flatpakCfg.ID, err)
		}
	}

	return nil
}

// BuildAllAppImages builds all configured AppImage packages
func BuildAllAppImages(ctx context.Context, cfg *config.Config, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) error {
	if len(cfg.AppImages) == 0 {
		return nil
	}

	log.Info("Building AppImage packages", "count", len(cfg.AppImages))

	for _, appImageCfg := range cfg.AppImages {
		if appImageCfg.Skip == "true" {
			log.Info("Skipping AppImage", "id", appImageCfg.ID)
			continue
		}

		// Convert categories from string to slice if needed
		var categories []string
		if appImageCfg.Categories != "" {
			categories = strings.Split(appImageCfg.Categories, ";")
		}

		builder := NewAppImageBuilder(AppImageConfig{
			ID:         appImageCfg.ID,
			Name:       appImageCfg.Name,
			Icon:       appImageCfg.Icon,
			Desktop:    appImageCfg.Desktop,
			Categories: categories,
			ExtraFiles: appImageCfg.ExtraFiles,
		}, tmplCtx, manager, distDir)

		if err := builder.Build(ctx); err != nil {
			return fmt.Errorf("failed to build AppImage %s: %w", appImageCfg.ID, err)
		}
	}

	return nil
}

// BuildAllSnaps builds all configured Snap packages
func BuildAllSnaps(ctx context.Context, cfg *config.Config, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) error {
	if len(cfg.Snapcrafts) == 0 {
		return nil
	}

	log.Info("Building Snap packages", "count", len(cfg.Snapcrafts))

	for _, snapCfg := range cfg.Snapcrafts {
		if snapCfg.Skip == "true" {
			log.Info("Skipping Snap", "id", snapCfg.ID)
			continue
		}

		builder := NewSnapBuilder(snapCfg, tmplCtx, manager, distDir)

		if err := builder.Build(ctx); err != nil {
			return fmt.Errorf("failed to build Snap %s: %w", snapCfg.ID, err)
		}
	}

	return nil
}
