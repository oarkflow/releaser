// Package publish provides publishing functionality for Releaser.
package publish

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/oarkflow/releaser/internal/artifact"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// SnapcraftPublisher publishes to Snapcraft/Snap Store
type SnapcraftPublisher struct {
	config  config.Snapcraft
	tmplCtx *tmpl.Context
}

// NewSnapcraftPublisher creates a new Snapcraft publisher
func NewSnapcraftPublisher(cfg config.Snapcraft, tmplCtx *tmpl.Context) *SnapcraftPublisher {
	return &SnapcraftPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish publishes snap packages to the Snap Store
func (p *SnapcraftPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	if !p.config.Publish {
		log.Info("Snapcraft publish disabled, skipping")
		return nil
	}

	// Check for snapcraft CLI
	if _, err := exec.LookPath("snapcraft"); err != nil {
		return fmt.Errorf("snapcraft CLI not found: %w", err)
	}

	log.Info("Publishing to Snap Store")

	// Find snap artifacts
	for _, a := range artifacts {
		if !strings.HasSuffix(a.Name, ".snap") {
			continue
		}

		if err := p.publishSnap(ctx, a); err != nil {
			return fmt.Errorf("failed to publish %s: %w", a.Name, err)
		}
	}

	log.Info("Published to Snap Store")
	return nil
}

// publishSnap publishes a single snap package
func (p *SnapcraftPublisher) publishSnap(ctx context.Context, a artifact.Artifact) error {
	log.Debug("Publishing snap", "name", a.Name)

	// Determine channels
	channels := p.config.ChannelTemplates
	if len(channels) == 0 {
		channels = []string{"stable"}
	}

	// Apply templates to channels
	var resolvedChannels []string
	for _, ch := range channels {
		resolved, _ := p.tmplCtx.Apply(ch)
		resolvedChannels = append(resolvedChannels, resolved)
	}

	// Build snapcraft push command
	args := []string{"upload", "--release=" + strings.Join(resolvedChannels, ",")}
	args = append(args, a.Path)

	cmd := exec.CommandContext(ctx, "snapcraft", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("snapcraft upload failed: %w", err)
	}

	log.Info("Snap published", "name", a.Name, "channels", resolvedChannels)
	return nil
}

// AURPublisher publishes to Arch User Repository
type AURPublisher struct {
	config  config.AUR
	tmplCtx *tmpl.Context
	manager *artifact.Manager
}

// NewAURPublisher creates a new AUR publisher
func NewAURPublisher(cfg config.AUR, tmplCtx *tmpl.Context, manager *artifact.Manager) *AURPublisher {
	return &AURPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
		manager: manager,
	}
}

// Publish publishes to AUR
func (p *AURPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	if p.config.SkipUpload == "true" {
		log.Info("AUR upload disabled, skipping")
		return nil
	}

	log.Info("Publishing to AUR", "package", p.config.Name)

	// Generate PKGBUILD
	pkgbuild, err := p.generatePKGBUILD(artifacts)
	if err != nil {
		return fmt.Errorf("failed to generate PKGBUILD: %w", err)
	}

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "aur-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Clone AUR repository
	gitURL := p.config.GitURL
	if gitURL == "" {
		gitURL = fmt.Sprintf("ssh://aur@aur.archlinux.org/%s.git", p.config.Name)
	}

	if err := p.cloneAUR(ctx, tmpDir, gitURL); err != nil {
		return fmt.Errorf("failed to clone AUR repo: %w", err)
	}

	// Write PKGBUILD
	pkgbuildPath := filepath.Join(tmpDir, "PKGBUILD")
	if err := os.WriteFile(pkgbuildPath, []byte(pkgbuild), 0644); err != nil {
		return err
	}

	// Generate .SRCINFO
	if err := p.generateSRCINFO(ctx, tmpDir); err != nil {
		log.Warn("Failed to generate .SRCINFO", "error", err)
	}

	// Commit and push
	if err := p.commitAndPush(ctx, tmpDir); err != nil {
		return fmt.Errorf("failed to push to AUR: %w", err)
	}

	log.Info("Published to AUR", "package", p.config.Name)
	return nil
}

// generatePKGBUILD generates an Arch Linux PKGBUILD file
func (p *AURPublisher) generatePKGBUILD(artifacts []artifact.Artifact) (string, error) {
	name := p.config.Name
	if name == "" {
		name = p.tmplCtx.Get("ProjectName")
	}

	version := strings.TrimPrefix(p.tmplCtx.Get("Version"), "v")
	desc := p.config.Description
	if desc == "" {
		desc = name
	}

	// Find Linux amd64 archive
	var sourceURL string
	for _, a := range artifacts {
		if a.Goos == "linux" && a.Goarch == "amd64" && a.Type == artifact.TypeArchive {
			urlTemplate := p.config.URLTemplate
			if urlTemplate == "" {
				urlTemplate = "https://github.com/{{ .Env.GITHUB_OWNER }}/{{ .Env.GITHUB_REPO }}/releases/download/{{ .Tag }}/{{ .ArtifactName }}"
			}
			tmplCtx := p.tmplCtx.WithArtifact(a.Name, a.Goos, a.Goarch, a.Goarm, a.Goamd64)
			sourceURL, _ = tmplCtx.Apply(urlTemplate)
			break
		}
	}

	if sourceURL == "" {
		return "", fmt.Errorf("no Linux amd64 archive found for AUR")
	}

	var buf strings.Builder
	buf.WriteString("# Maintainer: ")
	for i, m := range p.config.Maintainers {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(m)
	}
	buf.WriteString("\n")

	buf.WriteString(fmt.Sprintf("pkgname=%s\n", name))
	buf.WriteString(fmt.Sprintf("pkgver=%s\n", version))
	buf.WriteString("pkgrel=1\n")
	buf.WriteString(fmt.Sprintf("pkgdesc=\"%s\"\n", desc))
	buf.WriteString("arch=('x86_64')\n")
	buf.WriteString(fmt.Sprintf("url=\"%s\"\n", p.config.Homepage))
	buf.WriteString(fmt.Sprintf("license=('%s')\n", p.config.License))

	if len(p.config.Depends) > 0 {
		buf.WriteString(fmt.Sprintf("depends=('%s')\n", strings.Join(p.config.Depends, "' '")))
	}
	if len(p.config.OptDepends) > 0 {
		buf.WriteString(fmt.Sprintf("optdepends=('%s')\n", strings.Join(p.config.OptDepends, "' '")))
	}
	if len(p.config.Conflicts) > 0 {
		buf.WriteString(fmt.Sprintf("conflicts=('%s')\n", strings.Join(p.config.Conflicts, "' '")))
	}
	if len(p.config.Provides) > 0 {
		buf.WriteString(fmt.Sprintf("provides=('%s')\n", strings.Join(p.config.Provides, "' '")))
	}

	buf.WriteString(fmt.Sprintf("source=(\"%s\")\n", sourceURL))
	buf.WriteString("sha256sums=('SKIP')\n")
	buf.WriteString("\n")

	// Package function
	packageFunc := p.config.Package
	if packageFunc == "" {
		packageFunc = fmt.Sprintf(`install -Dm755 %s "$pkgdir/usr/bin/%s"`, name, name)
	}
	buf.WriteString(fmt.Sprintf("package() {\n    %s\n}\n", packageFunc))

	return buf.String(), nil
}

// cloneAUR clones the AUR repository
func (p *AURPublisher) cloneAUR(ctx context.Context, dir, gitURL string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", gitURL, dir)

	if p.config.GitSSHCommand != "" {
		cmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+p.config.GitSSHCommand)
	}

	if p.config.PrivateKey != "" {
		cmd.Env = append(os.Environ(),
			"GIT_SSH_COMMAND=ssh -i "+p.config.PrivateKey+" -o StrictHostKeyChecking=no")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// If clone fails (new package), initialize
	if err := cmd.Run(); err != nil {
		// Initialize new repo
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		initCmd := exec.CommandContext(ctx, "git", "init")
		initCmd.Dir = dir
		if err := initCmd.Run(); err != nil {
			return err
		}
		// Add remote
		remoteCmd := exec.CommandContext(ctx, "git", "remote", "add", "origin", gitURL)
		remoteCmd.Dir = dir
		return remoteCmd.Run()
	}
	return nil
}

// generateSRCINFO generates .SRCINFO from PKGBUILD
func (p *AURPublisher) generateSRCINFO(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "makepkg", "--printsrcinfo")
	cmd.Dir = dir

	output, err := cmd.Output()
	if err != nil {
		return err
	}

	srcInfoPath := filepath.Join(dir, ".SRCINFO")
	return os.WriteFile(srcInfoPath, output, 0644)
}

// commitAndPush commits and pushes to AUR
func (p *AURPublisher) commitAndPush(ctx context.Context, dir string) error {
	// Configure git
	email := "releaser@example.com"
	name := "Releaser"
	if p.config.CommitAuthor.Email != "" {
		email = p.config.CommitAuthor.Email
	}
	if p.config.CommitAuthor.Name != "" {
		name = p.config.CommitAuthor.Name
	}

	// Set git config
	exec.CommandContext(ctx, "git", "config", "user.email", email).Run()
	exec.CommandContext(ctx, "git", "config", "user.name", name).Run()

	// Add files
	addCmd := exec.CommandContext(ctx, "git", "add", "PKGBUILD", ".SRCINFO")
	addCmd.Dir = dir
	if err := addCmd.Run(); err != nil {
		return err
	}

	// Commit
	commitMsg := p.config.CommitMsgTemplate
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("Update to %s", p.tmplCtx.Get("Version"))
	}
	commitMsg, _ = p.tmplCtx.Apply(commitMsg)

	commitCmd := exec.CommandContext(ctx, "git", "commit", "-m", commitMsg)
	commitCmd.Dir = dir
	if err := commitCmd.Run(); err != nil {
		return err
	}

	// Push
	pushCmd := exec.CommandContext(ctx, "git", "push", "origin", "master")
	pushCmd.Dir = dir
	pushCmd.Env = os.Environ()
	if p.config.GitSSHCommand != "" {
		pushCmd.Env = append(pushCmd.Env, "GIT_SSH_COMMAND="+p.config.GitSSHCommand)
	}
	if p.config.PrivateKey != "" {
		pushCmd.Env = append(pushCmd.Env,
			"GIT_SSH_COMMAND=ssh -i "+p.config.PrivateKey+" -o StrictHostKeyChecking=no")
	}

	return pushCmd.Run()
}

// ChocolateyPublisher publishes to Chocolatey
type ChocolateyPublisher struct {
	config  config.Chocolatey
	tmplCtx *tmpl.Context
	manager *artifact.Manager
}

// NewChocolateyPublisher creates a new Chocolatey publisher
func NewChocolateyPublisher(cfg config.Chocolatey, tmplCtx *tmpl.Context, manager *artifact.Manager) *ChocolateyPublisher {
	return &ChocolateyPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
		manager: manager,
	}
}

// Publish publishes to Chocolatey
func (p *ChocolateyPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	if p.config.SkipPublish == "true" {
		log.Info("Chocolatey publish disabled, skipping")
		return nil
	}

	apiKey := p.config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("CHOCOLATEY_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("CHOCOLATEY_API_KEY is required")
	}

	log.Info("Publishing to Chocolatey", "package", p.config.Name)

	// Create temporary directory for package
	tmpDir, err := os.MkdirTemp("", "choco-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Generate nuspec
	if err := p.generateNuspec(tmpDir, artifacts); err != nil {
		return fmt.Errorf("failed to generate nuspec: %w", err)
	}

	// Generate install script
	if err := p.generateInstallScript(tmpDir, artifacts); err != nil {
		return fmt.Errorf("failed to generate install script: %w", err)
	}

	// Pack
	nupkgPath, err := p.packNuget(ctx, tmpDir)
	if err != nil {
		return fmt.Errorf("failed to pack: %w", err)
	}

	// Push
	if err := p.pushNuget(ctx, nupkgPath, apiKey); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	log.Info("Published to Chocolatey", "package", p.config.Name)
	return nil
}

// generateNuspec generates a Chocolatey nuspec file
func (p *ChocolateyPublisher) generateNuspec(dir string, artifacts []artifact.Artifact) error {
	name := p.config.Name
	if name == "" {
		name = p.tmplCtx.Get("ProjectName")
	}

	version := strings.TrimPrefix(p.tmplCtx.Get("Version"), "v")

	nuspec := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://schemas.microsoft.com/packaging/2015/06/nuspec.xsd">
  <metadata>
    <id>%s</id>
    <version>%s</version>
    <title>%s</title>
    <authors>%s</authors>
    <projectUrl>%s</projectUrl>
    <iconUrl>%s</iconUrl>
    <copyright>%s</copyright>
    <licenseUrl>%s</licenseUrl>
    <requireLicenseAcceptance>%t</requireLicenseAcceptance>
    <projectSourceUrl>%s</projectSourceUrl>
    <docsUrl>%s</docsUrl>
    <tags>%s</tags>
    <summary>%s</summary>
    <description>%s</description>
    <bugTrackerUrl>%s</bugTrackerUrl>
  </metadata>
  <files>
    <file src="tools\**" target="tools" />
  </files>
</package>`,
		name,
		version,
		p.config.Title,
		p.config.Authors,
		p.config.ProjectURL,
		p.config.IconURL,
		p.config.Copyright,
		p.config.LicenseURL,
		p.config.RequireLicenseAcceptance,
		p.config.ProjectSourceURL,
		p.config.DocsURL,
		p.config.Tags,
		p.config.Summary,
		p.config.Description,
		p.config.BugTrackerURL,
	)

	return os.WriteFile(filepath.Join(dir, name+".nuspec"), []byte(nuspec), 0644)
}

// generateInstallScript generates chocolateyinstall.ps1
func (p *ChocolateyPublisher) generateInstallScript(dir string, artifacts []artifact.Artifact) error {
	// Create tools directory
	toolsDir := filepath.Join(dir, "tools")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		return err
	}

	// Find Windows archive
	var downloadURL string
	for _, a := range artifacts {
		if a.Goos == "windows" && a.Goarch == "amd64" && a.Type == artifact.TypeArchive {
			urlTemplate := p.config.URLTemplate
			if urlTemplate == "" {
				urlTemplate = "https://github.com/{{ .Env.GITHUB_OWNER }}/{{ .Env.GITHUB_REPO }}/releases/download/{{ .Tag }}/{{ .ArtifactName }}"
			}
			tmplCtx := p.tmplCtx.WithArtifact(a.Name, a.Goos, a.Goarch, a.Goarm, a.Goamd64)
			downloadURL, _ = tmplCtx.Apply(urlTemplate)
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no Windows amd64 archive found for Chocolatey")
	}

	installScript := fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$toolsDir = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
$url = '%s'

$packageArgs = @{
    packageName   = $env:ChocolateyPackageName
    unzipLocation = $toolsDir
    url           = $url
}

Install-ChocolateyZipPackage @packageArgs
`, downloadURL)

	return os.WriteFile(filepath.Join(toolsDir, "chocolateyinstall.ps1"), []byte(installScript), 0644)
}

// packNuget creates a nupkg file
func (p *ChocolateyPublisher) packNuget(ctx context.Context, dir string) (string, error) {
	name := p.config.Name
	if name == "" {
		name = p.tmplCtx.Get("ProjectName")
	}

	// Use choco pack or nuget pack
	cmd := exec.CommandContext(ctx, "choco", "pack", name+".nuspec")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Try nuget as fallback
		cmd = exec.CommandContext(ctx, "nuget", "pack", name+".nuspec")
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", err
		}
	}

	// Find the created nupkg
	version := strings.TrimPrefix(p.tmplCtx.Get("Version"), "v")
	nupkgPath := filepath.Join(dir, fmt.Sprintf("%s.%s.nupkg", name, version))
	return nupkgPath, nil
}

// pushNuget pushes the package to Chocolatey
func (p *ChocolateyPublisher) pushNuget(ctx context.Context, nupkgPath, apiKey string) error {
	source := p.config.SourceRepo
	if source == "" {
		source = "https://push.chocolatey.org/"
	}

	cmd := exec.CommandContext(ctx, "choco", "push", nupkgPath,
		"--source", source,
		"--api-key", apiKey)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// WingetPublisher publishes to Windows Package Manager
type WingetPublisher struct {
	config  config.Winget
	tmplCtx *tmpl.Context
	manager *artifact.Manager
}

// NewWingetPublisher creates a new Winget publisher
func NewWingetPublisher(cfg config.Winget, tmplCtx *tmpl.Context, manager *artifact.Manager) *WingetPublisher {
	return &WingetPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
		manager: manager,
	}
}

// Publish publishes to winget-pkgs repository
func (p *WingetPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	if p.config.SkipUpload == "true" {
		log.Info("Winget upload disabled, skipping")
		return nil
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN is required for Winget publish")
	}

	log.Info("Publishing to Winget", "package", p.config.PackageIdentifier)

	// Generate manifest
	manifest, err := p.generateManifest(artifacts)
	if err != nil {
		return fmt.Errorf("failed to generate manifest: %w", err)
	}

	// Push to repository
	repo := p.config.Repository
	if repo.Owner == "" {
		repo.Owner = "microsoft"
		repo.Name = "winget-pkgs"
	}

	version := strings.TrimPrefix(p.tmplCtx.Get("Version"), "v")
	manifestPath := fmt.Sprintf("manifests/%s/%s/%s/%s.yaml",
		strings.ToLower(string(p.config.PackageIdentifier[0])),
		strings.ReplaceAll(p.config.PackageIdentifier, ".", "/"),
		version,
		p.config.PackageIdentifier,
	)

	if err := p.pushManifest(ctx, token, repo, manifestPath, manifest); err != nil {
		return fmt.Errorf("failed to push manifest: %w", err)
	}

	log.Info("Published to Winget", "package", p.config.PackageIdentifier)
	return nil
}

// generateManifest generates a Winget manifest
func (p *WingetPublisher) generateManifest(artifacts []artifact.Artifact) (string, error) {
	version := strings.TrimPrefix(p.tmplCtx.Get("Version"), "v")

	// Find Windows installer
	var installerURL string
	for _, a := range artifacts {
		if a.Goos == "windows" && a.Goarch == "amd64" {
			urlTemplate := p.config.URLTemplate
			if urlTemplate == "" {
				urlTemplate = "https://github.com/{{ .Env.GITHUB_OWNER }}/{{ .Env.GITHUB_REPO }}/releases/download/{{ .Tag }}/{{ .ArtifactName }}"
			}
			tmplCtx := p.tmplCtx.WithArtifact(a.Name, a.Goos, a.Goarch, a.Goarm, a.Goamd64)
			installerURL, _ = tmplCtx.Apply(urlTemplate)
			break
		}
	}

	if installerURL == "" {
		return "", fmt.Errorf("no Windows artifact found for Winget")
	}

	manifest := fmt.Sprintf(`PackageIdentifier: %s
PackageVersion: %s
PackageLocale: en-US
Publisher: %s
PublisherUrl: %s
PublisherSupportUrl: %s
Author: %s
PackageName: %s
PackageUrl: %s
License: %s
LicenseUrl: %s
Copyright: %s
CopyrightUrl: %s
ShortDescription: %s
Description: %s
Tags:
%s
Installers:
  - Architecture: x64
    InstallerUrl: %s
    InstallerSha256:
    InstallerType: zip
ManifestType: singleton
ManifestVersion: 1.0.0
`,
		p.config.PackageIdentifier,
		version,
		p.config.Publisher,
		p.config.PublisherURL,
		p.config.PublisherSupportURL,
		p.config.Publisher,
		p.config.Name,
		p.config.Homepage,
		p.config.License,
		p.config.LicenseURL,
		p.config.Copyright,
		p.config.CopyrightURL,
		p.config.ShortDescription,
		p.config.Description,
		formatTags(p.config.Tags),
		installerURL,
	)

	return manifest, nil
}

// formatTags formats tags for YAML
func formatTags(tags []string) string {
	var buf strings.Builder
	for _, tag := range tags {
		buf.WriteString(fmt.Sprintf("  - %s\n", tag))
	}
	return buf.String()
}

// pushManifest pushes the manifest to the repository
func (p *WingetPublisher) pushManifest(ctx context.Context, token string, repo config.RepoRef, path, content string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", repo.Owner, repo.Name, path)

	body := map[string]interface{}{
		"message": fmt.Sprintf("Add %s version %s", p.config.PackageIdentifier, p.tmplCtx.Get("Version")),
		"content": encodeBase64(content),
		"branch":  "main",
	}

	bodyJSON, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(bodyJSON))
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error: %s", respBody)
	}

	return nil
}

// ScoopPublisher publishes to Scoop bucket
type ScoopPublisher struct {
	config  config.Scoop
	tmplCtx *tmpl.Context
}

// NewScoopPublisher creates a new Scoop publisher
func NewScoopPublisher(cfg config.Scoop, tmplCtx *tmpl.Context) *ScoopPublisher {
	return &ScoopPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish publishes to a Scoop bucket
func (p *ScoopPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	if p.config.SkipUpload == "true" {
		log.Info("Scoop upload disabled, skipping")
		return nil
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN is required for Scoop publish")
	}

	log.Info("Publishing to Scoop", "package", p.config.Name)

	// Generate manifest
	manifest, err := p.generateManifest(artifacts)
	if err != nil {
		return fmt.Errorf("failed to generate manifest: %w", err)
	}

	// Determine manifest path
	manifestName := p.config.Name
	if manifestName == "" {
		manifestName = p.tmplCtx.Get("ProjectName")
	}

	folder := p.config.Directory
	if folder == "" {
		folder = "bucket"
	}

	manifestPath := fmt.Sprintf("%s/%s.json", folder, manifestName)

	// Push to repository
	repo := p.config.Repository
	if repo.Owner == "" || repo.Name == "" {
		return fmt.Errorf("scoop repository owner and name are required")
	}

	if err := p.pushManifest(ctx, token, repo, manifestPath, manifest); err != nil {
		return fmt.Errorf("failed to push manifest: %w", err)
	}

	log.Info("Published to Scoop", "package", manifestName)
	return nil
}

// generateManifest generates a Scoop manifest
func (p *ScoopPublisher) generateManifest(artifacts []artifact.Artifact) (string, error) {
	version := strings.TrimPrefix(p.tmplCtx.Get("Version"), "v")
	name := p.config.Name
	if name == "" {
		name = p.tmplCtx.Get("ProjectName")
	}

	// Find Windows 64-bit and 32-bit archives
	var url64, url32 string
	for _, a := range artifacts {
		if a.Goos == "windows" && a.Type == artifact.TypeArchive {
			urlTemplate := p.config.URLTemplate
			if urlTemplate == "" {
				urlTemplate = "https://github.com/{{ .Env.GITHUB_OWNER }}/{{ .Env.GITHUB_REPO }}/releases/download/{{ .Tag }}/{{ .ArtifactName }}"
			}
			tmplCtx := p.tmplCtx.WithArtifact(a.Name, a.Goos, a.Goarch, a.Goarm, a.Goamd64)
			url, _ := tmplCtx.Apply(urlTemplate)

			if a.Goarch == "amd64" {
				url64 = url
			} else if a.Goarch == "386" {
				url32 = url
			}
		}
	}

	if url64 == "" {
		return "", fmt.Errorf("no Windows amd64 archive found for Scoop")
	}

	manifest := map[string]interface{}{
		"version":     version,
		"description": p.config.Description,
		"homepage":    p.config.Homepage,
		"license":     p.config.License,
		"architecture": map[string]interface{}{
			"64bit": map[string]interface{}{
				"url":  url64,
				"bin":  name + ".exe",
				"hash": "",
			},
		},
	}

	if url32 != "" {
		arch := manifest["architecture"].(map[string]interface{})
		arch["32bit"] = map[string]interface{}{
			"url":  url32,
			"bin":  name + ".exe",
			"hash": "",
		}
	}

	if len(p.config.Depends) > 0 {
		manifest["depends"] = p.config.Depends
	}

	if len(p.config.Persist) > 0 {
		manifest["persist"] = p.config.Persist
	}

	if len(p.config.Shortcuts) > 0 {
		manifest["shortcuts"] = p.config.Shortcuts
	}

	jsonData, err := json.MarshalIndent(manifest, "", "    ")
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// pushManifest pushes the Scoop manifest to the repository
func (p *ScoopPublisher) pushManifest(ctx context.Context, token string, repo config.RepoRef, path, content string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", repo.Owner, repo.Name, path)

	// Check if file exists
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "token "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	var sha string
	if resp.StatusCode == 200 {
		var existing map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&existing)
		if s, ok := existing["sha"].(string); ok {
			sha = s
		}
	}
	resp.Body.Close()

	// Create/Update file
	commitMsg := p.config.CommitMsgTemplate
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("Update %s to %s", p.config.Name, p.tmplCtx.Get("Version"))
	}
	commitMsg, _ = p.tmplCtx.Apply(commitMsg)

	body := map[string]interface{}{
		"message": commitMsg,
		"content": encodeBase64(content),
	}
	if sha != "" {
		body["sha"] = sha
	}

	// Add commit author if configured
	if p.config.CommitAuthor.Name != "" || p.config.CommitAuthor.Email != "" {
		committer := make(map[string]string)
		if p.config.CommitAuthor.Name != "" {
			committer["name"] = p.config.CommitAuthor.Name
		}
		if p.config.CommitAuthor.Email != "" {
			committer["email"] = p.config.CommitAuthor.Email
		}
		body["committer"] = committer
	}

	bodyJSON, _ := json.Marshal(body)
	req, _ = http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(bodyJSON))
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error: %s", respBody)
	}

	return nil
}
