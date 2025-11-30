// Package packaging provides macOS-specific packaging (PKG, Universal Binary).
package packaging

import (
	"bytes"
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

// PKGBuilder creates macOS PKG installers.
type PKGBuilder struct {
	config  config.PKG
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewPKGBuilder creates a new PKG builder.
func NewPKGBuilder(cfg config.PKG, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *PKGBuilder {
	return &PKGBuilder{
		config:  cfg,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// Build creates a macOS PKG installer.
func (b *PKGBuilder) Build(ctx context.Context) error {
	log.Info("Building macOS PKG installer")

	// Get app bundles or darwin binaries
	var sources []artifact.Artifact
	if b.config.AppBundle != "" {
		// Use specified app bundle
		sources = b.manager.Filter(func(a artifact.Artifact) bool {
			return a.Type == artifact.TypeAppBundle && a.Name == b.config.AppBundle+".app"
		})
	} else {
		// Use app bundles
		sources = b.manager.Filter(artifact.ByType(artifact.TypeAppBundle))
	}

	if len(sources) == 0 {
		// Fall back to darwin binaries
		sources = b.manager.Filter(func(a artifact.Artifact) bool {
			return a.Type == artifact.TypeBinary && a.Goos == "darwin"
		})
	}

	if len(sources) == 0 {
		log.Debug("No darwin artifacts found for PKG creation, skipping")
		return nil
	}

	// Check if pkgbuild is available
	hasPkgbuild := false
	if _, err := exec.LookPath("pkgbuild"); err == nil {
		hasPkgbuild = true
	}

	for _, source := range sources {
		if hasPkgbuild {
			if err := b.createPKG(ctx, source); err != nil {
				return fmt.Errorf("failed to create PKG for %s: %w", source.Name, err)
			}
		} else {
			// Create an installer package structure for later use on macOS
			if err := b.createPkgStructure(ctx, source); err != nil {
				return fmt.Errorf("failed to create PKG structure for %s: %w", source.Name, err)
			}
		}
	}

	return nil
}

// createPkgStructure creates a PKG-like structure when pkgbuild isn't available
func (b *PKGBuilder) createPkgStructure(ctx context.Context, source artifact.Artifact) error {
	name := b.config.Name
	if name == "" {
		name = b.tmplCtx.Get("ProjectName")
	}

	version := b.config.Version
	if version == "" {
		version = b.tmplCtx.Get("Version")
	}

	identifier := b.config.Identifier
	if identifier == "" {
		identifier = fmt.Sprintf("com.example.%s", name)
	}

	// Create a tar.gz package with macOS installer structure
	pkgDirName := fmt.Sprintf("%s_%s_%s_macos_pkg", name, version, source.Goarch)
	pkgDir := filepath.Join(b.distDir, pkgDirName)

	// Create standard macOS package structure
	var payloadDir, scriptsDir string
	var binaryPath string

	if source.Type == artifact.TypeAppBundle {
		// For app bundles, install to /Applications
		payloadDir = filepath.Join(pkgDir, "payload", "Applications")
		scriptsDir = filepath.Join(pkgDir, "scripts")
		binaryPath = source.Path
	} else {
		// For binaries, install to /usr/local/bin
		payloadDir = filepath.Join(pkgDir, "payload", "usr", "local", "bin")
		scriptsDir = filepath.Join(pkgDir, "scripts")
		binaryPath = source.Path
	}

	for _, dir := range []string{payloadDir, scriptsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Copy source (binary or app bundle)
	if source.Type == artifact.TypeAppBundle {
		// Copy entire app bundle directory
		destPath := filepath.Join(payloadDir, filepath.Base(source.Path))
		if err := copyDirPkg(source.Path, destPath); err != nil {
			return err
		}
	} else {
		// Copy single binary file
		binaryDest := filepath.Join(payloadDir, filepath.Base(binaryPath))
		if err := copyFilePkg(binaryPath, binaryDest); err != nil {
			return err
		}
		os.Chmod(binaryDest, 0755)
	}

	// Create postinstall script
	postinstall := fmt.Sprintf(`#!/bin/bash
echo "Installing %s..."
chmod +x /usr/local/bin/%s
echo "Installation complete!"
`, name, filepath.Base(source.Path))
	os.WriteFile(filepath.Join(scriptsDir, "postinstall"), []byte(postinstall), 0755)

	// Create Distribution.xml for productbuild
	distXml := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<installer-gui-script minSpecVersion="1">
    <title>%s</title>
    <organization>%s</organization>
    <domains enable_localSystem="true"/>
    <options customize="never" require-scripts="true" rootVolumeOnly="true"/>
    <pkg-ref id="%s"/>
    <choices-outline>
        <line choice="default">
            <line choice="%s"/>
        </line>
    </choices-outline>
    <choice id="default"/>
    <choice id="%s" visible="false">
        <pkg-ref id="%s"/>
    </choice>
    <pkg-ref id="%s" version="%s" onConclusion="none">%s.pkg</pkg-ref>
</installer-gui-script>
`, name, identifier, identifier, identifier, identifier, identifier, identifier, version, name)
	os.WriteFile(filepath.Join(pkgDir, "Distribution.xml"), []byte(distXml), 0644)

	// Create README
	readme := fmt.Sprintf(`# %s macOS Installer

This package can be built into a .pkg installer on macOS using:

    cd %s
    pkgbuild --root payload --scripts scripts --identifier %s --version %s %s.pkg
    productbuild --distribution Distribution.xml --package-path . %s_%s.pkg

Or install manually:
    cp payload/usr/local/bin/* /usr/local/bin/
`, name, pkgDirName, identifier, version, name, name, version)
	os.WriteFile(filepath.Join(pkgDir, "README.txt"), []byte(readme), 0644)

	// Create tar.gz of the structure
	tarFileName := pkgDirName + ".tar.gz"
	tarPath := filepath.Join(b.distDir, tarFileName)

	cmd := exec.CommandContext(ctx, "tar", "-czf", tarPath, "-C", b.distDir, pkgDirName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create tar.gz: %w", err)
	}

	// Clean up the directory
	os.RemoveAll(pkgDir)

	// Add artifact
	b.manager.Add(artifact.Artifact{
		Name:   tarFileName,
		Path:   tarPath,
		Type:   artifact.TypeArchive,
		Goos:   "darwin",
		Goarch: source.Goarch,
		Extra: map[string]interface{}{
			"format":    "tar.gz",
			"installer": true,
			"pkg_ready": true,
		},
	})

	log.Info("macOS PKG installer package created (compile on macOS with pkgbuild)", "name", tarFileName)
	return nil
}

// createPKG creates a PKG from a source artifact.
func (b *PKGBuilder) createPKG(ctx context.Context, source artifact.Artifact) error {
	name := b.config.Name
	if name == "" {
		name = b.tmplCtx.Get("ProjectName")
	}

	version := b.config.Version
	if version == "" {
		version = b.tmplCtx.Get("Version")
	}

	identifier := b.config.Identifier
	if identifier == "" {
		identifier = fmt.Sprintf("com.example.%s", name)
	}

	pkgFileName := fmt.Sprintf("%s_%s_%s.pkg", name, version, source.Goarch)
	pkgPath := filepath.Join(b.distDir, pkgFileName)

	// Determine install location
	installLocation := b.config.InstallLocation
	if installLocation == "" {
		if source.Type == artifact.TypeAppBundle {
			installLocation = "/Applications"
		} else {
			installLocation = "/usr/local/bin"
		}
	}

	// Create scripts directory if needed
	var scriptsDir string
	if b.config.Scripts.PreInstall != "" || b.config.Scripts.PostInstall != "" {
		var err error
		scriptsDir, err = os.MkdirTemp("", "pkg-scripts-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(scriptsDir)

		if b.config.Scripts.PreInstall != "" {
			if err := copyFile(b.config.Scripts.PreInstall, filepath.Join(scriptsDir, "preinstall")); err != nil {
				return fmt.Errorf("failed to copy preinstall script: %w", err)
			}
			os.Chmod(filepath.Join(scriptsDir, "preinstall"), 0755)
		}
		if b.config.Scripts.PostInstall != "" {
			if err := copyFile(b.config.Scripts.PostInstall, filepath.Join(scriptsDir, "postinstall")); err != nil {
				return fmt.Errorf("failed to copy postinstall script: %w", err)
			}
			os.Chmod(filepath.Join(scriptsDir, "postinstall"), 0755)
		}
	}

	// Build component package first
	componentPkgPath := pkgPath
	if b.config.Distribution != "" || len(b.config.ExtraFiles) > 0 {
		componentPkgPath = filepath.Join(b.distDir, fmt.Sprintf("%s-component.pkg", name))
	}

	// Build pkgbuild arguments
	args := []string{
		"--root", filepath.Dir(source.Path),
		"--identifier", identifier,
		"--version", version,
		"--install-location", installLocation,
	}

	if scriptsDir != "" {
		args = append(args, "--scripts", scriptsDir)
	}

	// Sign if configured
	if b.config.Sign.Identity != "" {
		args = append(args, "--sign", b.config.Sign.Identity)
		if b.config.Sign.Keychain != "" {
			args = append(args, "--keychain", b.config.Sign.Keychain)
		}
	}

	args = append(args, componentPkgPath)

	// Run pkgbuild
	cmd := exec.CommandContext(ctx, "pkgbuild", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	log.Debug("Running pkgbuild", "args", args)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pkgbuild failed: %w\n%s", err, stderr.String())
	}

	// If we need a distribution package, use productbuild
	if b.config.Distribution != "" || componentPkgPath != pkgPath {
		if err := b.createDistributionPkg(ctx, componentPkgPath, pkgPath, name, version); err != nil {
			os.Remove(componentPkgPath)
			return err
		}
		os.Remove(componentPkgPath)
	}

	// Notarize if configured
	if b.config.Notarize.Enabled {
		if err := b.notarizePKG(ctx, pkgPath); err != nil {
			log.Warn("Notarization failed", "error", err)
		}
	}

	// Add artifact
	b.manager.Add(artifact.Artifact{
		Name:   pkgFileName,
		Path:   pkgPath,
		Type:   artifact.TypePKG,
		Goos:   "darwin",
		Goarch: source.Goarch,
	})

	log.Info("PKG created", "name", pkgFileName)
	return nil
}

// createDistributionPkg creates a distribution package with productbuild.
func (b *PKGBuilder) createDistributionPkg(ctx context.Context, componentPkg, outputPkg, name, version string) error {
	args := []string{
		"--package", componentPkg,
	}

	// Add distribution file if specified
	if b.config.Distribution != "" {
		args = append(args, "--distribution", b.config.Distribution)
	}

	// Add resources if specified
	if b.config.Resources != "" {
		args = append(args, "--resources", b.config.Resources)
	}

	// Sign if configured
	if b.config.Sign.Identity != "" {
		args = append(args, "--sign", b.config.Sign.Identity)
		if b.config.Sign.Keychain != "" {
			args = append(args, "--keychain", b.config.Sign.Keychain)
		}
	}

	args = append(args, outputPkg)

	cmd := exec.CommandContext(ctx, "productbuild", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	log.Debug("Running productbuild", "args", args)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("productbuild failed: %w\n%s", err, stderr.String())
	}

	return nil
}

// notarizePKG notarizes the PKG with Apple.
func (b *PKGBuilder) notarizePKG(ctx context.Context, pkgPath string) error {
	cfg := b.config.Notarize

	args := []string{
		"notarytool", "submit",
		pkgPath,
		"--apple-id", cfg.AppleID,
		"--password", cfg.Password,
		"--team-id", cfg.TeamID,
		"--wait",
	}

	cmd := exec.CommandContext(ctx, "xcrun", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("notarization failed: %w\n%s", err, stderr.String())
	}

	// Staple if configured
	if cfg.Staple {
		stapleCmd := exec.CommandContext(ctx, "xcrun", "stapler", "staple", pkgPath)
		if err := stapleCmd.Run(); err != nil {
			return fmt.Errorf("stapling failed: %w", err)
		}
	}

	log.Info("PKG notarized successfully")
	return nil
}

// UniversalBinaryBuilder creates macOS universal binaries.
type UniversalBinaryBuilder struct {
	config  config.UniversalBinary
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewUniversalBinaryBuilder creates a new universal binary builder.
func NewUniversalBinaryBuilder(cfg config.UniversalBinary, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *UniversalBinaryBuilder {
	return &UniversalBinaryBuilder{
		config:  cfg,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// Build creates macOS universal binaries by combining amd64 and arm64 binaries.
func (b *UniversalBinaryBuilder) Build(ctx context.Context) error {
	// Universal binary creation requires lipo (available on macOS or via cctools)
	lipoPath := "lipo"
	if _, err := exec.LookPath("lipo"); err != nil {
		// Try x86_64-apple-darwin-lipo (cctools)
		if _, err := exec.LookPath("x86_64-apple-darwin-lipo"); err == nil {
			lipoPath = "x86_64-apple-darwin-lipo"
		} else {
			log.Warn("Skipping universal binary creation: lipo not found")
			return nil
		}
	}

	log.Info("Building macOS universal binaries")

	// Get darwin binaries
	darwinBinaries := b.manager.Filter(func(a artifact.Artifact) bool {
		return a.Type == artifact.TypeBinary && a.Goos == "darwin"
	})

	if len(darwinBinaries) == 0 {
		log.Debug("No darwin binaries found for universal binary creation, skipping")
		return nil
	}

	// Group binaries by name
	binaryGroups := make(map[string][]artifact.Artifact)
	for _, bin := range darwinBinaries {
		// Filter by IDs if specified
		if len(b.config.IDs) > 0 {
			found := false
			for _, id := range b.config.IDs {
				if bin.BuildID == id {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		binaryGroups[bin.Name] = append(binaryGroups[bin.Name], bin)
	}

	// Create universal binary for each group
	for name, binaries := range binaryGroups {
		if len(binaries) < 2 {
			log.Debug("Skipping universal binary: need both amd64 and arm64", "name", name)
			continue
		}

		// Find amd64 and arm64 binaries
		var amd64Path, arm64Path string
		for _, bin := range binaries {
			if bin.Goarch == "amd64" {
				amd64Path = bin.Path
			} else if bin.Goarch == "arm64" {
				arm64Path = bin.Path
			}
		}

		if amd64Path == "" || arm64Path == "" {
			log.Debug("Skipping universal binary: missing architecture", "name", name)
			continue
		}

		if err := b.createUniversalBinary(ctx, lipoPath, name, amd64Path, arm64Path); err != nil {
			return fmt.Errorf("failed to create universal binary for %s: %w", name, err)
		}
	}

	return nil
}

// createUniversalBinary creates a single universal binary.
func (b *UniversalBinaryBuilder) createUniversalBinary(ctx context.Context, lipoPath, name, amd64Path, arm64Path string) error {
	// Determine output path
	nameTemplate := b.config.NameTemplate
	if nameTemplate == "" {
		nameTemplate = "{{ .ProjectName }}_universal"
	}

	outputName, err := b.tmplCtx.Apply(nameTemplate)
	if err != nil {
		outputName = name + "_universal"
	}

	// Create output directory
	outputDir := filepath.Join(b.distDir, fmt.Sprintf("%s_darwin_universal", outputName))
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	outputPath := filepath.Join(outputDir, name)

	// Run lipo to create universal binary
	args := []string{
		"-create",
		"-output", outputPath,
		amd64Path,
		arm64Path,
	}

	cmd := exec.CommandContext(ctx, lipoPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	log.Debug("Running lipo", "args", args)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("lipo failed: %w\n%s", err, stderr.String())
	}

	// Make executable
	if err := os.Chmod(outputPath, 0755); err != nil {
		return err
	}

	// Add artifact
	b.manager.Add(artifact.Artifact{
		Name:   name,
		Path:   outputPath,
		Type:   artifact.TypeUniversalBinary,
		Goos:   "darwin",
		Goarch: "universal",
	})

	// Replace original binaries if configured
	if b.config.Replace {
		// Remove original binaries from artifact list
		b.manager.Remove(func(a artifact.Artifact) bool {
			return a.Type == artifact.TypeBinary && a.Goos == "darwin" && a.Name == name
		})
	}

	log.Info("Universal binary created", "name", name)
	return nil
}

// BuildAllPKGs builds PKGs for all configurations.
func BuildAllPKGs(ctx context.Context, configs []config.PKG, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) error {
	for i, cfg := range configs {
		log.Info("Building PKG", "index", i+1, "total", len(configs))
		builder := NewPKGBuilder(cfg, tmplCtx, manager, distDir)
		if err := builder.Build(ctx); err != nil {
			return err
		}
	}
	return nil
}

// BuildAllUniversalBinaries builds universal binaries for all configurations.
func BuildAllUniversalBinaries(ctx context.Context, configs []config.UniversalBinary, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) error {
	for i, cfg := range configs {
		log.Info("Building universal binary", "index", i+1, "total", len(configs))
		builder := NewUniversalBinaryBuilder(cfg, tmplCtx, manager, distDir)
		if err := builder.Build(ctx); err != nil {
			return err
		}
	}
	return nil
}

// copyFilePkg copies a file from src to dst (used in PKG creation).
func copyFilePkg(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}

// copyDirPkg recursively copies a directory.
func copyDirPkg(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}
