// Package packaging provides macOS App Bundle, DMG, and Windows installer creation.
package packaging

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"

	"github.com/charmbracelet/log"
	"github.com/oarkflow/releaser/internal/artifact"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// AppBundleBuilder creates macOS App Bundles.
type AppBundleBuilder struct {
	config  config.AppBundle
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewAppBundleBuilder creates a new App Bundle builder.
func NewAppBundleBuilder(cfg config.AppBundle, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *AppBundleBuilder {
	return &AppBundleBuilder{
		config:  cfg,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// Build creates a macOS .app bundle.
func (b *AppBundleBuilder) Build(ctx context.Context) error {
	log.Info("Building macOS App Bundle")

	// Get darwin binaries
	binaries := b.manager.Filter(func(a artifact.Artifact) bool {
		return a.Type == artifact.TypeBinary && a.Goos == "darwin"
	})

	if len(binaries) == 0 {
		log.Warn("No darwin binaries found for App Bundle")
		return nil
	}

	for _, binary := range binaries {
		if err := b.createAppBundle(ctx, binary); err != nil {
			return fmt.Errorf("failed to create App Bundle for %s: %w", binary.Name, err)
		}
	}

	return nil
}

// createAppBundle creates a single App Bundle.
func (b *AppBundleBuilder) createAppBundle(ctx context.Context, binary artifact.Artifact) error {
	appName := b.config.Name
	if appName == "" {
		appName = b.tmplCtx.Get("ProjectName")
	}
	appName += ".app"

	bundleID := b.config.Identifier
	if bundleID == "" {
		bundleID = fmt.Sprintf("com.example.%s", b.tmplCtx.Get("ProjectName"))
	}

	version := b.config.Version
	if version == "" {
		version = b.tmplCtx.Get("Version")
	}

	// Create app bundle structure
	appPath := filepath.Join(b.distDir, appName)
	contentsPath := filepath.Join(appPath, "Contents")
	macOSPath := filepath.Join(contentsPath, "MacOS")
	resourcesPath := filepath.Join(contentsPath, "Resources")

	for _, dir := range []string{macOSPath, resourcesPath} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Copy binary
	execName := filepath.Base(binary.Path)
	execPath := filepath.Join(macOSPath, execName)
	if err := copyFile(binary.Path, execPath); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}
	if err := os.Chmod(execPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permission: %w", err)
	}

	// Create Info.plist
	plistPath := filepath.Join(contentsPath, "Info.plist")
	if err := b.createInfoPlist(plistPath, execName, bundleID, appName, version); err != nil {
		return fmt.Errorf("failed to create Info.plist: %w", err)
	}

	// Copy icon if provided
	if b.config.Icon != "" {
		iconPath := filepath.Join(resourcesPath, "icon.icns")
		if err := copyFile(b.config.Icon, iconPath); err != nil {
			log.Warn("Failed to copy icon", "error", err)
		}
	}

	// Copy extra files
	for _, file := range b.config.ExtraFiles {
		dst := file.Dst
		if dst == "" {
			dst = filepath.Join(resourcesPath, filepath.Base(file.Src))
		} else {
			dst = filepath.Join(contentsPath, dst)
		}
		if err := copyFile(file.Src, dst); err != nil {
			log.Warn("Failed to copy extra file", "src", file.Src, "error", err)
		}
	}

	// Add artifact
	b.manager.Add(artifact.Artifact{
		Name:   appName,
		Path:   appPath,
		Type:   artifact.TypeAppBundle,
		Goos:   "darwin",
		Goarch: binary.Goarch,
	})

	log.Info("App Bundle created", "name", appName)
	return nil
}

// createInfoPlist creates the Info.plist file.
func (b *AppBundleBuilder) createInfoPlist(path, execName, bundleID, appName, version string) error {
	plistTemplate := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>{{ .ExecName }}</string>
	<key>CFBundleIdentifier</key>
	<string>{{ .BundleID }}</string>
	<key>CFBundleName</key>
	<string>{{ .Name }}</string>
	<key>CFBundleVersion</key>
	<string>{{ .Version }}</string>
	<key>CFBundleShortVersionString</key>
	<string>{{ .Version }}</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleSignature</key>
	<string>????</string>
	<key>CFBundleIconFile</key>
	<string>icon</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>LSMinimumSystemVersion</key>
	<string>10.13</string>
</dict>
</plist>
`

	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, map[string]string{
		"ExecName": execName,
		"BundleID": bundleID,
		"Name":     appName,
		"Version":  version,
	})
}

// DMGBuilder creates macOS DMG disk images.
type DMGBuilder struct {
	config  config.DMG
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewDMGBuilder creates a new DMG builder.
func NewDMGBuilder(cfg config.DMG, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *DMGBuilder {
	return &DMGBuilder{
		config:  cfg,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// Build creates a DMG disk image.
func (b *DMGBuilder) Build(ctx context.Context) error {
	// DMG creation requires macOS with hdiutil
	if runtime.GOOS != "darwin" {
		// Check if hdiutil is available (might be running via cross-compilation tools)
		if _, err := exec.LookPath("hdiutil"); err != nil {
			log.Warn("Skipping DMG creation: hdiutil is only available on macOS")
			return nil
		}
	}

	log.Info("Building DMG disk image")

	// Get app bundles
	appBundles := b.manager.Filter(artifact.ByType(artifact.TypeAppBundle))

	if len(appBundles) == 0 {
		log.Warn("No App Bundles found for DMG creation")
		return nil
	}

	for _, app := range appBundles {
		if err := b.createDMG(ctx, app); err != nil {
			return fmt.Errorf("failed to create DMG for %s: %w", app.Name, err)
		}
	}

	return nil
}

// createDMG creates a DMG from an app bundle.
func (b *DMGBuilder) createDMG(ctx context.Context, app artifact.Artifact) error {
	dmgName := b.config.Name
	if dmgName == "" {
		dmgName = b.tmplCtx.Get("ProjectName")
	}

	version := b.tmplCtx.Get("Version")
	dmgFileName := fmt.Sprintf("%s_%s_%s.dmg", dmgName, version, app.Goarch)
	dmgPath := filepath.Join(b.distDir, dmgFileName)

	// Create temporary directory for DMG contents
	tmpDir, err := os.MkdirTemp("", "dmg-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Copy app bundle to temp directory
	appDest := filepath.Join(tmpDir, filepath.Base(app.Path))
	if err := copyDir(app.Path, appDest); err != nil {
		return fmt.Errorf("failed to copy app bundle: %w", err)
	}

	// Create Applications symlink if configured
	if b.config.ApplicationsSymlink {
		appsLink := filepath.Join(tmpDir, "Applications")
		if err := os.Symlink("/Applications", appsLink); err != nil {
			return fmt.Errorf("failed to create Applications symlink: %w", err)
		}
	}

	// Add extra contents
	for _, content := range b.config.Contents {
		dst := filepath.Join(tmpDir, filepath.Base(content.Src))
		if err := copyFile(content.Src, dst); err != nil {
			log.Warn("Failed to copy DMG content", "src", content.Src, "error", err)
		}
	}

	// Create DMG using hdiutil
	format := b.config.Format
	if format == "" {
		format = "UDZO"
	}

	cmd := exec.CommandContext(ctx, "hdiutil", "create",
		"-volname", dmgName,
		"-srcfolder", tmpDir,
		"-ov",
		"-format", format,
		dmgPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hdiutil failed: %w", err)
	}

	// Add artifact
	b.manager.Add(artifact.Artifact{
		Name:   dmgFileName,
		Path:   dmgPath,
		Type:   artifact.TypeDMG,
		Goos:   "darwin",
		Goarch: app.Goarch,
	})

	log.Info("DMG created", "name", dmgFileName)
	return nil
}

// MSIBuilder creates Windows MSI installers.
type MSIBuilder struct {
	config  config.MSI
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewMSIBuilder creates a new MSI builder.
func NewMSIBuilder(cfg config.MSI, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *MSIBuilder {
	return &MSIBuilder{
		config:  cfg,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// Build creates an MSI installer.
func (b *MSIBuilder) Build(ctx context.Context) error {
	// Check if MSI tools are available (wixl on Linux or WiX on Windows)
	hasWixl := false
	hasWix := false
	if _, err := exec.LookPath("wixl"); err == nil {
		hasWixl = true
	}
	if _, err := exec.LookPath("candle"); err == nil {
		hasWix = true
	}
	if !hasWixl && !hasWix {
		log.Warn("Skipping MSI creation: neither wixl (msitools) nor WiX Toolset found")
		return nil
	}

	log.Info("Building MSI installer")

	// Get windows binaries
	binaries := b.manager.Filter(func(a artifact.Artifact) bool {
		return a.Type == artifact.TypeBinary && a.Goos == "windows"
	})

	if len(binaries) == 0 {
		log.Warn("No Windows binaries found for MSI")
		return nil
	}

	for _, binary := range binaries {
		if err := b.createMSI(ctx, binary); err != nil {
			return fmt.Errorf("failed to create MSI for %s: %w", binary.Name, err)
		}
	}

	return nil
}

// createMSI creates an MSI installer.
func (b *MSIBuilder) createMSI(ctx context.Context, binary artifact.Artifact) error {
	name := b.config.Name
	if name == "" {
		name = b.tmplCtx.Get("ProjectName")
	}

	version := b.config.ProductVersion
	if version == "" {
		version = b.tmplCtx.Get("Version")
	}

	msiFileName := fmt.Sprintf("%s_%s_%s.msi", name, version, binary.Goarch)
	msiPath := filepath.Join(b.distDir, msiFileName)

	// Check if custom WXS file is provided
	wxsPath := b.config.WXS
	if wxsPath == "" {
		// Generate WiX source file
		wxsPath = filepath.Join(b.distDir, fmt.Sprintf("%s.wxs", name))
		if err := b.generateWxs(wxsPath, binary, name, version); err != nil {
			return fmt.Errorf("failed to generate WiX source: %w", err)
		}
	}

	// Try wixl (GNOME msitools) first, then WiX Toolset
	if err := b.runWixl(ctx, wxsPath, msiPath); err != nil {
		log.Debug("wixl not available, trying WiX Toolset", "error", err)
		if err := b.runWix(ctx, wxsPath, msiPath); err != nil {
			return fmt.Errorf("failed to create MSI: %w", err)
		}
	}

	// Add artifact
	b.manager.Add(artifact.Artifact{
		Name:   msiFileName,
		Path:   msiPath,
		Type:   artifact.TypeMSI,
		Goos:   "windows",
		Goarch: binary.Goarch,
	})

	log.Info("MSI created", "name", msiFileName)
	return nil
}

// generateWxs generates a WiX source file.
func (b *MSIBuilder) generateWxs(path string, binary artifact.Artifact, name, version string) error {
	wxsTemplate := `<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
  <Product Id="*" Name="{{ .Name }}" Language="1033" Version="{{ .Version }}"
           Manufacturer="{{ .Manufacturer }}" UpgradeCode="{{ .UpgradeCode }}">
    <Package InstallerVersion="200" Compressed="yes" InstallScope="perMachine" />
    <MediaTemplate EmbedCab="yes" />
    <Feature Id="Complete" Level="1">
      <ComponentRef Id="MainExecutable" />
    </Feature>
    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="ProgramFilesFolder">
        <Directory Id="INSTALLDIR" Name="{{ .Name }}">
          <Component Id="MainExecutable" Guid="*">
            <File Id="exe0" Source="{{ .BinaryPath }}" KeyPath="yes" />
          </Component>
        </Directory>
      </Directory>
    </Directory>
  </Product>
</Wix>
`

	tmpl, err := template.New("wxs").Parse(wxsTemplate)
	if err != nil {
		return err
	}

	manufacturer := b.config.Manufacturer
	if manufacturer == "" {
		manufacturer = "Unknown"
	}

	upgradeCode := b.config.UpgradeCode
	if upgradeCode == "" {
		upgradeCode = "00000000-0000-0000-0000-000000000000"
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, map[string]string{
		"Name":         name,
		"Version":      version,
		"BinaryPath":   binary.Path,
		"Manufacturer": manufacturer,
		"UpgradeCode":  upgradeCode,
	})
}

// runWixl runs wixl to create MSI.
func (b *MSIBuilder) runWixl(ctx context.Context, wxsPath, msiPath string) error {
	cmd := exec.CommandContext(ctx, "wixl", "-o", msiPath, wxsPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runWix runs WiX Toolset to create MSI.
func (b *MSIBuilder) runWix(ctx context.Context, wxsPath, msiPath string) error {
	wixobjPath := wxsPath + ".wixobj"

	// Compile
	candle := exec.CommandContext(ctx, "candle", "-o", wixobjPath, wxsPath)
	candle.Stdout = os.Stdout
	candle.Stderr = os.Stderr
	if err := candle.Run(); err != nil {
		return fmt.Errorf("candle failed: %w", err)
	}

	// Link
	light := exec.CommandContext(ctx, "light", "-o", msiPath, wixobjPath)
	light.Stdout = os.Stdout
	light.Stderr = os.Stderr
	if err := light.Run(); err != nil {
		return fmt.Errorf("light failed: %w", err)
	}

	return nil
}

// NSISBuilder creates Windows NSIS installers.
type NSISBuilder struct {
	config  config.NSIS
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewNSISBuilder creates a new NSIS builder.
func NewNSISBuilder(cfg config.NSIS, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *NSISBuilder {
	return &NSISBuilder{
		config:  cfg,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// Build creates an NSIS installer.
func (b *NSISBuilder) Build(ctx context.Context) error {
	// Check if makensis is available
	if _, err := exec.LookPath("makensis"); err != nil {
		log.Warn("Skipping NSIS creation: makensis not found (install NSIS)")
		return nil
	}

	log.Info("Building NSIS installer")

	// Get windows binaries
	binaries := b.manager.Filter(func(a artifact.Artifact) bool {
		return a.Type == artifact.TypeBinary && a.Goos == "windows"
	})

	if len(binaries) == 0 {
		log.Warn("No Windows binaries found for NSIS")
		return nil
	}

	for _, binary := range binaries {
		if err := b.createNSIS(ctx, binary); err != nil {
			return fmt.Errorf("failed to create NSIS installer for %s: %w", binary.Name, err)
		}
	}

	return nil
}

// createNSIS creates an NSIS installer.
func (b *NSISBuilder) createNSIS(ctx context.Context, binary artifact.Artifact) error {
	name := b.config.Name
	if name == "" {
		name = b.tmplCtx.Get("ProjectName")
	}

	version := b.tmplCtx.Get("Version")
	exeFileName := fmt.Sprintf("%s_%s_%s_setup.exe", name, version, binary.Goarch)
	exePath := filepath.Join(b.distDir, exeFileName)

	// Check if custom script is provided
	nsiPath := b.config.Script
	if nsiPath == "" {
		// Generate NSIS script
		nsiPath = filepath.Join(b.distDir, fmt.Sprintf("%s.nsi", name))
		if err := b.generateNsi(nsiPath, binary, name, version, exePath); err != nil {
			return fmt.Errorf("failed to generate NSIS script: %w", err)
		}
	}

	// Run makensis
	args := []string{}

	// Add defines
	for key, value := range b.config.Defines {
		args = append(args, fmt.Sprintf("-D%s=%s", key, value))
	}

	args = append(args, nsiPath)

	cmd := exec.CommandContext(ctx, "makensis", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("makensis failed: %w", err)
	}

	// Add artifact
	b.manager.Add(artifact.Artifact{
		Name:   exeFileName,
		Path:   exePath,
		Type:   artifact.TypeNSIS,
		Goos:   "windows",
		Goarch: binary.Goarch,
	})

	log.Info("NSIS installer created", "name", exeFileName)
	return nil
}

// generateNsi generates an NSIS script.
func (b *NSISBuilder) generateNsi(path string, binary artifact.Artifact, name, version, outputPath string) error {
	nsiTemplate := `!include "MUI2.nsh"

Name "{{ .Name }}"
OutFile "{{ .OutputPath }}"
InstallDir "$PROGRAMFILES\{{ .Name }}"
RequestExecutionLevel admin

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

Section "Install"
  SetOutPath $INSTDIR
  File "{{ .BinaryPath }}"
  WriteUninstaller "$INSTDIR\Uninstall.exe"

  CreateDirectory "$SMPROGRAMS\{{ .Name }}"
  CreateShortcut "$SMPROGRAMS\{{ .Name }}\{{ .Name }}.lnk" "$INSTDIR\{{ .BinaryName }}"
  CreateShortcut "$SMPROGRAMS\{{ .Name }}\Uninstall.lnk" "$INSTDIR\Uninstall.exe"
SectionEnd

Section "Uninstall"
  Delete "$INSTDIR\{{ .BinaryName }}"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\{{ .Name }}\{{ .Name }}.lnk"
  Delete "$SMPROGRAMS\{{ .Name }}\Uninstall.lnk"
  RMDir "$SMPROGRAMS\{{ .Name }}"
SectionEnd
`

	tmpl, err := template.New("nsi").Parse(nsiTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, map[string]string{
		"Name":       name,
		"Version":    version,
		"BinaryPath": binary.Path,
		"BinaryName": filepath.Base(binary.Path),
		"OutputPath": outputPath,
	})
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
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

		return copyFile(path, dstPath)
	})
}

// BuildAllAppBundles builds app bundles for all configurations.
func BuildAllAppBundles(ctx context.Context, configs []config.AppBundle, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) error {
	for i, cfg := range configs {
		log.Info("Building App Bundle", "index", i+1, "total", len(configs))
		builder := NewAppBundleBuilder(cfg, tmplCtx, manager, distDir)
		if err := builder.Build(ctx); err != nil {
			return err
		}
	}
	return nil
}

// BuildAllDMGs builds DMGs for all configurations.
func BuildAllDMGs(ctx context.Context, configs []config.DMG, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) error {
	for i, cfg := range configs {
		log.Info("Building DMG", "index", i+1, "total", len(configs))
		builder := NewDMGBuilder(cfg, tmplCtx, manager, distDir)
		if err := builder.Build(ctx); err != nil {
			return err
		}
	}
	return nil
}

// BuildAllMSIs builds MSIs for all configurations.
func BuildAllMSIs(ctx context.Context, configs []config.MSI, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) error {
	for i, cfg := range configs {
		log.Info("Building MSI", "index", i+1, "total", len(configs))
		builder := NewMSIBuilder(cfg, tmplCtx, manager, distDir)
		if err := builder.Build(ctx); err != nil {
			return err
		}
	}
	return nil
}

// BuildAllNSIS builds NSIS installers for all configurations.
func BuildAllNSIS(ctx context.Context, configs []config.NSIS, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) error {
	for i, cfg := range configs {
		log.Info("Building NSIS", "index", i+1, "total", len(configs))
		builder := NewNSISBuilder(cfg, tmplCtx, manager, distDir)
		if err := builder.Build(ctx); err != nil {
			return err
		}
	}
	return nil
}
