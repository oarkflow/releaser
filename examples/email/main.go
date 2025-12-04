package main

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// EmailConfig represents the fully normalized configuration.
type EmailConfig struct {
	From             string
	FromName         string
	EnvelopeFrom     string
	ReplyTo          []string
	To               []string
	CC               []string
	BCC              []string
	Subject          string
	Body             string
	TextBody         string
	HTMLBody         string
	Attachments      []Attachment
	Provider         string
	Transport        string
	Host             string
	Port             int
	Username         string
	Password         string
	APIKey           string
	APIToken         string
	Endpoint         string
	HTTPMethod       string
	Headers          map[string]string
	QueryParams      map[string]string
	HTTPPayload      map[string]interface{}
	PayloadFormat    string
	HTTPContentType  string
	HTMLTemplatePath string
	TextTemplatePath string
	BodyTemplatePath string
	AdditionalData   map[string]interface{}
	UseTLS           bool
	UseSSL           bool
	SkipTLSVerify    bool
	Timeout          time.Duration
	RetryCount       int
	RetryDelay       time.Duration
}

// Attachment describes a file to be included with the email.
type Attachment struct {
	Source   string
	Name     string
	MIMEType string
}

type encodedAttachment struct {
	Filename string
	MIMEType string
	Content  string
}

// ProviderSetting captures smart defaults for known providers.
type ProviderSetting struct {
	Host      string
	Port      int
	UseTLS    bool
	UseSSL    bool
	Transport string
	Endpoint  string
}

type payloadBuilder func(*EmailConfig) (interface{}, string, error)

type httpProviderProfile struct {
	Endpoint      string
	Method        string
	ContentType   string
	PayloadFormat string
	Headers       map[string]string
}

type placeholderMode int

const (
	placeholderModeInitial placeholderMode = iota
	placeholderModePostFinalize
)

var providerDefaults = map[string]ProviderSetting{
	"gmail":        {Host: "smtp.gmail.com", Port: 587, UseTLS: true},
	"google":       {Host: "smtp.gmail.com", Port: 587, UseTLS: true},
	"outlook":      {Host: "smtp-mail.outlook.com", Port: 587, UseTLS: true},
	"office365":    {Host: "smtp.office365.com", Port: 587, UseTLS: true},
	"yahoo":        {Host: "smtp.mail.yahoo.com", Port: 587, UseTLS: true},
	"zoho":         {Host: "smtp.zoho.com", Port: 587, UseTLS: true},
	"mailtrap":     {Host: "smtp.mailtrap.io", Port: 2525, UseTLS: true},
	"sendgrid":     {Host: "smtp.sendgrid.net", Port: 587, UseTLS: true},
	"mailgun":      {Host: "smtp.mailgun.org", Port: 587, UseTLS: true},
	"postmark":     {Host: "smtp.postmarkapp.com", Port: 587, UseTLS: true},
	"sparkpost":    {Host: "smtp.sparkpostmail.com", Port: 587, UseTLS: true},
	"amazon_ses":   {Host: "email-smtp.us-east-1.amazonaws.com", Port: 587, UseTLS: true},
	"amazon":       {Host: "email-smtp.us-east-1.amazonaws.com", Port: 587, UseTLS: true},
	"fastmail":     {Host: "smtp.fastmail.com", Port: 465, UseSSL: true},
	"protonmail":   {Transport: "http", Endpoint: "https://api.protonmail.ch"},
	"sendinblue":   {Host: "smtp-relay.sendinblue.com", Port: 587, UseTLS: true},
	"brevo":        {Host: "smtp-relay.brevo.com", Port: 587, UseTLS: true},
	"mailjet":      {Host: "in-v3.mailjet.com", Port: 587, UseTLS: true},
	"elasticemail": {Host: "smtp.elasticemail.com", Port: 2525, UseTLS: true},
}

var httpProviderProfiles = map[string]httpProviderProfile{
	"sendgrid": {
		Endpoint:      "https://api.sendgrid.com/v3/mail/send",
		Method:        http.MethodPost,
		ContentType:   "application/json",
		PayloadFormat: "sendgrid",
	},
	"brevo": {
		Endpoint:      "https://api.brevo.com/v3/smtp/email",
		Method:        http.MethodPost,
		ContentType:   "application/json",
		PayloadFormat: "brevo",
		Headers: map[string]string{
			"accept": "application/json",
		},
	},
	"sendinblue": {
		Endpoint:      "https://api.sendinblue.com/v3/smtp/email",
		Method:        http.MethodPost,
		ContentType:   "application/json",
		PayloadFormat: "brevo",
		Headers: map[string]string{
			"accept": "application/json",
		},
	},
	"mailtrap": {
		Endpoint:      "https://send.api.mailtrap.io/api/send",
		Method:        http.MethodPost,
		ContentType:   "application/json",
		PayloadFormat: "mailtrap",
	},
}

var httpPayloadBuilders = map[string]payloadBuilder{
	"sendgrid":   buildSendGridPayload,
	"brevo":      buildBrevoPayload,
	"sendinblue": buildBrevoPayload,
	"mailtrap":   buildMailtrapPayload,
}

var emailDomainMap = map[string]string{
	"gmail.com":      "gmail",
	"googlemail.com": "gmail",
	"outlook.com":    "outlook",
	"hotmail.com":    "outlook",
	"live.com":       "outlook",
	"office365.com":  "office365",
	"yahoo.com":      "yahoo",
	"yandex.com":     "mailgun",
	"zoho.com":       "zoho",
	"pm.me":          "protonmail",
	"protonmail.com": "protonmail",
	"fastmail.com":   "fastmail",
	"hey.com":        "mailgun",
	"icloud.com":     "mailgun",
	"me.com":         "mailgun",
	"mac.com":        "mailgun",
	"gmx.com":        "mailgun",
	"aol.com":        "mailgun",
}

var fieldAliases = map[string][]string{
	"from":              {"from", "sender", "from_email", "fromaddress", "sender_email", "mailfrom"},
	"from_name":         {"from_name", "sender_name", "fromname", "display_name", "name"},
	"reply_to":          {"reply_to", "replyto", "respond_to", "response_to"},
	"to":                {"to", "recipient", "recipients", "send_to", "sending_to", "mail_to", "to_email", "sendto"},
	"cc":                {"cc", "carbon_copy", "copy_to"},
	"bcc":               {"bcc", "blind_carbon_copy", "blind_copy"},
	"subject":           {"subject", "title", "email_subject"},
	"body":              {"body", "message", "msg", "content", "email_content", "text"},
	"body_html":         {"body_html", "html_body", "html", "message_html"},
	"body_text":         {"body_text", "text_body", "plain_text", "message_text"},
	"attachments":       {"attachments", "attachment", "files", "file", "attach"},
	"provider":          {"provider", "use", "service", "email_service"},
	"type":              {"type", "transport", "channel", "method"},
	"host":              {"host", "server", "smtp_host", "address", "addr", "smtp_server"},
	"port":              {"port", "smtp_port"},
	"username":          {"username", "user", "email", "login", "auth_user"},
	"password":          {"password", "pass", "pwd", "auth_password"},
	"api_key":           {"api_key", "apikey", "key"},
	"api_token":         {"api_token", "apitoken", "token", "access_token", "bearer", "bearer_token"},
	"endpoint":          {"endpoint", "url", "api_url", "api_endpoint"},
	"http_method":       {"http_method", "httpverb", "method"},
	"headers":           {"headers", "custom_headers", "http_headers"},
	"query_params":      {"query_params", "query", "params", "querystrings", "querystring"},
	"http_payload":      {"http_payload", "payload", "http_body", "custom_payload"},
	"payload_format":    {"payload_format", "http_profile", "http_format"},
	"http_content_type": {"http_content_type", "payload_content_type", "http_payload_type"},
	"html_template":     {"html_template", "template_html", "html_file", "html_path"},
	"text_template":     {"text_template", "template_text", "text_file", "text_path"},
	"body_template":     {"body_template", "message_template", "msg_template", "message_file", "template_message"},
	"timeout":           {"timeout", "timeout_seconds", "request_timeout", "http_timeout"},
	"retries":           {"retries", "retry", "retry_count", "attempts"},
	"retry_delay":       {"retry_delay", "retry_wait", "retry_backoff", "retry_pause"},
	"use_tls":           {"use_tls", "tls", "starttls", "enable_tls"},
	"use_ssl":           {"use_ssl", "ssl", "enable_ssl"},
	"skip_tls_verify":   {"skip_tls_verify", "insecure", "disable_tls_verify"},
}

func init() {
	log.SetFlags(0)
	for canonical, aliases := range fieldAliases {
		seen := make(map[string]struct{})
		normalized := make([]string, 0, len(aliases)+1)
		for _, alias := range aliases {
			alias = strings.TrimSpace(alias)
			if alias == "" {
				continue
			}
			lower := strings.ToLower(alias)
			if _, ok := seen[lower]; ok {
				continue
			}
			seen[lower] = struct{}{}
			normalized = append(normalized, alias)
		}
		if _, ok := seen[strings.ToLower(canonical)]; !ok {
			normalized = append(normalized, canonical)
		}
		fieldAliases[canonical] = normalized
	}
}

// RegisterProviderDefault adds or updates a provider's default settings.
// This allows extending the system with new email providers without modifying the core code.
func RegisterProviderDefault(provider string, setting ProviderSetting) {
	if provider == "" {
		return
	}
	providerDefaults[strings.ToLower(provider)] = setting
}

// RegisterHTTPProviderProfile adds or updates an HTTP provider profile.
// This enables support for new HTTP-based email services.
func RegisterHTTPProviderProfile(provider string, profile httpProviderProfile) {
	if provider == "" {
		return
	}
	httpProviderProfiles[strings.ToLower(provider)] = profile
}

// RegisterHTTPPayloadBuilder adds or updates a payload builder function for an HTTP provider.
// This allows custom payload formatting for new or existing providers.
func RegisterHTTPPayloadBuilder(provider string, builder payloadBuilder) {
	if provider == "" || builder == nil {
		return
	}
	httpPayloadBuilders[strings.ToLower(provider)] = builder
}

// RegisterEmailDomainMap adds or updates domain-to-provider mappings.
// This helps auto-detect providers based on email domains.
func RegisterEmailDomainMap(domain, provider string) {
	if domain == "" || provider == "" {
		return
	}
	emailDomainMap[strings.ToLower(domain)] = strings.ToLower(provider)
}

func main() {
	templatePath := flag.String("template", "", "path to the template JSON file (base config)")
	payloadPath := flag.String("payload", "", "path to the payload JSON file (overrides/template data)")
	flag.Parse()

	raw, err := loadConfigFiles(*templatePath, *payloadPath, flag.Args())
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	config, err := parseConfig(raw)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	log.Printf("Sending email to %v via %s (%s)...", config.To, config.TransportDetails(), config.ProviderOrHost())
	if err := sendEmail(config); err != nil {
		log.Fatalf("send failed: %v", err)
	}
	log.Println("Email sent successfully!")
}

func loadConfigFiles(templateFlag, payloadFlag string, args []string) (map[string]interface{}, error) {
	templatePath := templateFlag
	remaining := args
	if templatePath == "" {
		if len(remaining) == 0 {
			printUsage()
			return nil, errors.New("no template or config file provided")
		}
		templatePath = remaining[0]
		remaining = remaining[1:]
	}
	payloadPath := payloadFlag
	if payloadPath == "" && len(remaining) > 0 {
		payloadPath = remaining[0]
	}

	base, err := readJSONFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("template %s: %w", templatePath, err)
	}
	log.Printf("Loaded template: %s", templatePath)
	if payloadPath == "" {
		return base, nil
	}
	override, err := readJSONFile(payloadPath)
	if err != nil {
		return nil, fmt.Errorf("payload %s: %w", payloadPath, err)
	}
	log.Printf("Applying payload overrides: %s", payloadPath)
	return mergeConfigMaps(base, override), nil
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  go run main.go <config.json>")
	fmt.Println("  go run main.go --template template.json --payload payload.json")
	fmt.Println("  go run main.go template.json payload.json")
	fmt.Println("\nExamples:\n  go run main.go config.json\n  go run main.go --template template.smtp.json --payload payload.release.json")
}

func parseConfig(raw map[string]interface{}) (*EmailConfig, error) {
	norm := newNormalizedConfig(raw)
	cfg := &EmailConfig{
		Headers:     map[string]string{},
		QueryParams: map[string]string{},
	}

	cfg.From = getStringField(norm, "from")
	cfg.FromName = getStringField(norm, "from_name")
	cfg.ReplyTo = getStringArrayField(norm, "reply_to")
	cfg.To = getStringArrayField(norm, "to")
	cfg.CC = getStringArrayField(norm, "cc")
	cfg.BCC = getStringArrayField(norm, "bcc")
	cfg.Subject = getStringField(norm, "subject")
	cfg.Body = getStringField(norm, "body")
	cfg.TextBody = getStringField(norm, "body_text")
	cfg.HTMLBody = getStringField(norm, "body_html")
	cfg.HTMLTemplatePath = getStringField(norm, "html_template")
	cfg.TextTemplatePath = getStringField(norm, "text_template")
	cfg.BodyTemplatePath = getStringField(norm, "body_template")

	attachments, err := getAttachments(norm, "attachments")
	if err != nil {
		return nil, err
	}
	cfg.Attachments = attachments

	cfg.Provider = strings.ToLower(getStringField(norm, "provider"))
	cfg.Transport = strings.ToLower(getStringField(norm, "type"))
	cfg.Host = getStringField(norm, "host")
	cfg.Port = getIntField(norm, "port")
	cfg.Username = getStringField(norm, "username")
	cfg.Password = getStringField(norm, "password")
	cfg.APIKey = getStringField(norm, "api_key")
	cfg.APIToken = getStringField(norm, "api_token")
	cfg.Endpoint = getStringField(norm, "endpoint")
	cfg.HTTPMethod = strings.ToUpper(getStringField(norm, "http_method"))
	if cfg.HTTPMethod == "" {
		cfg.HTTPMethod = http.MethodPost
	}
	cfg.Headers = ensureStringMap(getStringMapField(norm, "headers"))
	cfg.QueryParams = ensureStringMap(getStringMapField(norm, "query_params"))
	cfg.HTTPPayload = getObjectField(norm, "http_payload")
	cfg.PayloadFormat = strings.ToLower(getStringField(norm, "payload_format"))
	cfg.HTTPContentType = getStringField(norm, "http_content_type")
	cfg.Timeout = getDurationField(norm, "timeout")
	cfg.RetryCount = getIntField(norm, "retries")
	cfg.RetryDelay = getDurationField(norm, "retry_delay")
	cfg.UseTLS = getBoolField(norm, "use_tls")
	cfg.UseSSL = getBoolField(norm, "use_ssl")
	cfg.SkipTLSVerify = getBoolField(norm, "skip_tls_verify")
	cfg.AdditionalData = norm.leftovers()
	if cfg.AdditionalData == nil {
		cfg.AdditionalData = map[string]interface{}{}
	}

	if err := applyPlaceholders(cfg, placeholderModeInitial); err != nil {
		return nil, err
	}

	if err := finalizeConfig(cfg); err != nil {
		return nil, err
	}

	if err := loadTemplateBodies(cfg); err != nil {
		return nil, err
	}

	if err := applyPlaceholders(cfg, placeholderModePostFinalize); err != nil {
		return nil, err
	}
	resolveBodies(cfg)

	return cfg, nil
}

func readJSONFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func mergeConfigMaps(base, override map[string]interface{}) map[string]interface{} {
	if base == nil {
		base = map[string]interface{}{}
	}
	for key, value := range override {
		if existing, ok := base[key]; ok {
			existingMap, okExisting := asMap(existing)
			valueMap, okValue := asMap(value)
			if okExisting && okValue {
				base[key] = mergeConfigMaps(existingMap, valueMap)
				continue
			}
		}
		base[key] = value
	}
	return base
}

func asMap(value interface{}) (map[string]interface{}, bool) {
	switch v := value.(type) {
	case map[string]interface{}:
		return v, true
	case map[string]string:
		result := make(map[string]interface{}, len(v))
		for key, val := range v {
			result[key] = val
		}
		return result, true
	default:
		return nil, false
	}
}

func finalizeConfig(cfg *EmailConfig) error {
	cfg.Provider = strings.ToLower(cfg.Provider)
	if cfg.Provider == "" {
		cfg.Provider = inferProvider(cfg.From, cfg.Username)
	}
	applyProviderDefaults(cfg)
	applyHTTPProfile(cfg)

	if cfg.Transport == "" {
		if cfg.Endpoint != "" && looksLikeURL(cfg.Endpoint) {
			cfg.Transport = "http"
		} else if looksLikeURL(cfg.Host) {
			cfg.Transport = "http"
		} else {
			cfg.Transport = "smtp"
		}
	}

	if cfg.Transport != "http" {
		cfg.Transport = "smtp"
	}

	if cfg.Transport == "http" && cfg.Endpoint == "" {
		cfg.Endpoint = cfg.Host
	}

	if cfg.Transport == "http" && cfg.Endpoint != "" && !looksLikeURL(cfg.Endpoint) {
		cfg.Endpoint = "https://" + strings.TrimLeft(cfg.Endpoint, ":/")
	}

	if cfg.From == "" && cfg.Username != "" {
		cfg.From = cfg.Username
	}
	name, addr := splitAddress(cfg.From)
	if cfg.FromName == "" {
		cfg.FromName = name
	}
	if addr == "" {
		return errors.New("sender address is required")
	}
	cfg.From = addr
	cfg.EnvelopeFrom = addr
	if cfg.Username == "" {
		cfg.Username = addr
	}

	if cfg.Subject == "" {
		cfg.Subject = "(no subject)"
	}
	resolveBodies(cfg)

	if len(cfg.To) == 0 {
		return errors.New("at least one recipient (to) is required")
	}

	if cfg.Transport == "smtp" {
		if cfg.Host == "" {
			return errors.New("smtp host is required")
		}
		if cfg.Port == 0 {
			if cfg.UseSSL {
				cfg.Port = 465
			} else if cfg.UseTLS {
				cfg.Port = 587
			} else {
				cfg.Port = 25
			}
		}
	} else {
		if cfg.Endpoint == "" {
			return errors.New("http endpoint is required when type=http")
		}
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.RetryCount <= 0 {
		cfg.RetryCount = 1
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 2 * time.Second
	}

	return nil
}

func applyHTTPProfile(cfg *EmailConfig) {
	if cfg.Transport != "http" {
		return
	}
	profile, ok := httpProviderProfiles[cfg.Provider]
	if !ok {
		return
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = profile.Endpoint
	}
	if cfg.HTTPMethod == "" && profile.Method != "" {
		cfg.HTTPMethod = profile.Method
	}
	if cfg.PayloadFormat == "" && profile.PayloadFormat != "" {
		cfg.PayloadFormat = profile.PayloadFormat
	}
	if cfg.HTTPContentType == "" {
		cfg.HTTPContentType = profile.ContentType
	}
	if cfg.Headers == nil {
		cfg.Headers = map[string]string{}
	}
	for k, v := range profile.Headers {
		if _, exists := cfg.Headers[k]; !exists {
			cfg.Headers[k] = v
		}
	}
}

func applyProviderDefaults(cfg *EmailConfig) {
	if cfg.Provider == "" {
		return
	}
	if defaults, ok := providerDefaults[cfg.Provider]; ok {
		if cfg.Host == "" {
			cfg.Host = defaults.Host
		}
		if cfg.Port == 0 {
			cfg.Port = defaults.Port
		}
		if !cfg.UseTLS && !cfg.UseSSL {
			cfg.UseTLS = defaults.UseTLS
			cfg.UseSSL = defaults.UseSSL
		}
		if cfg.Transport == "" && defaults.Transport != "" {
			cfg.Transport = defaults.Transport
		}
		if cfg.Endpoint == "" && defaults.Endpoint != "" {
			cfg.Endpoint = defaults.Endpoint
		}
	}
}

func inferProvider(addresses ...string) string {
	for _, addr := range addresses {
		_, email := splitAddress(addr)
		if email == "" {
			continue
		}
		parts := strings.Split(email, "@")
		if len(parts) != 2 {
			continue
		}
		domain := strings.ToLower(strings.TrimSpace(parts[1]))
		if provider, ok := emailDomainMap[domain]; ok {
			return provider
		}
	}
	return ""
}

func resolveBodies(cfg *EmailConfig) {
	text := strings.TrimSpace(cfg.TextBody)
	html := strings.TrimSpace(cfg.HTMLBody)
	base := strings.TrimSpace(cfg.Body)

	if html == "" && looksLikeHTML(base) {
		html = base
	}
	if text == "" {
		if html == "" {
			text = base
		} else if base != "" && !looksLikeHTML(base) {
			text = base
		}
	}
	if text == "" && html == "" {
		text = "(empty message)"
	}

	cfg.TextBody = text
	cfg.HTMLBody = html
}

func loadTemplateBodies(cfg *EmailConfig) error {
	if path := strings.TrimSpace(cfg.HTMLTemplatePath); path != "" {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read html template %s: %w", path, err)
		}
		cfg.HTMLBody = string(content)
		log.Printf("Loaded HTML template: %s", path)
	}
	if path := strings.TrimSpace(cfg.TextTemplatePath); path != "" {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read text template %s: %w", path, err)
		}
		cfg.TextBody = string(content)
		log.Printf("Loaded text template: %s", path)
	}
	if path := strings.TrimSpace(cfg.BodyTemplatePath); path != "" {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read body template %s: %w", path, err)
		}
		cfg.Body = string(content)
		log.Printf("Loaded message template: %s", path)
	}
	return nil
}

func sendEmail(cfg *EmailConfig) error {
	var lastErr error
	for attempt := 1; attempt <= cfg.RetryCount; attempt++ {
		if cfg.Transport == "http" {
			lastErr = sendViaHTTP(cfg)
		} else {
			lastErr = sendViaSMTP(cfg)
		}
		if lastErr == nil {
			return nil
		}
		if attempt < cfg.RetryCount {
			log.Printf("attempt %d/%d failed: %v (retrying in %s)", attempt, cfg.RetryCount, lastErr, cfg.RetryDelay)
			time.Sleep(cfg.RetryDelay)
		}
	}
	return lastErr
}

func sendViaSMTP(cfg *EmailConfig) error {
	msg, err := buildMessage(cfg)
	if err != nil {
		return err
	}
	recipients, err := gatherRecipients(cfg)
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		return errors.New("no valid recipients found")
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	var client *smtp.Client
	if cfg.UseSSL {
		client, err = dialTLSClient(cfg, addr)
	} else {
		client, err = dialPlainClient(cfg, addr)
	}
	if err != nil {
		return err
	}
	defer client.Quit()

	if cfg.UseTLS && !cfg.UseSSL {
		tlsConfig := &tls.Config{ServerName: cfg.Host, InsecureSkipVerify: cfg.SkipTLSVerify}
		if err := client.StartTLS(tlsConfig); err != nil {
			return err
		}
	}

	if cfg.Username != "" && cfg.Password != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}

	if err := client.Mail(cfg.EnvelopeFrom); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	return nil
}

func sendViaHTTP(cfg *EmailConfig) error {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		return errors.New("http endpoint is required")
	}
	if len(cfg.QueryParams) > 0 {
		if parsed, err := url.Parse(endpoint); err == nil {
			query := parsed.Query()
			for k, v := range cfg.QueryParams {
				query.Set(k, v)
			}
			parsed.RawQuery = query.Encode()
			endpoint = parsed.String()
		}
	}

	payload, hintedType, err := cfg.resolveHTTPPayload()
	if err != nil {
		return err
	}
	bodyReader, finalType, err := encodePayload(payload, hintedType)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(cfg.HTTPMethod, endpoint, bodyReader)
	if err != nil {
		return err
	}
	if len(cfg.Headers) == 0 {
		cfg.Headers = map[string]string{}
	}
	contentTypeSet := false
	if finalType != "" {
		req.Header.Set("Content-Type", finalType)
		contentTypeSet = true
	}
	for k, v := range cfg.Headers {
		if strings.EqualFold(k, "Content-Type") {
			contentTypeSet = true
		}
		req.Header.Set(k, v)
	}
	if !contentTypeSet {
		req.Header.Set("Content-Type", "application/json")
	}
	applyAuthHeaders(req, cfg)

	client := &http.Client{Timeout: cfg.Timeout}
	if cfg.SkipTLSVerify {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("http send failed: %s %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func (cfg *EmailConfig) resolveHTTPPayload() (interface{}, string, error) {
	if cfg.HTTPPayload != nil {
		return cfg.HTTPPayload, pickContentType(cfg.HTTPContentType, ""), nil
	}
	if cfg.PayloadFormat != "" {
		if builder, ok := httpPayloadBuilders[cfg.PayloadFormat]; ok {
			payload, contentType, err := builder(cfg)
			return payload, pickContentType(cfg.HTTPContentType, contentType), err
		}
	}
	if builder, ok := httpPayloadBuilders[cfg.Provider]; ok {
		payload, contentType, err := builder(cfg)
		return payload, pickContentType(cfg.HTTPContentType, contentType), err
	}
	payload, err := buildHTTPPayload(cfg)
	return payload, pickContentType(cfg.HTTPContentType, ""), err
}

func encodePayload(payload interface{}, contentType string) (io.Reader, string, error) {
	switch v := payload.(type) {
	case nil:
		return bytes.NewReader(nil), contentType, nil
	case []byte:
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		return bytes.NewReader(v), contentType, nil
	case string:
		if contentType == "" {
			contentType = "text/plain"
		}
		return strings.NewReader(v), contentType, nil
	case url.Values:
		if contentType == "" {
			contentType = "application/x-www-form-urlencoded"
		}
		return strings.NewReader(v.Encode()), contentType, nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, "", err
		}
		if contentType == "" {
			contentType = "application/json"
		}
		return bytes.NewReader(data), contentType, nil
	}
}

func pickContentType(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

func buildHTTPPayload(cfg *EmailConfig) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"from":        cfg.From,
		"from_name":   cfg.FromName,
		"reply_to":    cfg.ReplyTo,
		"to":          cfg.To,
		"cc":          cfg.CC,
		"bcc":         cfg.BCC,
		"subject":     cfg.Subject,
		"text_body":   cfg.TextBody,
		"html_body":   cfg.HTMLBody,
		"provider":    cfg.Provider,
		"attachments": []map[string]string{},
	}

	if len(cfg.Attachments) > 0 {
		files := make([]map[string]string, 0, len(cfg.Attachments))
		for _, att := range cfg.Attachments {
			encoded, err := encodeAttachment(att)
			if err != nil {
				return nil, err
			}
			files = append(files, encoded)
		}
		payload["attachments"] = files
	}

	payload = mergeAdditional(payload, cfg.AdditionalData, false)

	return payload, nil
}

func buildSendGridPayload(cfg *EmailConfig) (interface{}, string, error) {
	personalization := map[string]interface{}{
		"to": addressMaps(parseAddressList(cfg.To), "email", "name"),
	}
	if len(cfg.CC) > 0 {
		personalization["cc"] = addressMaps(parseAddressList(cfg.CC), "email", "name")
	}
	if len(cfg.BCC) > 0 {
		personalization["bcc"] = addressMaps(parseAddressList(cfg.BCC), "email", "name")
	}

	fromName, fromEmail := splitAddress(cfg.From)
	fromEntry := singleAddressMap(simpleAddress{Name: fromName, Email: fromEmail}, "email", "name")
	contents := make([]map[string]string, 0, 2)
	if cfg.TextBody != "" {
		contents = append(contents, map[string]string{"type": "text/plain", "value": cfg.TextBody})
	}
	if cfg.HTMLBody != "" {
		contents = append(contents, map[string]string{"type": "text/html", "value": cfg.HTMLBody})
	}
	if len(contents) == 0 {
		contents = append(contents, map[string]string{"type": "text/plain", "value": fallbackBody(cfg.TextBody)})
	}

	payload := map[string]interface{}{
		"personalizations": []interface{}{personalization},
		"from":             fromEntry,
		"subject":          cfg.Subject,
		"content":          contents,
	}
	if reply := firstAddressEntry(cfg.ReplyTo); reply.Email != "" {
		payload["reply_to"] = singleAddressMap(reply, "email", "name")
	}
	encoded, err := encodeAllAttachments(cfg)
	if err != nil {
		return nil, "", err
	}
	if len(encoded) > 0 {
		attachments := make([]map[string]string, 0, len(encoded))
		for _, att := range encoded {
			attachments = append(attachments, map[string]string{
				"content":  att.Content,
				"type":     att.MIMEType,
				"filename": att.Filename,
			})
		}
		payload["attachments"] = attachments
	}
	payload = mergeAdditional(payload, cfg.AdditionalData, true)
	return payload, "application/json", nil
}

func buildMailtrapPayload(cfg *EmailConfig) (interface{}, string, error) {
	fromName, fromEmail := splitAddress(cfg.From)
	sender := singleAddressMap(simpleAddress{Name: fromName, Email: fromEmail}, "email", "name")
	payload := map[string]interface{}{
		"from":    sender,
		"to":      addressMaps(parseAddressList(cfg.To), "email", "name"),
		"subject": cfg.Subject,
		"text":    fallbackBody(cfg.TextBody),
		"html":    cfg.HTMLBody,
	}
	if len(cfg.CC) > 0 {
		payload["cc"] = addressMaps(parseAddressList(cfg.CC), "email", "name")
	}
	if len(cfg.BCC) > 0 {
		payload["bcc"] = addressMaps(parseAddressList(cfg.BCC), "email", "name")
	}
	if reply := firstAddressEntry(cfg.ReplyTo); reply.Email != "" {
		payload["reply_to"] = singleAddressMap(reply, "email", "name")
	}
	encoded, err := encodeAllAttachments(cfg)
	if err != nil {
		return nil, "", err
	}
	if len(encoded) > 0 {
		attachments := make([]map[string]string, 0, len(encoded))
		for _, att := range encoded {
			attachments = append(attachments, map[string]string{
				"content":  att.Content,
				"type":     att.MIMEType,
				"filename": att.Filename,
			})
		}
		payload["attachments"] = attachments
	}
	payload = mergeAdditional(payload, cfg.AdditionalData, true)
	return payload, "application/json", nil
}

func buildBrevoPayload(cfg *EmailConfig) (interface{}, string, error) {
	fromName, fromEmail := splitAddress(cfg.From)
	sender := singleAddressMap(simpleAddress{Name: fromName, Email: fromEmail}, "email", "name")
	payload := map[string]interface{}{
		"sender":      sender,
		"to":          addressMaps(parseAddressList(cfg.To), "email", "name"),
		"subject":     cfg.Subject,
		"textContent": fallbackBody(cfg.TextBody),
		"htmlContent": cfg.HTMLBody,
	}
	if len(cfg.CC) > 0 {
		payload["cc"] = addressMaps(parseAddressList(cfg.CC), "email", "name")
	}
	if len(cfg.BCC) > 0 {
		payload["bcc"] = addressMaps(parseAddressList(cfg.BCC), "email", "name")
	}
	if reply := firstAddressEntry(cfg.ReplyTo); reply.Email != "" {
		payload["replyTo"] = singleAddressMap(reply, "email", "name")
	}
	encoded, err := encodeAllAttachments(cfg)
	if err != nil {
		return nil, "", err
	}
	if len(encoded) > 0 {
		attachments := make([]map[string]string, 0, len(encoded))
		for _, att := range encoded {
			attachments = append(attachments, map[string]string{
				"name":    att.Filename,
				"content": att.Content,
			})
		}
		payload["attachment"] = attachments
	}
	payload = mergeAdditional(payload, cfg.AdditionalData, true)
	return payload, "application/json", nil
}

func applyAuthHeaders(req *http.Request, cfg *EmailConfig) {
	token := strings.TrimSpace(cfg.APIToken)
	apiKey := strings.TrimSpace(cfg.APIKey)
	if token == "" {
		token = apiKey
	}
	switch cfg.Provider {
	case "brevo", "sendinblue":
		if apiKey == "" {
			apiKey = token
		}
		if apiKey == "" || req.Header.Get("api-key") != "" {
			return
		}
		req.Header.Set("api-key", apiKey)
		return
	case "mailgun":
		if token == "" {
			return
		}
		req.SetBasicAuth("api", token)
		return
	}
	if token == "" {
		return
	}
	if req.Header.Get("Authorization") != "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
}

func buildMessage(cfg *EmailConfig) (string, error) {
	var msg strings.Builder
	fromAddr := mail.Address{Name: cfg.FromName, Address: cfg.From}
	msg.WriteString(fmt.Sprintf("From: %s\r\n", fromAddr.String()))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(cfg.To, ", ")))
	if len(cfg.CC) > 0 {
		msg.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(cfg.CC, ", ")))
	}
	if len(cfg.ReplyTo) > 0 {
		msg.WriteString(fmt.Sprintf("Reply-To: %s\r\n", strings.Join(cfg.ReplyTo, ", ")))
	}
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", cfg.Subject))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	msg.WriteString(fmt.Sprintf("Message-ID: <%s@%s>\r\n", randomBoundary("msg"), cfg.Host))
	msg.WriteString("MIME-Version: 1.0\r\n")
	for k, v := range cfg.Headers {
		if strings.EqualFold(k, "Content-Type") {
			continue
		}
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}

	if len(cfg.Attachments) > 0 {
		mixedBoundary := randomBoundary("mixed")
		msg.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n\r\n", mixedBoundary))
		if err := writeBodyPart(&msg, cfg, mixedBoundary); err != nil {
			return "", err
		}
		for _, att := range cfg.Attachments {
			if err := writeAttachmentPart(&msg, att, mixedBoundary); err != nil {
				return "", err
			}
		}
		msg.WriteString(fmt.Sprintf("--%s--\r\n", mixedBoundary))
		return msg.String(), nil
	}

	if err := writeStandaloneBody(&msg, cfg); err != nil {
		return "", err
	}
	return msg.String(), nil
}

func writeBodyPart(msg *strings.Builder, cfg *EmailConfig, boundary string) error {
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	return writeAlternativeBody(msg, cfg)
}

func writeStandaloneBody(msg *strings.Builder, cfg *EmailConfig) error {
	return writeAlternativeBody(msg, cfg)
}

func writeAlternativeBody(msg *strings.Builder, cfg *EmailConfig) error {
	if cfg.HTMLBody != "" && cfg.TextBody != "" {
		altBoundary := randomBoundary("alt")
		msg.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%s\r\n\r\n", altBoundary))
		msg.WriteString(fmt.Sprintf("--%s\r\n", altBoundary))
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		msg.WriteString(cfg.TextBody)
		msg.WriteString("\r\n\r\n")
		msg.WriteString(fmt.Sprintf("--%s\r\n", altBoundary))
		msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		msg.WriteString(cfg.HTMLBody)
		msg.WriteString("\r\n\r\n")
		msg.WriteString(fmt.Sprintf("--%s--\r\n", altBoundary))
		return nil
	}

	contentType := "text/plain"
	body := cfg.TextBody
	if cfg.HTMLBody != "" {
		contentType = "text/html"
		body = cfg.HTMLBody
	}
	msg.WriteString(fmt.Sprintf("Content-Type: %s; charset=UTF-8\r\n\r\n", contentType))
	msg.WriteString(body)
	msg.WriteString("\r\n")
	return nil
}

func writeAttachmentPart(msg *strings.Builder, att Attachment, boundary string) error {
	data, filename, mimeType, err := loadAttachment(att)
	if err != nil {
		return err
	}
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString(fmt.Sprintf("Content-Type: %s\r\n", mimeType))
	msg.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", filename))
	msg.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	encoded := base64.StdEncoding.EncodeToString(data)
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		msg.WriteString(encoded[i:end])
		msg.WriteString("\r\n")
	}
	msg.WriteString("\r\n")
	return nil
}

func gatherRecipients(cfg *EmailConfig) ([]string, error) {
	unique := make(map[string]struct{})
	var recipients []string
	for _, set := range [][]string{cfg.To, cfg.CC, cfg.BCC} {
		for _, candidate := range set {
			_, addr := splitAddress(candidate)
			if addr == "" {
				continue
			}
			addr = strings.ToLower(strings.TrimSpace(addr))
			if addr == "" {
				continue
			}
			if _, exists := unique[addr]; exists {
				continue
			}
			unique[addr] = struct{}{}
			recipients = append(recipients, addr)
		}
	}
	return recipients, nil
}

func dialPlainClient(cfg *EmailConfig, addr string) (*smtp.Client, error) {
	dialer := &net.Dialer{Timeout: cfg.Timeout}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return client, nil
}

func dialTLSClient(cfg *EmailConfig, addr string) (*smtp.Client, error) {
	dialer := &net.Dialer{Timeout: cfg.Timeout}
	tlsConfig := &tls.Config{ServerName: cfg.Host, InsecureSkipVerify: cfg.SkipTLSVerify}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	if err != nil {
		return nil, err
	}
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return client, nil
}

func loadAttachment(att Attachment) ([]byte, string, string, error) {
	source := strings.TrimSpace(att.Source)
	if source == "" {
		return nil, "", "", errors.New("attachment source is empty")
	}
	if strings.HasPrefix(source, "data:") {
		return decodeDataURI(source, att)
	}
	if looksLikeURL(source) {
		data, name, err := downloadFile(source)
		if err != nil {
			return nil, "", "", err
		}
		mimeType := att.MIMEType
		if mimeType == "" {
			mimeType = detectMIMEType(name, data)
		}
		return data, name, mimeType, nil
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, "", "", err
	}
	filename := att.Name
	if filename == "" {
		filename = filepath.Base(source)
	}
	mimeType := att.MIMEType
	if mimeType == "" {
		mimeType = detectMIMEType(filename, data)
	}
	return data, filename, mimeType, nil
}

func decodeDataURI(uri string, att Attachment) ([]byte, string, string, error) {
	parts := strings.SplitN(uri, ",", 2)
	if len(parts) != 2 {
		return nil, "", "", fmt.Errorf("invalid data URI for attachment")
	}
	meta := parts[0]
	dataPart := parts[1]
	var data []byte
	var err error
	if strings.HasSuffix(meta, ";base64") {
		data, err = base64.StdEncoding.DecodeString(dataPart)
		if err != nil {
			return nil, "", "", err
		}
	} else {
		decoded, decErr := url.QueryUnescape(dataPart)
		if decErr != nil {
			return nil, "", "", decErr
		}
		data = []byte(decoded)
	}
	mimeType := att.MIMEType
	if mimeType == "" {
		mimeType = strings.TrimPrefix(meta, "data:")
		mimeType = strings.TrimSuffix(mimeType, ";base64")
	}
	name := att.Name
	if name == "" {
		name = "attachment.bin"
	}
	return data, name, mimeType, nil
}

func encodeAttachment(att Attachment) (map[string]string, error) {
	data, filename, mimeType, err := loadAttachment(att)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"filename":     filename,
		"content":      base64.StdEncoding.EncodeToString(data),
		"content_type": mimeType,
	}, nil
}

func encodeAllAttachments(cfg *EmailConfig) ([]encodedAttachment, error) {
	if len(cfg.Attachments) == 0 {
		return nil, nil
	}
	result := make([]encodedAttachment, 0, len(cfg.Attachments))
	for _, att := range cfg.Attachments {
		data, filename, mimeType, err := loadAttachment(att)
		if err != nil {
			return nil, err
		}
		result = append(result, encodedAttachment{
			Filename: filename,
			MIMEType: mimeType,
			Content:  base64.StdEncoding.EncodeToString(data),
		})
	}
	return result, nil
}

func detectMIMEType(filename string, data []byte) string {
	if ext := filepath.Ext(filename); ext != "" {
		if mt := mime.TypeByExtension(ext); mt != "" {
			return mt
		}
	}
	if len(data) == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(data)
}

func downloadFile(link string) ([]byte, string, error) {
	resp, err := http.Get(link)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to download %s: %s", link, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	filename := filenameFromURL(link)
	if disp := resp.Header.Get("Content-Disposition"); disp != "" {
		if name := parseFilenameFromDisposition(disp); name != "" {
			filename = name
		}
	}
	return data, filename, nil
}

func filenameFromURL(link string) string {
	parsed, err := url.Parse(link)
	if err != nil {
		return "attachment"
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) == 0 {
		return "attachment"
	}
	if segments[len(segments)-1] == "" {
		return "attachment"
	}
	return segments[len(segments)-1]
}

func parseFilenameFromDisposition(header string) string {
	parts := strings.Split(header, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "filename=") {
			return strings.Trim(part[len("filename="):], "\"")
		}
	}
	return ""
}

// ---------- normalization helpers ----------

type configEntry struct {
	original  string
	sanitized string
	value     interface{}
	used      bool
}

type normalizedConfig struct {
	entries map[string][]*configEntry
}

func newNormalizedConfig(raw map[string]interface{}) *normalizedConfig {
	entries := make(map[string][]*configEntry)
	for key, value := range raw {
		sanitized := sanitizeKey(key)
		e := &configEntry{original: key, sanitized: sanitized, value: value}
		entries[sanitized] = append(entries[sanitized], e)
	}
	return &normalizedConfig{entries: entries}
}

func (n *normalizedConfig) leftOverEntries() []*configEntry {
	var result []*configEntry
	for _, list := range n.entries {
		for _, entry := range list {
			if !entry.used {
				result = append(result, entry)
			}
		}
	}
	return result
}

func (n *normalizedConfig) leftovers() map[string]interface{} {
	result := make(map[string]interface{})
	for _, entry := range n.leftOverEntries() {
		result[entry.original] = entry.value
	}
	return result
}

func (n *normalizedConfig) pullValue(canonical string) (interface{}, bool) {
	if canonical == "" {
		return nil, false
	}
	if aliases, ok := fieldAliases[canonical]; ok {
		if val, ok := n.consumeAliases(aliases); ok {
			return val, true
		}
	}
	if val, ok := n.consumeExact(canonical); ok {
		return val, true
	}
	return n.consumeFuzzy(canonical)
}

func (n *normalizedConfig) consumeAliases(aliases []string) (interface{}, bool) {
	for _, alias := range aliases {
		if val, ok := n.consumeExact(alias); ok {
			return val, true
		}
	}
	return nil, false
}

func (n *normalizedConfig) consumeExact(key string) (interface{}, bool) {
	sanitized := sanitizeKey(key)
	if entries, ok := n.entries[sanitized]; ok {
		for _, entry := range entries {
			if entry.used {
				continue
			}
			entry.used = true
			return entry.value, true
		}
	}
	return nil, false
}

func (n *normalizedConfig) consumeFuzzy(target string) (interface{}, bool) {
	token := sanitizeKey(target)
	if len(token) < 4 {
		return nil, false
	}
	for key, entries := range n.entries {
		if len(key) < 4 {
			continue
		}
		if !strings.Contains(key, token) && !strings.Contains(token, key) {
			continue
		}
		for _, entry := range entries {
			if entry.used {
				continue
			}
			entry.used = true
			return entry.value, true
		}
	}
	return nil, false
}

func sanitizeKey(key string) string {
	lower := strings.ToLower(key)
	var b strings.Builder
	for _, r := range lower {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func getStringField(norm *normalizedConfig, canonical string) string {
	val, ok := norm.pullValue(canonical)
	if !ok || val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(v)
	}
}

func getStringArrayField(norm *normalizedConfig, canonical string) []string {
	val, ok := norm.pullValue(canonical)
	if !ok || val == nil {
		return nil
	}
	return normalizeStringSlice(val)
}

func normalizeStringSlice(val interface{}) []string {
	switch v := val.(type) {
	case string:
		return splitList(v)
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			switch entry := item.(type) {
			case string:
				if trimmed := strings.TrimSpace(entry); trimmed != "" {
					out = append(out, trimmed)
				}
			default:
				out = append(out, strings.TrimSpace(fmt.Sprint(entry)))
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	default:
		return []string{strings.TrimSpace(fmt.Sprint(v))}
	}
}

type simpleAddress struct {
	Name  string
	Email string
}

func parseAddressList(values []string) []simpleAddress {
	var result []simpleAddress
	for _, raw := range values {
		name, addr := splitAddress(raw)
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		result = append(result, simpleAddress{Name: strings.TrimSpace(name), Email: addr})
	}
	return result
}

func firstAddressEntry(values []string) simpleAddress {
	list := parseAddressList(values)
	if len(list) == 0 {
		return simpleAddress{}
	}
	return list[0]
}

func addressMaps(addresses []simpleAddress, emailKey, nameKey string) []map[string]string {
	result := make([]map[string]string, 0, len(addresses))
	for _, addr := range addresses {
		entry := map[string]string{emailKey: addr.Email}
		if addr.Name != "" {
			entry[nameKey] = addr.Name
		}
		result = append(result, entry)
	}
	return result
}

func singleAddressMap(addr simpleAddress, emailKey, nameKey string) map[string]string {
	if addr.Email == "" {
		return nil
	}
	entry := map[string]string{emailKey: addr.Email}
	if addr.Name != "" {
		entry[nameKey] = addr.Name
	}
	return entry
}

func splitList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func getIntField(norm *normalizedConfig, canonical string) int {
	val, ok := norm.pullValue(canonical)
	if !ok || val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return i
		}
	}
	return 0
}

func getBoolField(norm *normalizedConfig, canonical string) bool {
	val, ok := norm.pullValue(canonical)
	if !ok || val == nil {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		lower := strings.ToLower(strings.TrimSpace(v))
		return lower == "true" || lower == "yes" || lower == "1"
	case float64:
		return v != 0
	case int:
		return v != 0
	}
	return false
}

func getDurationField(norm *normalizedConfig, canonical string) time.Duration {
	val, ok := norm.pullValue(canonical)
	if !ok || val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return time.Duration(v) * time.Second
	case int:
		return time.Duration(v) * time.Second
	case string:
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil {
			return d
		}
		if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return time.Duration(i) * time.Second
		}
	}
	return 0
}

func getStringMapField(norm *normalizedConfig, canonical string) map[string]string {
	val, ok := norm.pullValue(canonical)
	if !ok || val == nil {
		return nil
	}
	result := map[string]string{}
	switch v := val.(type) {
	case map[string]interface{}:
		for key, value := range v {
			result[key] = strings.TrimSpace(fmt.Sprint(value))
		}
	case map[string]string:
		for key, value := range v {
			result[key] = strings.TrimSpace(value)
		}
	case []interface{}:
		for _, item := range v {
			switch entry := item.(type) {
			case string:
				k, val := splitKeyValue(entry)
				if k != "" {
					result[k] = val
				}
			case map[string]interface{}:
				for key, value := range entry {
					result[key] = strings.TrimSpace(fmt.Sprint(value))
				}
			}
		}
	case string:
		pairs := strings.FieldsFunc(v, func(r rune) bool { return r == ';' || r == ',' || r == '\n' })
		for _, pair := range pairs {
			k, val := splitKeyValue(pair)
			if k != "" {
				result[k] = val
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func getObjectField(norm *normalizedConfig, canonical string) map[string]interface{} {
	val, ok := norm.pullValue(canonical)
	if !ok || val == nil {
		return nil
	}
	return normalizeObject(val)
}

func mergeAdditional(base map[string]interface{}, extras map[string]interface{}, overwrite bool) map[string]interface{} {
	if len(extras) == 0 {
		return base
	}
	if base == nil {
		base = make(map[string]interface{}, len(extras))
	}
	for k, v := range extras {
		if !overwrite {
			if _, exists := base[k]; exists {
				continue
			}
		}
		base[k] = v
	}
	return base
}

func normalizeObject(val interface{}) map[string]interface{} {
	switch v := val.(type) {
	case map[string]interface{}:
		return v
	case map[string]string:
		result := make(map[string]interface{}, len(v))
		for k, value := range v {
			result[k] = value
		}
		return result
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil
		}
		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
			return decoded
		}
	}
	return nil
}

const placeholderMaxDepth = 5

var placeholderPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

func applyPlaceholders(cfg *EmailConfig, mode placeholderMode) error {
	resolver := newPlaceholderResolver(cfg)
	cfg.AdditionalData = resolver.expandObjectMap(cfg.AdditionalData)
	if err := resolver.Err(); err != nil {
		return err
	}

	for pass := 0; pass < 2; pass++ {
		resolver = newPlaceholderResolver(cfg)
		if mode == placeholderModeInitial && pass == 0 {
			cfg.From = strings.TrimSpace(resolver.expandString(cfg.From))
			cfg.FromName = strings.TrimSpace(resolver.expandString(cfg.FromName))
			cfg.Username = strings.TrimSpace(resolver.expandString(cfg.Username))
			cfg.Password = resolver.expandString(cfg.Password)
			cfg.APIKey = resolver.expandString(cfg.APIKey)
			cfg.APIToken = resolver.expandString(cfg.APIToken)
			cfg.Provider = strings.ToLower(strings.TrimSpace(resolver.expandString(cfg.Provider)))
			cfg.Transport = strings.ToLower(strings.TrimSpace(resolver.expandString(cfg.Transport)))
			cfg.Host = strings.TrimSpace(resolver.expandString(cfg.Host))
			cfg.Endpoint = strings.TrimSpace(resolver.expandString(cfg.Endpoint))
			cfg.HTMLTemplatePath = strings.TrimSpace(resolver.expandString(cfg.HTMLTemplatePath))
			cfg.TextTemplatePath = strings.TrimSpace(resolver.expandString(cfg.TextTemplatePath))
			cfg.BodyTemplatePath = strings.TrimSpace(resolver.expandString(cfg.BodyTemplatePath))
			cfg.ReplyTo = resolver.expandSlice(cfg.ReplyTo)
			cfg.To = resolver.expandSlice(cfg.To)
			cfg.CC = resolver.expandSlice(cfg.CC)
			cfg.BCC = resolver.expandSlice(cfg.BCC)
		}

		cfg.Subject = resolver.expandString(cfg.Subject)
		cfg.Body = resolver.expandString(cfg.Body)
		cfg.TextBody = resolver.expandString(cfg.TextBody)
		cfg.HTMLBody = resolver.expandString(cfg.HTMLBody)
		cfg.Endpoint = strings.TrimSpace(resolver.expandString(cfg.Endpoint))
		cfg.Headers = resolver.expandMap(cfg.Headers)
		cfg.QueryParams = resolver.expandMap(cfg.QueryParams)
		cfg.HTTPPayload = resolver.expandObjectMap(cfg.HTTPPayload)
		cfg.Attachments = resolver.expandAttachments(cfg.Attachments)

		if err := resolver.Err(); err != nil {
			return err
		}
	}
	return nil
}

type placeholderResolver struct {
	values  map[string]string
	missing map[string]struct{}
}

func newPlaceholderResolver(cfg *EmailConfig) *placeholderResolver {
	return &placeholderResolver{
		values:  buildPlaceholderValues(cfg),
		missing: map[string]struct{}{},
	}
}

func (r *placeholderResolver) Err() error {
	if len(r.missing) == 0 {
		return nil
	}
	keys := make([]string, 0, len(r.missing))
	for k := range r.missing {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return fmt.Errorf("unknown placeholders: %s", strings.Join(keys, ", "))
}

func (r *placeholderResolver) expandString(input string) string {
	if input == "" || !strings.Contains(input, "{{") {
		return input
	}
	result := input
	for depth := 0; depth < placeholderMaxDepth; depth++ {
		changed := false
		result = placeholderPattern.ReplaceAllStringFunc(result, func(match string) string {
			subs := placeholderPattern.FindStringSubmatch(match)
			if len(subs) < 2 {
				return ""
			}
			key := subs[1]
			if val, ok := r.lookup(key); ok {
				changed = true
				return val
			}
			r.logMissing(key)
			r.markMissing(key)
			return ""
		})
		if !changed || !strings.Contains(result, "{{") {
			break
		}
	}
	return result
}

func (r *placeholderResolver) expandSlice(values []string) []string {
	if len(values) == 0 {
		return values
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if expanded := strings.TrimSpace(r.expandString(value)); expanded != "" {
			result = append(result, expanded)
		}
	}
	return result
}

func (r *placeholderResolver) expandMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return values
	}
	for key, value := range values {
		values[key] = r.expandString(value)
	}
	return values
}

func (r *placeholderResolver) expandAttachments(list []Attachment) []Attachment {
	if len(list) == 0 {
		return list
	}
	result := make([]Attachment, 0, len(list))
	for _, att := range list {
		result = append(result, Attachment{
			Source:   strings.TrimSpace(r.expandString(att.Source)),
			Name:     r.expandString(att.Name),
			MIMEType: r.expandString(att.MIMEType),
		})
	}
	return result
}

func (r *placeholderResolver) expandObjectMap(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}
	expanded := r.expandInterface(input)
	if out, ok := expanded.(map[string]interface{}); ok {
		return out
	}
	return input
}

func (r *placeholderResolver) expandInterface(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return r.expandString(v)
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, item := range v {
			result[key] = r.expandInterface(item)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = r.expandInterface(item)
		}
		return result
	case []string:
		items := make([]interface{}, len(v))
		for i, item := range v {
			items[i] = r.expandInterface(item)
		}
		return items
	default:
		return value
	}
}

func (r *placeholderResolver) lookup(raw string) (string, bool) {
	key := strings.TrimSpace(raw)
	if key == "" {
		return "", false
	}
	lower := strings.ToLower(key)
	if strings.HasPrefix(lower, "env.") {
		name := strings.TrimSpace(raw[len("env."):])
		if name == "" {
			return "", false
		}
		if value, ok := os.LookupEnv(name); ok {
			logPlaceholderResolved("env."+name, value)
			return value, true
		}
		return "", false
	}
	if value, ok := r.values[lower]; ok {
		logPlaceholderResolved(lower, value)
		return value, true
	}
	return "", false
}

func (r *placeholderResolver) markMissing(key string) {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return
	}
	r.missing[key] = struct{}{}
}

func (r *placeholderResolver) logMissing(key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	log.Printf("[placeholders] missing value for %s", key)
}

func logPlaceholderResolved(key, value string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	log.Printf("[placeholders] %s => %s", key, maskPlaceholderValue(key, value))
}

func maskPlaceholderValue(key, value string) string {
	lower := strings.ToLower(key)
	if lower == "" {
		return value
	}
	sensitive := strings.Contains(lower, "pass") || strings.Contains(lower, "pwd") || strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "key") || strings.Contains(lower, "auth")
	if sensitive {
		if value == "" {
			return "(empty)"
		}
		return "[redacted]"
	}
	if len(value) > 200 {
		return value[:200] + "..."
	}
	return value
}

func buildPlaceholderValues(cfg *EmailConfig) map[string]string {
	values := map[string]string{}
	now := time.Now()
	registerValue(values, now.Format(time.RFC3339), true, "now", "datetime")
	registerValue(values, now.Format("2006-01-02"), true, "today", "date")
	registerValue(values, fmt.Sprintf("%d", now.Unix()), true, "timestamp")
	registerValue(values, cfg.Provider, true, "provider", "service")
	registerValue(values, cfg.Transport, true, "transport", "type")
	registerValue(values, cfg.HTTPMethod, true, "http_method", "verb")
	registerValue(values, cfg.Endpoint, true, "endpoint", "url", "api_url")
	registerValue(values, cfg.Host, true, "host", "server", "smtp_host")
	registerValue(values, cfg.From, true, "from", "sender", "from_email")
	registerValue(values, cfg.FromName, true, "from_name", "sender_name")
	registerValue(values, cfg.Username, true, "username", "user", "login")
	registerValue(values, cfg.Password, true, "password", "pass")
	registerValue(values, cfg.APIKey, true, "api_key", "key")
	registerValue(values, cfg.APIToken, true, "api_token", "token", "bearer")
	registerValue(values, cfg.Subject, true, "subject", "title")
	registerValue(values, cfg.Body, true, "body", "message", "content", "raw_body")
	registerValue(values, cfg.TextBody, true, "text_body", "text", "plain_text")
	registerValue(values, cfg.HTMLBody, true, "html_body", "html")
	registerSliceValue(values, cfg.To, true, "to", "recipients", "send_to")
	registerSliceValue(values, cfg.CC, true, "cc")
	registerSliceValue(values, cfg.BCC, true, "bcc")
	registerSliceValue(values, cfg.ReplyTo, true, "reply_to")
	if cfg.AdditionalData != nil {
		flattenAdditionalData(values, cfg.AdditionalData)
	}
	return values
}

func registerSliceValue(values map[string]string, source []string, overwrite bool, keys ...string) {
	clean := make([]string, 0, len(source))
	for _, item := range source {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			clean = append(clean, trimmed)
		}
	}
	if len(clean) == 0 {
		return
	}
	registerValue(values, strings.Join(clean, ","), overwrite, keys...)
}

func registerValue(values map[string]string, value string, overwrite bool, keys ...string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	for _, key := range keys {
		normalized := normalizePlaceholderKey(key)
		if normalized == "" {
			continue
		}
		if !overwrite {
			if _, exists := values[normalized]; exists {
				continue
			}
		}
		values[normalized] = value
	}
}

func normalizePlaceholderKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" {
		return ""
	}
	var b strings.Builder
	last := rune(0)
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			last = r
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			last = r
		case r == '.' || r == '_':
			if last == '.' || last == '_' || b.Len() == 0 {
				continue
			}
			b.WriteRune(r)
			last = r
		default:
			if last == '_' {
				continue
			}
			if b.Len() == 0 {
				continue
			}
			b.WriteRune('_')
			last = '_'
		}
	}
	return strings.Trim(b.String(), "._")
}

func flattenAdditionalData(values map[string]string, data map[string]interface{}) {
	var walker func(prefix string, input interface{})
	walker = func(prefix string, input interface{}) {
		switch v := input.(type) {
		case map[string]interface{}:
			for key, val := range v {
				next := normalizePlaceholderKey(key)
				if next == "" {
					continue
				}
				fullKey := next
				if prefix != "" {
					fullKey = prefix + "." + next
				}
				walker(fullKey, val)
			}
		case []interface{}:
			parts := make([]string, 0, len(v))
			for _, item := range v {
				parts = append(parts, strings.TrimSpace(fmt.Sprint(item)))
			}
			registerAdditionalValue(values, prefix, strings.Join(parts, ","))
		case []string:
			parts := make([]string, 0, len(v))
			for _, item := range v {
				parts = append(parts, strings.TrimSpace(item))
			}
			registerAdditionalValue(values, prefix, strings.Join(parts, ","))
		default:
			registerAdditionalValue(values, prefix, strings.TrimSpace(fmt.Sprint(input)))
		}
	}
	walker("", data)
}

func registerAdditionalValue(values map[string]string, key, value string) {
	if key == "" {
		return
	}
	registerValue(values, value, false, key)
	registerValue(values, value, true, "data."+key)
}

func splitKeyValue(input string) (string, string) {
	if strings.Contains(input, "=") {
		parts := strings.SplitN(input, "=", 2)
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	if strings.Contains(input, ":") {
		parts := strings.SplitN(input, ":", 2)
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return "", ""
}

func ensureStringMap(input map[string]string) map[string]string {
	if input != nil {
		return input
	}
	return map[string]string{}
}

func fallbackBody(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(empty message)"
	}
	return value
}

func getAttachments(norm *normalizedConfig, canonical string) ([]Attachment, error) {
	val, ok := norm.pullValue(canonical)
	if !ok || val == nil {
		return nil, nil
	}
	switch v := val.(type) {
	case string:
		return []Attachment{{Source: strings.TrimSpace(v)}}, nil
	case []interface{}:
		attachments := make([]Attachment, 0, len(v))
		for _, item := range v {
			att, err := normalizeAttachmentItem(item)
			if err != nil {
				return nil, err
			}
			if att.Source != "" {
				attachments = append(attachments, att)
			}
		}
		return attachments, nil
	case map[string]interface{}:
		var attachments []Attachment
		for _, item := range v {
			att, err := normalizeAttachmentItem(item)
			if err != nil {
				return nil, err
			}
			if att.Source != "" {
				attachments = append(attachments, att)
			}
		}
		return attachments, nil
	default:
		att, err := normalizeAttachmentItem(v)
		return []Attachment{att}, err
	}
}

func normalizeAttachmentItem(item interface{}) (Attachment, error) {
	switch v := item.(type) {
	case string:
		return Attachment{Source: strings.TrimSpace(v)}, nil
	case map[string]interface{}:
		att := Attachment{}
		if source := firstString(v, "source", "path", "file", "filepath", "url"); source != "" {
			att.Source = source
		}
		if name := firstString(v, "name", "filename", "label"); name != "" {
			att.Name = name
		}
		if mime := firstString(v, "content_type", "mimetype", "mime"); mime != "" {
			att.MIMEType = mime
		}
		if att.Source == "" {
			return att, errors.New("attachment entry missing source")
		}
		return att, nil
	default:
		return Attachment{}, fmt.Errorf("unsupported attachment format %T", item)
	}
}

func firstString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if raw, ok := values[key]; ok {
			switch v := raw.(type) {
			case string:
				if trimmed := strings.TrimSpace(v); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}

// ---------- misc helpers ----------

func splitAddress(value string) (string, string) {
	if strings.TrimSpace(value) == "" {
		return "", ""
	}
	addr, err := mail.ParseAddress(value)
	if err != nil {
		return "", strings.TrimSpace(value)
	}
	return addr.Name, addr.Address
}

func looksLikeHTML(body string) bool {
	body = strings.TrimSpace(body)
	return strings.HasPrefix(body, "<") && strings.Contains(body, ">")
}

func looksLikeURL(value string) bool {
	lower := strings.ToLower(value)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func randomBoundary(prefix string) string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(buf))
}

func (cfg *EmailConfig) TransportDetails() string {
	if cfg.Transport == "http" {
		return cfg.Endpoint
	}
	return fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
}

func (cfg *EmailConfig) ProviderOrHost() string {
	if cfg.Provider != "" {
		return cfg.Provider
	}
	return cfg.Host
}
