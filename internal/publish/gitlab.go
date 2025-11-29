// Package publish provides publishing functionality for Releaser.
package publish

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/oarkflow/releaser/internal/artifact"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// GitLabPublisher publishes to GitLab Releases
type GitLabPublisher struct {
	config  config.Release
	tmplCtx *tmpl.Context
	token   string
	baseURL string
}

// NewGitLabPublisher creates a new GitLab publisher
func NewGitLabPublisher(cfg config.Release, tmplCtx *tmpl.Context) *GitLabPublisher {
	baseURL := os.Getenv("GITLAB_URL")
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}

	return &GitLabPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
		token:   os.Getenv("GITLAB_TOKEN"),
		baseURL: strings.TrimSuffix(baseURL, "/"),
	}
}

// Publish publishes artifacts to GitLab Releases
func (p *GitLabPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	if p.token == "" {
		return fmt.Errorf("GITLAB_TOKEN is required")
	}

	owner := p.config.GitLab.Owner
	repo := p.config.GitLab.Name

	if owner == "" || repo == "" {
		return fmt.Errorf("GitLab owner and repo are required in release.gitlab config")
	}

	// Apply templates
	owner, _ = p.tmplCtx.Apply(owner)
	repo, _ = p.tmplCtx.Apply(repo)

	projectPath := url.PathEscape(owner + "/" + repo)

	log.Info("Publishing to GitLab Releases", "project", owner+"/"+repo, "base_url", p.baseURL)

	// Get or create release
	tag := p.tmplCtx.Get("Tag")
	releaseURL, err := p.getOrCreateRelease(ctx, projectPath, tag)
	if err != nil {
		return err
	}

	// Upload assets as generic packages, then link to release
	for _, a := range artifacts {
		if err := p.uploadAndLinkAsset(ctx, projectPath, tag, a); err != nil {
			return fmt.Errorf("failed to upload %s: %w", a.Name, err)
		}
	}

	log.Info("Published to GitLab Releases", "url", releaseURL)
	return nil
}

// getOrCreateRelease gets or creates a GitLab release
func (p *GitLabPublisher) getOrCreateRelease(ctx context.Context, projectPath, tag string) (string, error) {
	// Try to get existing release
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/releases/%s", p.baseURL, projectPath, url.PathEscape(tag))

	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	req.Header.Set("PRIVATE-TOKEN", p.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var release struct {
			Links struct {
				Self string `json:"self"`
			} `json:"_links"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			return "", err
		}
		log.Debug("Found existing GitLab release", "tag", tag)
		return release.Links.Self, nil
	}

	// Create new release
	name := p.config.NameTemplate
	if name == "" {
		name = tag
	}
	name, _ = p.tmplCtx.Apply(name)

	description := p.tmplCtx.Get("Changelog")
	if description == "" {
		description = fmt.Sprintf("Release %s", tag)
	}

	body := map[string]interface{}{
		"tag_name":    tag,
		"name":        name,
		"description": description,
	}

	bodyJSON, _ := json.Marshal(body)
	apiURL = fmt.Sprintf("%s/api/v4/projects/%s/releases", p.baseURL, projectPath)

	req, _ = http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyJSON))
	req.Header.Set("PRIVATE-TOKEN", p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create release: %s", respBody)
	}

	var release struct {
		Links struct {
			Self string `json:"self"`
		} `json:"_links"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	log.Info("Created GitLab release", "tag", tag)
	return release.Links.Self, nil
}

// uploadAndLinkAsset uploads an asset to GitLab Package Registry and links it to the release
func (p *GitLabPublisher) uploadAndLinkAsset(ctx context.Context, projectPath, tag string, a artifact.Artifact) error {
	log.Debug("Uploading asset to GitLab", "name", a.Name)

	file, err := os.Open(a.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	// Upload to Generic Package Registry
	version := strings.TrimPrefix(p.tmplCtx.Get("Version"), "v")
	packageURL := fmt.Sprintf("%s/api/v4/projects/%s/packages/generic/release/%s/%s",
		p.baseURL, projectPath, version, a.Name)

	req, _ := http.NewRequestWithContext(ctx, "PUT", packageURL, file)
	req.Header.Set("PRIVATE-TOKEN", p.token)
	req.ContentLength = stat.Size()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upload package: %s", respBody)
	}

	// Link asset to release
	linkURL := fmt.Sprintf("%s/api/v4/projects/%s/releases/%s/assets/links",
		p.baseURL, projectPath, url.PathEscape(tag))

	linkBody := map[string]interface{}{
		"name":      a.Name,
		"url":       packageURL,
		"link_type": getLinkType(a),
	}

	linkJSON, _ := json.Marshal(linkBody)
	req, _ = http.NewRequestWithContext(ctx, "POST", linkURL, bytes.NewReader(linkJSON))
	req.Header.Set("PRIVATE-TOKEN", p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		// Ignore "already exists" errors
		if !strings.Contains(string(respBody), "already exists") {
			return fmt.Errorf("failed to link asset: %s", respBody)
		}
	}

	log.Debug("Asset uploaded and linked", "name", a.Name)
	return nil
}

// getLinkType returns the GitLab link type for an artifact
func getLinkType(a artifact.Artifact) string {
	ext := strings.ToLower(filepath.Ext(a.Name))
	switch ext {
	case ".deb", ".rpm", ".apk", ".msi", ".exe", ".dmg", ".pkg":
		return "package"
	case ".tar.gz", ".zip", ".tar.xz", ".tar.bz2":
		return "other"
	default:
		return "other"
	}
}
