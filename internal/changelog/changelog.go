/*
Package changelog provides changelog generation for Releaser.
*/
package changelog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/git"
)

// Options for changelog generation
type Options struct {
	ConfigFile string
	Since      string
	Until      string
	UseAI      bool
	Format     string
}

// Generator generates changelogs
type Generator struct {
	options Options
	config  *config.Config
}

// New creates a new changelog generator
func New(opts Options) (*Generator, error) {
	cfgPath := opts.ConfigFile
	if cfgPath == "" {
		cfgPath = ".releaser.yaml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}

	return &Generator{
		options: opts,
		config:  cfg,
	}, nil
}

// Generate generates a changelog
func (g *Generator) Generate(ctx context.Context) (string, error) {
	// Get git info
	gitInfo, err := git.GetInfo(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get git info: %w", err)
	}

	// Determine range
	since := g.options.Since
	if since == "" && gitInfo.PreviousTag != "" {
		since = gitInfo.PreviousTag
	}

	// Get commits
	commits, err := git.GetCommitsSince(ctx, since)
	if err != nil {
		return "", fmt.Errorf("failed to get commits: %w", err)
	}

	// Filter commits
	if g.config.Changelog.Filters.Exclude != nil || g.config.Changelog.Filters.Include != nil {
		commits = git.FilterCommits(commits, g.config.Changelog.Filters.Include, g.config.Changelog.Filters.Exclude)
	}

	// Group commits
	var groups []git.CommitGroup
	for _, g := range g.config.Changelog.Groups {
		groups = append(groups, git.CommitGroup{
			Title:  g.Title,
			Regexp: g.Regexp,
			Order:  g.Order,
		})
	}
	grouped := git.GroupCommits(commits, groups)

	// Sort groups by order
	sort.Slice(grouped, func(i, j int) bool {
		return grouped[i].Order < grouped[j].Order
	})

	// Generate changelog content
	var changelog string
	switch g.options.Format {
	case "json":
		changelog, err = g.formatJSON(grouped, gitInfo)
	case "yaml":
		changelog, err = g.formatYAML(grouped, gitInfo)
	default:
		changelog, err = g.formatMarkdown(grouped, gitInfo)
	}

	if err != nil {
		return "", err
	}

	// Enhance with AI if requested
	if g.options.UseAI || g.config.Changelog.AI.Enabled {
		enhanced, err := g.enhanceWithAI(ctx, changelog)
		if err != nil {
			// Log warning but don't fail
			fmt.Printf("Warning: AI enhancement failed: %v\n", err)
		} else {
			changelog = enhanced
		}
	}

	return changelog, nil
}

// formatMarkdown formats the changelog as Markdown
func (g *Generator) formatMarkdown(groups []git.GroupedCommits, gitInfo *git.Info) (string, error) {
	var buf bytes.Buffer

	// Header
	if g.config.Release.Header != "" {
		header, err := applyTemplate(g.config.Release.Header, gitInfo)
		if err != nil {
			return "", err
		}
		buf.WriteString(header)
		buf.WriteString("\n\n")
	}

	// Version header
	version := gitInfo.CurrentTag
	if version == "" {
		version = "Unreleased"
	}
	buf.WriteString(fmt.Sprintf("## %s\n\n", version))

	// Group entries
	for _, group := range groups {
		if len(group.Commits) == 0 {
			continue
		}

		buf.WriteString(fmt.Sprintf("### %s\n\n", group.Title))

		// Sort commits if configured
		commits := group.Commits
		if g.config.Changelog.Sort == "asc" {
			sort.Slice(commits, func(i, j int) bool {
				return commits[i].Date.Before(commits[j].Date)
			})
		} else if g.config.Changelog.Sort == "desc" {
			sort.Slice(commits, func(i, j int) bool {
				return commits[i].Date.After(commits[j].Date)
			})
		}

		for _, commit := range commits {
			// Clean up subject
			subject := cleanSubject(commit.Subject)
			shortHash := commit.Hash[:8]
			buf.WriteString(fmt.Sprintf("* %s (%s)\n", subject, shortHash))
		}
		buf.WriteString("\n")

		// Add divider if configured
		if g.config.Changelog.Divider != "" {
			buf.WriteString(g.config.Changelog.Divider)
			buf.WriteString("\n\n")
		}
	}

	// Footer
	if g.config.Release.Footer != "" {
		footer, err := applyTemplate(g.config.Release.Footer, gitInfo)
		if err != nil {
			return "", err
		}
		buf.WriteString(footer)
		buf.WriteString("\n")
	}

	return buf.String(), nil
}

// formatJSON formats the changelog as JSON
func (g *Generator) formatJSON(groups []git.GroupedCommits, gitInfo *git.Info) (string, error) {
	type entry struct {
		Hash    string `json:"hash"`
		Subject string `json:"subject"`
		Author  string `json:"author"`
		Date    string `json:"date"`
	}

	type group struct {
		Title   string  `json:"title"`
		Entries []entry `json:"entries"`
	}

	type changelog struct {
		Version string  `json:"version"`
		Date    string  `json:"date"`
		Groups  []group `json:"groups"`
	}

	cl := changelog{
		Version: gitInfo.CurrentTag,
		Date:    gitInfo.CommitDate.Format("2006-01-02"),
	}

	for _, g := range groups {
		grp := group{Title: g.Title}
		for _, c := range g.Commits {
			grp.Entries = append(grp.Entries, entry{
				Hash:    c.Hash[:8],
				Subject: cleanSubject(c.Subject),
				Author:  c.AuthorName,
				Date:    c.Date.Format("2006-01-02"),
			})
		}
		cl.Groups = append(cl.Groups, grp)
	}

	data, err := json.MarshalIndent(cl, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// formatYAML formats the changelog as YAML
func (g *Generator) formatYAML(groups []git.GroupedCommits, gitInfo *git.Info) (string, error) {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("version: %s\n", gitInfo.CurrentTag))
	buf.WriteString(fmt.Sprintf("date: %s\n", gitInfo.CommitDate.Format("2006-01-02")))
	buf.WriteString("groups:\n")

	for _, group := range groups {
		buf.WriteString(fmt.Sprintf("  - title: %s\n", group.Title))
		buf.WriteString("    entries:\n")
		for _, c := range group.Commits {
			buf.WriteString(fmt.Sprintf("      - hash: %s\n", c.Hash[:8]))
			buf.WriteString(fmt.Sprintf("        subject: %s\n", cleanSubject(c.Subject)))
			buf.WriteString(fmt.Sprintf("        author: %s\n", c.AuthorName))
			buf.WriteString(fmt.Sprintf("        date: %s\n", c.Date.Format("2006-01-02")))
		}
	}

	return buf.String(), nil
}

// enhanceWithAI enhances the changelog with AI
func (g *Generator) enhanceWithAI(ctx context.Context, changelog string) (string, error) {
	// Get AI configuration
	aiConfig := g.config.Changelog.AI
	if aiConfig.Provider == "" {
		aiConfig.Provider = "openai"
	}
	if aiConfig.Model == "" {
		aiConfig.Model = "gpt-4"
	}

	// Get API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("AI_API_KEY")
	}
	if apiKey == "" {
		return changelog, fmt.Errorf("OPENAI_API_KEY or AI_API_KEY environment variable not set")
	}

	// Default prompt
	prompt := aiConfig.Prompt
	if prompt == "" {
		prompt = `You are a technical writer. Please improve the following changelog by:
1. Fixing any typos or grammatical errors
2. Making descriptions clearer and more user-friendly
3. Ensuring consistent formatting
4. Adding context where helpful

Keep the same structure and format. Return only the improved changelog without any explanation.

Here is the changelog:

`
	}

	// Build request based on provider
	switch aiConfig.Provider {
	case "openai", "":
		return g.callOpenAI(ctx, apiKey, aiConfig.Model, prompt+changelog)
	case "anthropic":
		return g.callAnthropic(ctx, apiKey, aiConfig.Model, prompt+changelog)
	default:
		return changelog, fmt.Errorf("unsupported AI provider: %s", aiConfig.Provider)
	}
}

// OpenAIRequest represents an OpenAI API request
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
}

// OpenAIMessage represents a message in OpenAI format
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIResponse represents an OpenAI API response
type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// callOpenAI calls the OpenAI API
func (g *Generator) callOpenAI(ctx context.Context, apiKey, model, prompt string) (string, error) {
	reqBody := OpenAIRequest{
		Model: model,
		Messages: []OpenAIMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.3,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var openAIResp OpenAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if openAIResp.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s", openAIResp.Error.Message)
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return openAIResp.Choices[0].Message.Content, nil
}

// AnthropicRequest represents an Anthropic API request
type AnthropicRequest struct {
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
	Messages  []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

// AnthropicResponse represents an Anthropic API response
type AnthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// callAnthropic calls the Anthropic API
func (g *Generator) callAnthropic(ctx context.Context, apiKey, model, prompt string) (string, error) {
	if model == "" || model == "gpt-4" {
		model = "claude-3-sonnet-20240229"
	}

	reqBody := AnthropicRequest{
		Model:     model,
		MaxTokens: 4096,
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if anthropicResp.Error != nil {
		return "", fmt.Errorf("Anthropic API error: %s", anthropicResp.Error.Message)
	}

	if len(anthropicResp.Content) == 0 {
		return "", fmt.Errorf("no response from Anthropic")
	}

	return anthropicResp.Content[0].Text, nil
}

// cleanSubject cleans up a commit subject
func cleanSubject(subject string) string {
	// Remove conventional commit prefixes for display
	re := regexp.MustCompile(`^(feat|fix|docs|style|refactor|perf|test|chore|build|ci)(\(.+\))?:\s*`)
	subject = re.ReplaceAllString(subject, "")

	// Capitalize first letter
	if len(subject) > 0 {
		subject = strings.ToUpper(subject[:1]) + subject[1:]
	}

	return subject
}

// applyTemplate applies a template string
func applyTemplate(tmpl string, gitInfo *git.Info) (string, error) {
	t, err := template.New("").Parse(tmpl)
	if err != nil {
		return "", err
	}

	data := map[string]interface{}{
		"Tag":         gitInfo.CurrentTag,
		"Version":     strings.TrimPrefix(gitInfo.CurrentTag, "v"),
		"PreviousTag": gitInfo.PreviousTag,
		"Date":        gitInfo.CommitDate.Format("2006-01-02"),
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
