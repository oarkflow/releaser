// Package announce provides release announcement functionality.
package announce

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/template"

	"github.com/charmbracelet/log"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// Announcer sends release announcements.
type Announcer struct {
	config  config.Announce
	tmplCtx *tmpl.Context
}

// NewAnnouncer creates a new announcer.
func NewAnnouncer(cfg config.Announce, tmplCtx *tmpl.Context) *Announcer {
	return &Announcer{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Run sends all configured announcements.
func (a *Announcer) Run(ctx context.Context) error {
	if a.config.Skip == "true" {
		log.Info("Skipping announcements")
		return nil
	}

	log.Info("Sending release announcements")

	var errs []error

	// Slack
	if a.config.Slack.Enabled {
		if err := a.announceSlack(ctx); err != nil {
			errs = append(errs, fmt.Errorf("slack: %w", err))
		}
	}

	// Discord
	if a.config.Discord.Enabled {
		if err := a.announceDiscord(ctx); err != nil {
			errs = append(errs, fmt.Errorf("discord: %w", err))
		}
	}

	// Teams
	if a.config.Teams.Enabled {
		if err := a.announceTeams(ctx); err != nil {
			errs = append(errs, fmt.Errorf("teams: %w", err))
		}
	}

	// Mastodon
	if a.config.Mastodon.Enabled {
		if err := a.announceMastodon(ctx); err != nil {
			errs = append(errs, fmt.Errorf("mastodon: %w", err))
		}
	}

	// Telegram
	if a.config.Telegram.Enabled {
		if err := a.announceTelegram(ctx); err != nil {
			errs = append(errs, fmt.Errorf("telegram: %w", err))
		}
	}

	// Webhook
	if a.config.Webhook.Enabled {
		if err := a.announceWebhook(ctx); err != nil {
			errs = append(errs, fmt.Errorf("webhook: %w", err))
		}
	}

	// SMTP
	if a.config.SMTP.Enabled {
		if err := a.announceSMTP(ctx); err != nil {
			errs = append(errs, fmt.Errorf("smtp: %w", err))
		}
	}

	if len(errs) > 0 {
		var errStrings []string
		for _, err := range errs {
			errStrings = append(errStrings, err.Error())
		}
		return fmt.Errorf("some announcements failed: %s", strings.Join(errStrings, "; "))
	}

	log.Info("All announcements sent successfully")
	return nil
}

// announceSlack sends a Slack notification.
func (a *Announcer) announceSlack(ctx context.Context) error {
	webhook := os.Getenv("SLACK_WEBHOOK_URL")
	if webhook == "" {
		return fmt.Errorf("SLACK_WEBHOOK_URL environment variable not set")
	}

	message, err := a.formatMessage(a.config.Slack.MessageTemplate, "slack")
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"text": message,
	}

	if a.config.Slack.Channel != "" {
		payload["channel"] = a.config.Slack.Channel
	}
	if a.config.Slack.Username != "" {
		payload["username"] = a.config.Slack.Username
	}
	if a.config.Slack.IconEmoji != "" {
		payload["icon_emoji"] = a.config.Slack.IconEmoji
	}

	if err := a.postJSON(ctx, webhook, payload); err != nil {
		return err
	}

	log.Info("Slack announcement sent")
	return nil
}

// announceDiscord sends a Discord notification.
func (a *Announcer) announceDiscord(ctx context.Context) error {
	webhook := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhook == "" {
		return fmt.Errorf("DISCORD_WEBHOOK_URL environment variable not set")
	}

	message, err := a.formatMessage(a.config.Discord.MessageTemplate, "discord")
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"content": message,
	}

	if a.config.Discord.Author != "" {
		payload["username"] = a.config.Discord.Author
	}

	if err := a.postJSON(ctx, webhook, payload); err != nil {
		return err
	}

	log.Info("Discord announcement sent")
	return nil
}

// announceTeams sends a Microsoft Teams notification.
func (a *Announcer) announceTeams(ctx context.Context) error {
	webhook := os.Getenv("TEAMS_WEBHOOK_URL")
	if webhook == "" {
		return fmt.Errorf("TEAMS_WEBHOOK_URL environment variable not set")
	}

	message, err := a.formatMessage(a.config.Teams.MessageTemplate, "teams")
	if err != nil {
		return err
	}

	title := a.config.Teams.TitleTemplate
	if title == "" {
		title = fmt.Sprintf("%s %s Released", a.tmplCtx.Get("ProjectName"), a.tmplCtx.Get("Version"))
	}

	payload := map[string]interface{}{
		"@type":    "MessageCard",
		"@context": "http://schema.org/extensions",
		"summary":  title,
		"title":    title,
		"text":     message,
	}

	if a.config.Teams.Color != "" {
		payload["themeColor"] = a.config.Teams.Color
	}

	if err := a.postJSON(ctx, webhook, payload); err != nil {
		return err
	}

	log.Info("Teams announcement sent")
	return nil
}

// announceMastodon posts to Mastodon.
func (a *Announcer) announceMastodon(ctx context.Context) error {
	server := a.config.Mastodon.Server
	token := os.Getenv("MASTODON_ACCESS_TOKEN")
	if server == "" || token == "" {
		return fmt.Errorf("mastodon server config and MASTODON_ACCESS_TOKEN required")
	}

	message, err := a.formatMessage(a.config.Mastodon.MessageTemplate, "mastodon")
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/statuses", strings.TrimSuffix(server, "/"))

	payload := map[string]interface{}{
		"status": message,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("mastodon returned status %d", resp.StatusCode)
	}

	log.Info("Mastodon announcement sent")
	return nil
}

// announceTelegram sends a Telegram message.
func (a *Announcer) announceTelegram(ctx context.Context) error {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := a.config.Telegram.ChatID
	if token == "" || chatID == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN and telegram.chat_id required")
	}

	message, err := a.formatMessage(a.config.Telegram.MessageTemplate, "telegram")
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	parseMode := a.config.Telegram.ParseMode
	if parseMode == "" {
		parseMode = "Markdown"
	}

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": parseMode,
	}

	if err := a.postJSON(ctx, url, payload); err != nil {
		return err
	}

	log.Info("Telegram announcement sent")
	return nil
}

// announceWebhook sends a generic webhook.
func (a *Announcer) announceWebhook(ctx context.Context) error {
	webhookURL := a.config.Webhook.EndpointURL
	if webhookURL == "" {
		webhookURL = os.Getenv("ANNOUNCE_WEBHOOK_URL")
	}
	if webhookURL == "" {
		return fmt.Errorf("webhook URL not configured")
	}

	message, err := a.formatMessage(a.config.Webhook.MessageTemplate, "webhook")
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"version":     a.tmplCtx.Get("Version"),
		"tag":         a.tmplCtx.Get("Tag"),
		"project":     a.tmplCtx.Get("ProjectName"),
		"message":     message,
		"is_snapshot": a.tmplCtx.Get("IsSnapshot") == "true",
	}

	// Add custom headers
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	for key, value := range a.config.Webhook.Headers {
		expandedValue, _ := a.tmplCtx.Apply(value)
		req.Header.Set(key, expandedValue)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	log.Info("Webhook announcement sent")
	return nil
}

// announceSMTP sends an email notification.
func (a *Announcer) announceSMTP(ctx context.Context) error {
	// SMTP requires more complex setup - placeholder
	log.Warn("SMTP announcements require additional implementation")
	return nil
}

// formatMessage applies template to message.
func (a *Announcer) formatMessage(messageTemplate, service string) (string, error) {
	if messageTemplate == "" {
		messageTemplate = defaultMessageTemplate()
	}

	tmpl, err := template.New("message").Parse(messageTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse message template: %w", err)
	}

	data := map[string]interface{}{
		"ProjectName": a.tmplCtx.Get("ProjectName"),
		"Tag":         a.tmplCtx.Get("Tag"),
		"Version":     a.tmplCtx.Get("Version"),
		"Changelog":   a.tmplCtx.Get("Changelog"),
		"ReleaseURL":  a.tmplCtx.Get("ReleaseURL"),
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute message template: %w", err)
	}

	return buf.String(), nil
}

// postJSON sends a JSON POST request.
func (a *Announcer) postJSON(ctx context.Context, url string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	return nil
}

// defaultMessageTemplate returns the default announcement template.
func defaultMessageTemplate() string {
	return `🚀 {{ .ProjectName }} {{ .Version }} has been released!

{{ if .ReleaseURL }}Check it out: {{ .ReleaseURL }}{{ end }}`
}
