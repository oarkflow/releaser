package config

// AppBundle represents macOS App Bundle configuration
type AppBundle struct {
	ID             string                 `yaml:"id,omitempty"`
	Build          string                 `yaml:"build,omitempty"`
	Name           string                 `yaml:"name,omitempty"`
	DisplayName    string                 `yaml:"display_name,omitempty"`
	Identifier     string                 `yaml:"identifier,omitempty"`
	Icon           string                 `yaml:"icon,omitempty"`
	Version        string                 `yaml:"version,omitempty"`
	ShortVersion   string                 `yaml:"short_version,omitempty"`
	Copyright      string                 `yaml:"copyright,omitempty"`
	Category       string                 `yaml:"category,omitempty"`
	Sign           AppBundleSign          `yaml:"sign,omitempty"`
	InfoPlist      map[string]interface{} `yaml:"info_plist,omitempty"`
	Entitlements   map[string]interface{} `yaml:"entitlements,omitempty"`
	ExtraFiles     []AppBundleFile        `yaml:"extra_files,omitempty"`
	HighResolution bool                   `yaml:"high_resolution,omitempty"`
}

// AppBundleSign for code signing configuration
type AppBundleSign struct {
	Identity         string   `yaml:"identity,omitempty"`
	Keychain         string   `yaml:"keychain,omitempty"`
	KeychainPassword string   `yaml:"keychain_password,omitempty"`
	Entitlements     string   `yaml:"entitlements,omitempty"`
	Hardened         bool     `yaml:"hardened,omitempty"`
	Timestamp        bool     `yaml:"timestamp,omitempty"`
	Options          []string `yaml:"options,omitempty"`
}

// AppBundleFile for extra files in app bundle
type AppBundleFile struct {
	Src string `yaml:"src"`
	Dst string `yaml:"dst,omitempty"`
}

// DMG represents macOS DMG configuration
type DMG struct {
	ID                  string          `yaml:"id,omitempty"`
	AppBundle           string          `yaml:"app_bundle,omitempty"`
	Name                string          `yaml:"name,omitempty"`
	NameTemplate        string          `yaml:"name_template,omitempty"`
	Format              string          `yaml:"format,omitempty"`
	Filesystem          string          `yaml:"filesystem,omitempty"`
	Icon                string          `yaml:"icon,omitempty"`
	Background          string          `yaml:"background,omitempty"`
	WindowWidth         int             `yaml:"window_width,omitempty"`
	WindowHeight        int             `yaml:"window_height,omitempty"`
	IconSize            int             `yaml:"icon_size,omitempty"`
	TextSize            int             `yaml:"text_size,omitempty"`
	IconPosition        DMGIconPosition `yaml:"icon_position,omitempty"`
	ApplicationsSymlink bool            `yaml:"applications_symlink,omitempty"`
	Contents            []DMGContent    `yaml:"contents,omitempty"`
	Notarize            DMGNotarize     `yaml:"notarize,omitempty"`
	CodeSign            DMGCodeSign     `yaml:"code_sign,omitempty"`
}

// DMGIconPosition for icon placement
type DMGIconPosition struct {
	AppX          int `yaml:"app_x,omitempty"`
	AppY          int `yaml:"app_y,omitempty"`
	ApplicationsX int `yaml:"applications_x,omitempty"`
	ApplicationsY int `yaml:"applications_y,omitempty"`
}

// DMGContent for additional DMG content
type DMGContent struct {
	Src  string `yaml:"src"`
	X    int    `yaml:"x,omitempty"`
	Y    int    `yaml:"y,omitempty"`
	Type string `yaml:"type,omitempty"`
}

// DMGNotarize for macOS notarization
type DMGNotarize struct {
	Enabled  bool   `yaml:"enabled,omitempty"`
	AppleID  string `yaml:"apple_id,omitempty"`
	Password string `yaml:"password,omitempty"`
	TeamID   string `yaml:"team_id,omitempty"`
	BundleID string `yaml:"bundle_id,omitempty"`
	Timeout  string `yaml:"timeout,omitempty"`
	Staple   bool   `yaml:"staple,omitempty"`
}

// DMGCodeSign for DMG signing
type DMGCodeSign struct {
	Identity string `yaml:"identity,omitempty"`
	Keychain string `yaml:"keychain,omitempty"`
}

// MSI represents Windows MSI installer configuration
type MSI struct {
	ID             string        `yaml:"id,omitempty"`
	Build          string        `yaml:"build,omitempty"`
	Name           string        `yaml:"name,omitempty"`
	NameTemplate   string        `yaml:"name_template,omitempty"`
	WXS            string        `yaml:"wxs,omitempty"`
	ProductName    string        `yaml:"product_name,omitempty"`
	ProductVersion string        `yaml:"product_version,omitempty"`
	Manufacturer   string        `yaml:"manufacturer,omitempty"`
	UpgradeCode    string        `yaml:"upgrade_code,omitempty"`
	Icon           string        `yaml:"icon,omitempty"`
	License        string        `yaml:"license,omitempty"`
	Shortcuts      []MSIShortcut `yaml:"shortcuts,omitempty"`
	InstallDir     string        `yaml:"install_dir,omitempty"`
	ExtraFiles     []MSIFile     `yaml:"extra_files,omitempty"`
	Sign           MSISign       `yaml:"sign,omitempty"`
}

// MSIShortcut for desktop/start menu shortcuts
type MSIShortcut struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Target      string `yaml:"target"`
	Arguments   string `yaml:"arguments,omitempty"`
	Desktop     bool   `yaml:"desktop,omitempty"`
	StartMenu   bool   `yaml:"start_menu,omitempty"`
}

// MSIFile for additional MSI files
type MSIFile struct {
	Src string `yaml:"src"`
	Dst string `yaml:"dst,omitempty"`
}

// MSISign for MSI signing
type MSISign struct {
	Certificate         string `yaml:"certificate,omitempty"`
	CertificatePassword string `yaml:"certificate_password,omitempty"`
	TimestampServer     string `yaml:"timestamp_server,omitempty"`
}

// NSIS represents Windows NSIS installer configuration
type NSIS struct {
	ID           string            `yaml:"id,omitempty"`
	Build        string            `yaml:"build,omitempty"`
	Name         string            `yaml:"name,omitempty"`
	NameTemplate string            `yaml:"name_template,omitempty"`
	Script       string            `yaml:"script,omitempty"`
	Installer    string            `yaml:"installer,omitempty"`
	OutFile      string            `yaml:"out_file,omitempty"`
	Defines      map[string]string `yaml:"defines,omitempty"`
	ExtraFiles   []NSISFile        `yaml:"extra_files,omitempty"`
	Sign         NSISSign          `yaml:"sign,omitempty"`
}

// NSISFile for NSIS files
type NSISFile struct {
	Src string `yaml:"src"`
	Dst string `yaml:"dst,omitempty"`
}

// NSISSign for NSIS signing
type NSISSign struct {
	Certificate         string `yaml:"certificate,omitempty"`
	CertificatePassword string `yaml:"certificate_password,omitempty"`
	TimestampServer     string `yaml:"timestamp_server,omitempty"`
}

// Sign represents artifact signing configuration
type Sign struct {
	ID          string   `yaml:"id,omitempty"`
	Cmd         string   `yaml:"cmd,omitempty"`
	Args        []string `yaml:"args,omitempty"`
	Signature   string   `yaml:"signature,omitempty"`
	Artifacts   string   `yaml:"artifacts,omitempty"`
	IDs         []string `yaml:"ids,omitempty"`
	Stdin       string   `yaml:"stdin,omitempty"`
	StdinFile   string   `yaml:"stdin_file,omitempty"`
	Env         []string `yaml:"env,omitempty"`
	Certificate string   `yaml:"certificate,omitempty"`
	Output      bool     `yaml:"output,omitempty"`
}

// DockerSign represents Docker image signing
type DockerSign struct {
	ID        string   `yaml:"id,omitempty"`
	Artifacts string   `yaml:"artifacts,omitempty"`
	Images    []string `yaml:"images,omitempty"`
	Cmd       string   `yaml:"cmd,omitempty"`
	Args      []string `yaml:"args,omitempty"`
	Env       []string `yaml:"env,omitempty"`
}

// Checksum represents checksum configuration
type Checksum struct {
	NameTemplate string      `yaml:"name_template,omitempty"`
	Algorithm    string      `yaml:"algorithm,omitempty"`
	IDs          []string    `yaml:"ids,omitempty"`
	Disable      bool        `yaml:"disable,omitempty"`
	ExtraFiles   []ExtraFile `yaml:"extra_files,omitempty"`
	Split        bool        `yaml:"split,omitempty"`
}

// ExtraFile for additional files
type ExtraFile struct {
	Glob string `yaml:"glob"`
}

// Changelog represents changelog configuration
type Changelog struct {
	Use     string           `yaml:"use,omitempty"`
	Sort    string           `yaml:"sort,omitempty"`
	Abbrev  int              `yaml:"abbrev,omitempty"`
	Filters ChangelogFilters `yaml:"filters,omitempty"`
	Groups  []ChangelogGroup `yaml:"groups,omitempty"`
	Divider string           `yaml:"divider,omitempty"`
	AI      ChangelogAI      `yaml:"ai,omitempty"`
}

// ChangelogFilters for filtering commits
type ChangelogFilters struct {
	Exclude []string `yaml:"exclude,omitempty"`
	Include []string `yaml:"include,omitempty"`
}

// ChangelogGroup for grouping commits
type ChangelogGroup struct {
	Title  string `yaml:"title"`
	Regexp string `yaml:"regexp,omitempty"`
	Order  int    `yaml:"order,omitempty"`
}

// ChangelogAI for AI-enhanced changelogs
type ChangelogAI struct {
	Enabled  bool   `yaml:"enabled,omitempty"`
	Provider string `yaml:"provider,omitempty"`
	Model    string `yaml:"model,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
	Prompt   string `yaml:"prompt,omitempty"`
}

// Release represents release configuration
type Release struct {
	GitHub                   ReleaseRepo `yaml:"github,omitempty"`
	GitLab                   ReleaseRepo `yaml:"gitlab,omitempty"`
	Gitea                    ReleaseRepo `yaml:"gitea,omitempty"`
	Draft                    bool        `yaml:"draft,omitempty"`
	Prerelease               string      `yaml:"prerelease,omitempty"`
	NameTemplate             string      `yaml:"name_template,omitempty"`
	ReplaceExisting          bool        `yaml:"replace_existing,omitempty"`
	ReplaceExistingDraft     bool        `yaml:"replace_existing_draft,omitempty"`
	ReplaceExistingArtifacts bool        `yaml:"replace_existing_artifacts,omitempty"`
	TargetCommitish          string      `yaml:"target_commitish,omitempty"`
	Mode                     string      `yaml:"mode,omitempty"`
	Header                   string      `yaml:"header,omitempty"`
	Footer                   string      `yaml:"footer,omitempty"`
	ExtraFiles               []ExtraFile `yaml:"extra_files,omitempty"`
	IDs                      []string    `yaml:"ids,omitempty"`
	SkipUpload               bool        `yaml:"skip_upload,omitempty"`
	MakeLatest               string      `yaml:"make_latest,omitempty"`
}

// ReleaseRepo for release repository configuration
type ReleaseRepo struct {
	Owner string `yaml:"owner,omitempty"`
	Name  string `yaml:"name,omitempty"`
}

// Announce represents announcement configuration
type Announce struct {
	Skip       string             `yaml:"skip,omitempty"`
	Slack      AnnounceSlack      `yaml:"slack,omitempty"`
	Discord    AnnounceDiscord    `yaml:"discord,omitempty"`
	Twitter    AnnounceTwitter    `yaml:"twitter,omitempty"`
	Mastodon   AnnounceMastodon   `yaml:"mastodon,omitempty"`
	Reddit     AnnounceReddit     `yaml:"reddit,omitempty"`
	Teams      AnnounceTeams      `yaml:"teams,omitempty"`
	Telegram   AnnounceTelegram   `yaml:"telegram,omitempty"`
	Webhook    AnnounceWebhook    `yaml:"webhook,omitempty"`
	SMTP       AnnounceSMTP       `yaml:"smtp,omitempty"`
	Mattermost AnnounceMattermost `yaml:"mattermost,omitempty"`
	LinkedIn   AnnounceLinkedIn   `yaml:"linkedin,omitempty"`
	Bluesky    AnnounceBluesky    `yaml:"bluesky,omitempty"`
}

// AnnounceSlack for Slack announcements
type AnnounceSlack struct {
	Enabled         bool          `yaml:"enabled,omitempty"`
	Channel         string        `yaml:"channel,omitempty"`
	Username        string        `yaml:"username,omitempty"`
	IconEmoji       string        `yaml:"icon_emoji,omitempty"`
	IconURL         string        `yaml:"icon_url,omitempty"`
	MessageTemplate string        `yaml:"message_template,omitempty"`
	Blocks          []interface{} `yaml:"blocks,omitempty"`
	Attachments     []interface{} `yaml:"attachments,omitempty"`
}

// AnnounceDiscord for Discord announcements
type AnnounceDiscord struct {
	Enabled         bool   `yaml:"enabled,omitempty"`
	MessageTemplate string `yaml:"message_template,omitempty"`
	Author          string `yaml:"author,omitempty"`
	Color           string `yaml:"color,omitempty"`
	IconURL         string `yaml:"icon_url,omitempty"`
}

// AnnounceTwitter for Twitter announcements
type AnnounceTwitter struct {
	Enabled         bool   `yaml:"enabled,omitempty"`
	MessageTemplate string `yaml:"message_template,omitempty"`
}

// AnnounceMastodon for Mastodon announcements
type AnnounceMastodon struct {
	Enabled         bool   `yaml:"enabled,omitempty"`
	Server          string `yaml:"server,omitempty"`
	MessageTemplate string `yaml:"message_template,omitempty"`
}

// AnnounceReddit for Reddit announcements
type AnnounceReddit struct {
	Enabled       bool   `yaml:"enabled,omitempty"`
	ApplicationID string `yaml:"application_id,omitempty"`
	Username      string `yaml:"username,omitempty"`
	TitleTemplate string `yaml:"title_template,omitempty"`
	URLTemplate   string `yaml:"url_template,omitempty"`
	Sub           string `yaml:"sub,omitempty"`
}

// AnnounceTeams for Microsoft Teams announcements
type AnnounceTeams struct {
	Enabled         bool   `yaml:"enabled,omitempty"`
	TitleTemplate   string `yaml:"title_template,omitempty"`
	MessageTemplate string `yaml:"message_template,omitempty"`
	Color           string `yaml:"color,omitempty"`
	IconURL         string `yaml:"icon_url,omitempty"`
}

// AnnounceTelegram for Telegram announcements
type AnnounceTelegram struct {
	Enabled         bool   `yaml:"enabled,omitempty"`
	ChatID          string `yaml:"chat_id,omitempty"`
	MessageTemplate string `yaml:"message_template,omitempty"`
	ParseMode       string `yaml:"parse_mode,omitempty"`
}

// AnnounceWebhook for generic webhook announcements
type AnnounceWebhook struct {
	Enabled         bool              `yaml:"enabled,omitempty"`
	EndpointURL     string            `yaml:"endpoint_url,omitempty"`
	MessageTemplate string            `yaml:"message_template,omitempty"`
	Headers         map[string]string `yaml:"headers,omitempty"`
	ContentType     string            `yaml:"content_type,omitempty"`
	SkipTLSVerify   bool              `yaml:"skip_tls_verify,omitempty"`
}

// AnnounceSMTP for email announcements
type AnnounceSMTP struct {
	Enabled            bool     `yaml:"enabled,omitempty"`
	Host               string   `yaml:"host,omitempty"`
	Port               int      `yaml:"port,omitempty"`
	Username           string   `yaml:"username,omitempty"`
	Password           string   `yaml:"password,omitempty"`
	From               string   `yaml:"from,omitempty"`
	To                 []string `yaml:"to,omitempty"`
	SubjectTemplate    string   `yaml:"subject_template,omitempty"`
	BodyTemplate       string   `yaml:"body_template,omitempty"`
	InsecureSkipVerify bool     `yaml:"insecure_skip_verify,omitempty"`
}

// AnnounceMattermost for Mattermost announcements
type AnnounceMattermost struct {
	Enabled         bool   `yaml:"enabled,omitempty"`
	MessageTemplate string `yaml:"message_template,omitempty"`
	TitleTemplate   string `yaml:"title_template,omitempty"`
	Color           string `yaml:"color,omitempty"`
	Channel         string `yaml:"channel,omitempty"`
	Username        string `yaml:"username,omitempty"`
	IconEmoji       string `yaml:"icon_emoji,omitempty"`
	IconURL         string `yaml:"icon_url,omitempty"`
}

// AnnounceLinkedIn for LinkedIn announcements
type AnnounceLinkedIn struct {
	Enabled         bool   `yaml:"enabled,omitempty"`
	MessageTemplate string `yaml:"message_template,omitempty"`
}

// AnnounceBluesky for Bluesky announcements
type AnnounceBluesky struct {
	Enabled         bool   `yaml:"enabled,omitempty"`
	MessageTemplate string `yaml:"message_template,omitempty"`
}
