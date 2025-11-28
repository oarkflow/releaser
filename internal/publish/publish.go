/*
Package publish provides publishing functionality for Releaser.
*/
package publish

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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

// Publisher interface for different publishing targets
type Publisher interface {
	Publish(ctx context.Context, artifacts []artifact.Artifact) error
}

// GitHubPublisher publishes to GitHub Releases
type GitHubPublisher struct {
	config  config.Release
	tmplCtx *tmpl.Context
	token   string
}

// NewGitHubPublisher creates a new GitHub publisher
func NewGitHubPublisher(cfg config.Release, tmplCtx *tmpl.Context) *GitHubPublisher {
	return &GitHubPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
		token:   os.Getenv("GITHUB_TOKEN"),
	}
}

// Publish publishes artifacts to GitHub Releases
func (p *GitHubPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	if p.token == "" {
		return fmt.Errorf("GITHUB_TOKEN is required")
	}

	owner := p.config.GitHub.Owner
	repo := p.config.GitHub.Name

	if owner == "" || repo == "" {
		return fmt.Errorf("GitHub owner and repo are required")
	}

	// Apply templates
	owner, _ = p.tmplCtx.Apply(owner)
	repo, _ = p.tmplCtx.Apply(repo)

	log.Info("Publishing to GitHub Releases", "owner", owner, "repo", repo)

	// Get or create release
	tag := p.tmplCtx.Get("Tag")
	releaseID, err := p.getOrCreateRelease(ctx, owner, repo, tag)
	if err != nil {
		return err
	}

	// Upload assets
	for _, a := range artifacts {
		if err := p.uploadAsset(ctx, owner, repo, releaseID, a); err != nil {
			return fmt.Errorf("failed to upload %s: %w", a.Name, err)
		}
	}

	log.Info("Published to GitHub Releases")
	return nil
}

// getOrCreateRelease gets or creates a GitHub release
func (p *GitHubPublisher) getOrCreateRelease(ctx context.Context, owner, repo, tag string) (int64, error) {
	// Try to get existing release
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", owner, repo, tag)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "token "+p.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var release struct {
			ID int64 `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			return 0, err
		}
		return release.ID, nil
	}

	// Create new release
	name := p.config.NameTemplate
	if name == "" {
		name = tag
	}
	name, _ = p.tmplCtx.Apply(name)

	body := map[string]interface{}{
		"tag_name":               tag,
		"name":                   name,
		"draft":                  p.config.Draft,
		"prerelease":             p.config.Prerelease == "true",
		"generate_release_notes": false,
	}

	if p.config.TargetCommitish != "" {
		body["target_commitish"] = p.config.TargetCommitish
	}

	bodyJSON, _ := json.Marshal(body)
	url = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", owner, repo)
	req, _ = http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	req.Header.Set("Authorization", "token "+p.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to create release: %s", body)
	}

	var release struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return 0, err
	}

	return release.ID, nil
}

// uploadAsset uploads an asset to a GitHub release
func (p *GitHubPublisher) uploadAsset(ctx context.Context, owner, repo string, releaseID int64, a artifact.Artifact) error {
	log.Debug("Uploading asset", "name", a.Name)

	file, err := os.Open(a.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://uploads.github.com/repos/%s/%s/releases/%d/assets?name=%s",
		owner, repo, releaseID, a.Name)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, file)
	req.Header.Set("Authorization", "token "+p.token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = stat.Size()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upload asset: %s", body)
	}

	return nil
}

// NPMPublisher publishes to NPM registries
type NPMPublisher struct {
	config  config.NPM
	tmplCtx *tmpl.Context
}

// NewNPMPublisher creates a new NPM publisher
func NewNPMPublisher(cfg config.NPM, tmplCtx *tmpl.Context) *NPMPublisher {
	return &NPMPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish publishes to NPM
func (p *NPMPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	registry := p.config.Registry
	if registry == "" {
		registry = "https://registry.npmjs.org"
	}

	token := p.config.Token
	if token == "" {
		token = os.Getenv("NPM_TOKEN")
	}

	if token == "" {
		return fmt.Errorf("NPM_TOKEN is required")
	}

	log.Info("Publishing to NPM", "registry", registry)

	// Create a temporary directory for the package
	tmpDir, err := os.MkdirTemp("", "npm-publish-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create package.json
	pkg := map[string]interface{}{
		"name":        p.config.Name,
		"version":     strings.TrimPrefix(p.tmplCtx.Get("Version"), "v"),
		"description": p.config.Description,
		"homepage":    p.config.Homepage,
		"license":     p.config.License,
	}

	if p.config.Scope != "" {
		pkg["name"] = "@" + p.config.Scope + "/" + p.config.Name
	}

	if len(p.config.Keywords) > 0 {
		pkg["keywords"] = p.config.Keywords
	}

	if len(p.config.Dependencies) > 0 {
		pkg["dependencies"] = p.config.Dependencies
	}

	if len(p.config.Bin) > 0 {
		pkg["bin"] = p.config.Bin
	}

	if len(p.config.Files) > 0 {
		pkg["files"] = p.config.Files
	}

	for k, v := range p.config.ExtraFields {
		pkg[k] = v
	}

	// Write package.json
	pkgJSON, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal package.json: %w", err)
	}

	pkgPath := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(pkgPath, pkgJSON, 0644); err != nil {
		return fmt.Errorf("failed to write package.json: %w", err)
	}

	// Copy artifacts to temp directory
	for _, a := range artifacts {
		if a.Type == artifact.TypeBinary || a.Type == artifact.TypeArchive {
			destPath := filepath.Join(tmpDir, filepath.Base(a.Path))
			data, err := os.ReadFile(a.Path)
			if err != nil {
				log.Warn("Failed to read artifact", "path", a.Path, "error", err)
				continue
			}
			if err := os.WriteFile(destPath, data, 0755); err != nil {
				log.Warn("Failed to copy artifact", "path", a.Path, "error", err)
				continue
			}
		}
	}

	// Create .npmrc for authentication
	npmrcContent := fmt.Sprintf("//%s/:_authToken=%s\n", strings.TrimPrefix(strings.TrimPrefix(registry, "https://"), "http://"), token)
	npmrcPath := filepath.Join(tmpDir, ".npmrc")
	if err := os.WriteFile(npmrcPath, []byte(npmrcContent), 0600); err != nil {
		return fmt.Errorf("failed to write .npmrc: %w", err)
	}

	// Run npm publish
	args := []string{"publish", "--registry", registry}
	if p.config.Access != "" {
		args = append(args, "--access", p.config.Access)
	}
	if p.config.Tag != "" {
		args = append(args, "--tag", p.config.Tag)
	}

	cmd := exec.CommandContext(ctx, "npm", args...)
	cmd.Dir = tmpDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm publish failed: %w", err)
	}

	log.Info("NPM package published successfully", "name", pkg["name"], "version", pkg["version"])
	return nil
}

// HomebrewPublisher publishes to Homebrew taps
type HomebrewPublisher struct {
	config  config.Brew
	tmplCtx *tmpl.Context
}

// NewHomebrewPublisher creates a new Homebrew publisher
func NewHomebrewPublisher(cfg config.Brew, tmplCtx *tmpl.Context) *HomebrewPublisher {
	return &HomebrewPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish publishes to Homebrew
func (p *HomebrewPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	log.Info("Publishing to Homebrew")

	// Generate formula
	formula, err := p.generateFormula(artifacts)
	if err != nil {
		return err
	}

	// Determine tap repository
	tap := p.config.Tap
	if tap.Owner == "" && p.config.Repository.Owner != "" {
		tap = p.config.Repository
	}

	if tap.Owner == "" {
		return fmt.Errorf("Homebrew tap repository is required")
	}

	log.Debug("Generated Homebrew formula", "formula", formula)

	// Push to tap repository using GitHub API
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN is required for Homebrew tap push")
	}

	// Determine formula file path
	name := p.config.Name
	if name == "" {
		name = p.tmplCtx.Get("ProjectName")
	}
	formulaPath := fmt.Sprintf("Formula/%s.rb", name)
	if p.config.Directory != "" {
		formulaPath = fmt.Sprintf("%s/%s.rb", p.config.Directory, name)
	}

	// Get current file SHA (for updates)
	sha, err := p.getFileSHA(ctx, token, tap.Owner, tap.Name, formulaPath)
	if err != nil {
		log.Debug("No existing formula found, will create new one", "error", err)
	}

	// Commit the formula
	commitMsg := fmt.Sprintf("Update %s to %s", name, p.tmplCtx.Get("Version"))
	if p.config.CommitMsgTemplate != "" {
		commitMsg, _ = p.tmplCtx.Apply(p.config.CommitMsgTemplate)
	}

	if err := p.pushFormula(ctx, token, tap.Owner, tap.Name, formulaPath, formula, commitMsg, sha); err != nil {
		return fmt.Errorf("failed to push formula: %w", err)
	}

	log.Info("Homebrew formula published", "repo", fmt.Sprintf("%s/%s", tap.Owner, tap.Name), "path", formulaPath)
	return nil
}

// getFileSHA gets the SHA of an existing file in a GitHub repository
func (p *HomebrewPublisher) getFileSHA(ctx context.Context, token, owner, repo, path string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, path)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("file not found")
	}

	var result struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.SHA, nil
}

// pushFormula pushes the formula to the tap repository
func (p *HomebrewPublisher) pushFormula(ctx context.Context, token, owner, repo, path, content, message, sha string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, path)

	// Base64 encode the content
	encodedContent := encodeBase64(content)

	body := map[string]interface{}{
		"message": message,
		"content": encodedContent,
	}

	if sha != "" {
		body["sha"] = sha
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

// encodeBase64 encodes a string to base64
func encodeBase64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// generateFormula generates a Homebrew formula
func (p *HomebrewPublisher) generateFormula(artifacts []artifact.Artifact) (string, error) {
	name := p.config.Name
	if name == "" {
		name = p.tmplCtx.Get("ProjectName")
	}

	// Capitalize first letter for Ruby class name
	className := strings.Title(strings.ReplaceAll(name, "-", ""))

	var formula strings.Builder
	formula.WriteString(fmt.Sprintf("class %s < Formula\n", className))
	formula.WriteString(fmt.Sprintf("  desc \"%s\"\n", p.config.Description))
	formula.WriteString(fmt.Sprintf("  homepage \"%s\"\n", p.config.Homepage))
	formula.WriteString(fmt.Sprintf("  version \"%s\"\n", p.tmplCtx.Get("Version")))
	formula.WriteString(fmt.Sprintf("  license \"%s\"\n", p.config.License))
	formula.WriteString("\n")

	// Add URLs for each platform
	for _, a := range artifacts {
		if a.Type != artifact.TypeArchive {
			continue
		}

		urlTemplate := p.config.URLTemplate
		if urlTemplate == "" {
			urlTemplate = "https://github.com/{{ .Env.GITHUB_OWNER }}/{{ .Env.GITHUB_REPO }}/releases/download/{{ .Tag }}/{{ .ArtifactName }}"
		}

		tmplCtx := p.tmplCtx.WithArtifact(a.Name, a.Goos, a.Goarch, a.Goarm, a.Goamd64)
		url, _ := tmplCtx.Apply(urlTemplate)

		if a.Goos == "darwin" && a.Goarch == "amd64" {
			formula.WriteString(fmt.Sprintf("  url \"%s\"\n", url))
		} else if a.Goos == "darwin" && a.Goarch == "arm64" {
			formula.WriteString(fmt.Sprintf("  on_arm do\n"))
			formula.WriteString(fmt.Sprintf("    url \"%s\"\n", url))
			formula.WriteString(fmt.Sprintf("  end\n"))
		}
	}

	// Add dependencies
	for _, dep := range p.config.Dependencies {
		if dep.Type == "" {
			formula.WriteString(fmt.Sprintf("  depends_on \"%s\"\n", dep.Name))
		} else {
			formula.WriteString(fmt.Sprintf("  depends_on \"%s\" => :%s\n", dep.Name, dep.Type))
		}
	}

	// Add install section
	if p.config.Install != "" {
		formula.WriteString("\n  def install\n")
		formula.WriteString(fmt.Sprintf("    %s\n", p.config.Install))
		formula.WriteString("  end\n")
	}

	// Add test section
	if p.config.Test != "" {
		formula.WriteString("\n  test do\n")
		formula.WriteString(fmt.Sprintf("    %s\n", p.config.Test))
		formula.WriteString("  end\n")
	}

	// Add caveats
	if p.config.Caveats != "" {
		formula.WriteString("\n  def caveats\n")
		formula.WriteString(fmt.Sprintf("    <<~EOS\n      %s\n    EOS\n", p.config.Caveats))
		formula.WriteString("  end\n")
	}

	formula.WriteString("end\n")

	return formula.String(), nil
}

// DockerPublisher publishes Docker images
type DockerPublisher struct {
	config  config.Docker
	tmplCtx *tmpl.Context
}

// NewDockerPublisher creates a new Docker publisher
func NewDockerPublisher(cfg config.Docker, tmplCtx *tmpl.Context) *DockerPublisher {
	return &DockerPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish pushes Docker images
func (p *DockerPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	log.Info("Pushing Docker images")

	// TODO: Implement Docker push

	return nil
}

// CloudSmithPublisher publishes to CloudSmith
type CloudSmithPublisher struct {
	config  config.CloudSmith
	tmplCtx *tmpl.Context
}

// NewCloudSmithPublisher creates a new CloudSmith publisher
func NewCloudSmithPublisher(cfg config.CloudSmith, tmplCtx *tmpl.Context) *CloudSmithPublisher {
	return &CloudSmithPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish publishes packages to CloudSmith
func (p *CloudSmithPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	log.Info("Publishing to CloudSmith", "owner", p.config.Owner, "repo", p.config.Repository)

	token := os.Getenv("CLOUDSMITH_API_KEY")
	if token == "" {
		return fmt.Errorf("CLOUDSMITH_API_KEY is required")
	}

	for _, a := range artifacts {
		if a.Type != artifact.TypeLinuxPackage {
			continue
		}

		if err := p.uploadPackage(ctx, token, a); err != nil {
			return fmt.Errorf("failed to upload %s: %w", a.Name, err)
		}
	}

	return nil
}

// uploadPackage uploads a package to CloudSmith
func (p *CloudSmithPublisher) uploadPackage(ctx context.Context, token string, a artifact.Artifact) error {
	log.Debug("Uploading package", "name", a.Name)

	file, err := os.Open(a.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("package_file", filepath.Base(a.Path))
	if err != nil {
		return err
	}

	if _, err := io.Copy(part, file); err != nil {
		return err
	}

	if p.config.Distribution != "" {
		writer.WriteField("distribution", p.config.Distribution)
	}

	writer.Close()

	url := fmt.Sprintf("https://upload.cloudsmith.io/%s/%s/",
		p.config.Owner, p.config.Repository)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, &body)
	req.Header.Set("X-Api-Key", token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed: %s", respBody)
	}

	return nil
}

// FuryPublisher publishes to Fury.io
type FuryPublisher struct {
	config  config.Fury
	tmplCtx *tmpl.Context
}

// NewFuryPublisher creates a new Fury publisher
func NewFuryPublisher(cfg config.Fury, tmplCtx *tmpl.Context) *FuryPublisher {
	return &FuryPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish publishes packages to Fury.io
func (p *FuryPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	log.Info("Publishing to Fury.io", "account", p.config.Account)

	token := os.Getenv("FURY_TOKEN")
	if token == "" {
		return fmt.Errorf("FURY_TOKEN is required")
	}

	for _, a := range artifacts {
		if a.Type != artifact.TypeLinuxPackage {
			continue
		}

		if err := p.uploadPackage(ctx, token, a); err != nil {
			return fmt.Errorf("failed to upload %s: %w", a.Name, err)
		}
	}

	return nil
}

// uploadPackage uploads a package to Fury.io
func (p *FuryPublisher) uploadPackage(ctx context.Context, token string, a artifact.Artifact) error {
	log.Debug("Uploading package", "name", a.Name)

	file, err := os.Open(a.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("package", filepath.Base(a.Path))
	if err != nil {
		return err
	}

	if _, err := io.Copy(part, file); err != nil {
		return err
	}

	writer.Close()

	url := fmt.Sprintf("https://push.fury.io/%s/", p.config.Account)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, &body)
	req.SetBasicAuth(token, "")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed: %s", respBody)
	}

	return nil
}
