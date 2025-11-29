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
	"strings"

	"github.com/charmbracelet/log"

	"github.com/oarkflow/releaser/internal/artifact"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// BitbucketPublisher publishes to Bitbucket Downloads
type BitbucketPublisher struct {
	config   config.Release
	tmplCtx  *tmpl.Context
	username string
	password string
	baseURL  string
}

// NewBitbucketPublisher creates a new Bitbucket publisher
func NewBitbucketPublisher(cfg config.Release, tmplCtx *tmpl.Context) *BitbucketPublisher {
	baseURL := os.Getenv("BITBUCKET_URL")
	if baseURL == "" {
		baseURL = "https://api.bitbucket.org/2.0"
	}

	return &BitbucketPublisher{
		config:   cfg,
		tmplCtx:  tmplCtx,
		username: os.Getenv("BITBUCKET_USERNAME"),
		password: os.Getenv("BITBUCKET_APP_PASSWORD"),
		baseURL:  strings.TrimSuffix(baseURL, "/"),
	}
}

// Publish publishes artifacts to Bitbucket Downloads
func (p *BitbucketPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	if p.username == "" || p.password == "" {
		return fmt.Errorf("BITBUCKET_USERNAME and BITBUCKET_APP_PASSWORD are required")
	}

	// Get workspace and repo from config (Bitbucket uses workspace/repo)
	// Reuse GitHub config structure for simplicity
	workspace := p.config.GitHub.Owner
	repo := p.config.GitHub.Name

	if workspace == "" || repo == "" {
		return fmt.Errorf("Bitbucket workspace and repo are required (use release.github.owner/name)")
	}

	// Apply templates
	workspace, _ = p.tmplCtx.Apply(workspace)
	repo, _ = p.tmplCtx.Apply(repo)

	log.Info("Publishing to Bitbucket Downloads", "workspace", workspace, "repo", repo)

	// Upload each artifact to Downloads
	for _, a := range artifacts {
		if err := p.uploadDownload(ctx, workspace, repo, a); err != nil {
			return fmt.Errorf("failed to upload %s: %w", a.Name, err)
		}
	}

	log.Info("Published to Bitbucket Downloads")
	return nil
}

// uploadDownload uploads a file to Bitbucket Downloads
func (p *BitbucketPublisher) uploadDownload(ctx context.Context, workspace, repo string, a artifact.Artifact) error {
	log.Debug("Uploading to Bitbucket Downloads", "name", a.Name)

	file, err := os.Open(a.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Bitbucket Downloads API uses multipart form
	url := fmt.Sprintf("%s/repositories/%s/%s/downloads", p.baseURL, workspace, repo)

	// Create multipart form body
	var body bytes.Buffer
	boundary := "----WebKitFormBoundary7MA4YWxkTrZu0gW"

	body.WriteString("--" + boundary + "\r\n")
	body.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"files\"; filename=\"%s\"\r\n", a.Name))
	body.WriteString("Content-Type: application/octet-stream\r\n\r\n")

	fileContent, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	body.Write(fileContent)
	body.WriteString("\r\n--" + boundary + "--\r\n")

	req, err := http.NewRequestWithContext(ctx, "POST", url, &body)
	if err != nil {
		return err
	}

	req.SetBasicAuth(p.username, p.password)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed: %s", respBody)
	}

	log.Debug("Asset uploaded to Bitbucket", "name", a.Name)
	return nil
}

// GiteaPublisher publishes to Gitea Releases
type GiteaPublisher struct {
	config  config.Release
	tmplCtx *tmpl.Context
	token   string
	baseURL string
}

// NewGiteaPublisher creates a new Gitea publisher
func NewGiteaPublisher(cfg config.Release, tmplCtx *tmpl.Context) *GiteaPublisher {
	baseURL := os.Getenv("GITEA_URL")
	if baseURL == "" {
		baseURL = "https://gitea.com"
	}

	return &GiteaPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
		token:   os.Getenv("GITEA_TOKEN"),
		baseURL: strings.TrimSuffix(baseURL, "/"),
	}
}

// Publish publishes artifacts to Gitea Releases
func (p *GiteaPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	if p.token == "" {
		return fmt.Errorf("GITEA_TOKEN is required")
	}

	owner := p.config.Gitea.Owner
	repo := p.config.Gitea.Name

	if owner == "" || repo == "" {
		return fmt.Errorf("Gitea owner and repo are required in release.gitea config")
	}

	// Apply templates
	owner, _ = p.tmplCtx.Apply(owner)
	repo, _ = p.tmplCtx.Apply(repo)

	log.Info("Publishing to Gitea Releases", "owner", owner, "repo", repo, "base_url", p.baseURL)

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

	log.Info("Published to Gitea Releases")
	return nil
}

// getOrCreateRelease gets or creates a Gitea release
func (p *GiteaPublisher) getOrCreateRelease(ctx context.Context, owner, repo, tag string) (int64, error) {
	// Try to get existing release by tag
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/releases/tags/%s", p.baseURL, owner, repo, tag)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "token "+p.token)

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
		log.Debug("Found existing Gitea release", "tag", tag)
		return release.ID, nil
	}

	// Create new release
	name := p.config.NameTemplate
	if name == "" {
		name = tag
	}
	name, _ = p.tmplCtx.Apply(name)

	body := map[string]interface{}{
		"tag_name":   tag,
		"name":       name,
		"body":       p.tmplCtx.Get("Changelog"),
		"draft":      p.config.Draft,
		"prerelease": p.config.Prerelease == "true",
	}

	if p.config.TargetCommitish != "" {
		body["target_commitish"] = p.config.TargetCommitish
	}

	bodyJSON, _ := json.Marshal(body)
	url = fmt.Sprintf("%s/api/v1/repos/%s/%s/releases", p.baseURL, owner, repo)

	req, _ = http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	req.Header.Set("Authorization", "token "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to create release: %s", respBody)
	}

	var release struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return 0, err
	}

	log.Info("Created Gitea release", "tag", tag, "id", release.ID)
	return release.ID, nil
}

// uploadAsset uploads an asset to a Gitea release
func (p *GiteaPublisher) uploadAsset(ctx context.Context, owner, repo string, releaseID int64, a artifact.Artifact) error {
	log.Debug("Uploading asset to Gitea", "name", a.Name)

	file, err := os.Open(a.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/releases/%d/assets?name=%s",
		p.baseURL, owner, repo, releaseID, a.Name)

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
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upload asset: %s", respBody)
	}

	log.Debug("Asset uploaded to Gitea", "name", a.Name)
	return nil
}
