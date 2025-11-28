package config

import "os"

// NFPM represents Linux package configuration
type NFPM struct {
	ID               string                  `yaml:"id,omitempty"`
	Builds           []string                `yaml:"builds,omitempty"`
	Formats          []string                `yaml:"formats,omitempty"`
	Vendor           string                  `yaml:"vendor,omitempty"`
	Homepage         string                  `yaml:"homepage,omitempty"`
	Maintainer       string                  `yaml:"maintainer,omitempty"`
	Description      string                  `yaml:"description,omitempty"`
	License          string                  `yaml:"license,omitempty"`
	Section          string                  `yaml:"section,omitempty"`
	Priority         string                  `yaml:"priority,omitempty"`
	Conflicts        []string                `yaml:"conflicts,omitempty"`
	Depends          []string                `yaml:"depends,omitempty"`
	Recommends       []string                `yaml:"recommends,omitempty"`
	Suggests         []string                `yaml:"suggests,omitempty"`
	Replaces         []string                `yaml:"replaces,omitempty"`
	Provides         []string                `yaml:"provides,omitempty"`
	Contents         []NFPMContent           `yaml:"contents,omitempty"`
	Scripts          NFPMScripts             `yaml:"scripts,omitempty"`
	Overrides        map[string]NFPMOverride `yaml:"overrides,omitempty"`
	Meta             bool                    `yaml:"meta,omitempty"`
	FileNameTemplate string                  `yaml:"file_name_template,omitempty"`
	Bindir           string                  `yaml:"bindir,omitempty"`
	Epoch            string                  `yaml:"epoch,omitempty"`
	Release          string                  `yaml:"release,omitempty"`
	Prerelease       string                  `yaml:"prerelease,omitempty"`
	VersionMetadata  string                  `yaml:"version_metadata,omitempty"`
	Deb              NFPMDeb                 `yaml:"deb,omitempty"`
	RPM              NFPMRPM                 `yaml:"rpm,omitempty"`
	APK              NFPMAPK                 `yaml:"apk,omitempty"`
	Archlinux        NFPMArchlinux           `yaml:"archlinux,omitempty"`
	Skip             string                  `yaml:"skip,omitempty"`
	PackageName      string                  `yaml:"package_name,omitempty"`
	Dependencies     []string                `yaml:"dependencies,omitempty"`
}

// NFPMContent represents file contents for packages
type NFPMContent struct {
	Src      string          `yaml:"src,omitempty"`
	Dst      string          `yaml:"dst"`
	Type     string          `yaml:"type,omitempty"`
	Packager string          `yaml:"packager,omitempty"`
	FileInfo NFPMContentInfo `yaml:"file_info,omitempty"`
	Expand   bool            `yaml:"expand,omitempty"`
}

// NFPMContentInfo represents file metadata
type NFPMContentInfo struct {
	Owner string      `yaml:"owner,omitempty"`
	Group string      `yaml:"group,omitempty"`
	Mode  os.FileMode `yaml:"mode,omitempty"`
	MTime string      `yaml:"mtime,omitempty"`
}

// NFPMScripts for package scripts
type NFPMScripts struct {
	PreInstall  string `yaml:"preinstall,omitempty"`
	PostInstall string `yaml:"postinstall,omitempty"`
	PreRemove   string `yaml:"preremove,omitempty"`
	PostRemove  string `yaml:"postremove,omitempty"`
}

// NFPMOverride for format-specific overrides
type NFPMOverride struct {
	Depends    []string      `yaml:"depends,omitempty"`
	Recommends []string      `yaml:"recommends,omitempty"`
	Suggests   []string      `yaml:"suggests,omitempty"`
	Replaces   []string      `yaml:"replaces,omitempty"`
	Conflicts  []string      `yaml:"conflicts,omitempty"`
	Contents   []NFPMContent `yaml:"contents,omitempty"`
	Scripts    NFPMScripts   `yaml:"scripts,omitempty"`
}

// NFPMDeb for Debian-specific options
type NFPMDeb struct {
	Compression string            `yaml:"compression,omitempty"`
	Signature   NFPMDebSignature  `yaml:"signature,omitempty"`
	Scripts     NFPMDebScripts    `yaml:"scripts,omitempty"`
	Triggers    NFPMDebTriggers   `yaml:"triggers,omitempty"`
	Breaks      []string          `yaml:"breaks,omitempty"`
	Predepends  []string          `yaml:"predepends,omitempty"`
	Fields      map[string]string `yaml:"fields,omitempty"`
}

// NFPMDebSignature for Debian package signing
type NFPMDebSignature struct {
	KeyFile       string `yaml:"key_file,omitempty"`
	KeyID         string `yaml:"key_id,omitempty"`
	KeyPassphrase string `yaml:"key_passphrase,omitempty"`
	Type          string `yaml:"type,omitempty"`
}

// NFPMDebScripts for Debian-specific scripts
type NFPMDebScripts struct {
	Rules    string `yaml:"rules,omitempty"`
	Postinst string `yaml:"postinst,omitempty"`
}

// NFPMDebTriggers for Debian triggers
type NFPMDebTriggers struct {
	Interest        []string `yaml:"interest,omitempty"`
	InterestAwait   []string `yaml:"interest_await,omitempty"`
	InterestNoAwait []string `yaml:"interest_noawait,omitempty"`
	Activate        []string `yaml:"activate,omitempty"`
	ActivateAwait   []string `yaml:"activate_await,omitempty"`
	ActivateNoAwait []string `yaml:"activate_noawait,omitempty"`
}

// NFPMRPM for RPM-specific options
type NFPMRPM struct {
	Summary     string           `yaml:"summary,omitempty"`
	Group       string           `yaml:"group,omitempty"`
	Compression string           `yaml:"compression,omitempty"`
	Signature   NFPMRPMSignature `yaml:"signature,omitempty"`
	Scripts     NFPMRPMScripts   `yaml:"scripts,omitempty"`
	Prefixes    []string         `yaml:"prefixes,omitempty"`
	Packager    string           `yaml:"packager,omitempty"`
}

// NFPMRPMSignature for RPM signing
type NFPMRPMSignature struct {
	KeyFile       string `yaml:"key_file,omitempty"`
	KeyID         string `yaml:"key_id,omitempty"`
	KeyPassphrase string `yaml:"key_passphrase,omitempty"`
}

// NFPMRPMScripts for RPM-specific scripts
type NFPMRPMScripts struct {
	Pretrans  string `yaml:"pretrans,omitempty"`
	Posttrans string `yaml:"posttrans,omitempty"`
}

// NFPMAPK for Alpine-specific options
type NFPMAPK struct {
	Signature NFPMAPKSignature `yaml:"signature,omitempty"`
	Scripts   NFPMAPKScripts   `yaml:"scripts,omitempty"`
}

// NFPMAPKSignature for APK signing
type NFPMAPKSignature struct {
	KeyFile       string `yaml:"key_file,omitempty"`
	KeyID         string `yaml:"key_id,omitempty"`
	KeyName       string `yaml:"key_name,omitempty"`
	KeyPassphrase string `yaml:"key_passphrase,omitempty"`
}

// NFPMAPKScripts for Alpine scripts
type NFPMAPKScripts struct {
	PreUpgrade  string `yaml:"preupgrade,omitempty"`
	PostUpgrade string `yaml:"postupgrade,omitempty"`
}

// NFPMArchlinux for Archlinux-specific options
type NFPMArchlinux struct {
	Pkgbase  string               `yaml:"pkgbase,omitempty"`
	Packager string               `yaml:"packager,omitempty"`
	Scripts  NFPMArchlinuxScripts `yaml:"scripts,omitempty"`
}

// NFPMArchlinuxScripts for Archlinux scripts
type NFPMArchlinuxScripts struct {
	PreUpgrade  string `yaml:"preupgrade,omitempty"`
	PostUpgrade string `yaml:"postupgrade,omitempty"`
}

// Snapcraft represents Snapcraft configuration
type Snapcraft struct {
	ID               string                     `yaml:"id,omitempty"`
	Builds           []string                   `yaml:"builds,omitempty"`
	Name             string                     `yaml:"name,omitempty"`
	Title            string                     `yaml:"title,omitempty"`
	Publish          bool                       `yaml:"publish,omitempty"`
	Summary          string                     `yaml:"summary,omitempty"`
	Description      string                     `yaml:"description,omitempty"`
	License          string                     `yaml:"license,omitempty"`
	Grade            string                     `yaml:"grade,omitempty"`
	Confinement      string                     `yaml:"confinement,omitempty"`
	Base             string                     `yaml:"base,omitempty"`
	ChannelTemplates []string                   `yaml:"channel_templates,omitempty"`
	Apps             map[string]SnapcraftApp    `yaml:"apps,omitempty"`
	Plugs            []string                   `yaml:"plugs,omitempty"`
	ExtraFiles       []SnapcraftExtraFile       `yaml:"extra_files,omitempty"`
	Layout           map[string]SnapcraftLayout `yaml:"layout,omitempty"`
	Skip             string                     `yaml:"skip,omitempty"`
}

// SnapcraftApp represents a Snap application
type SnapcraftApp struct {
	Command   string   `yaml:"command"`
	Plugs     []string `yaml:"plugs,omitempty"`
	Daemon    string   `yaml:"daemon,omitempty"`
	Completer string   `yaml:"completer,omitempty"`
	Args      string   `yaml:"args,omitempty"`
}

// SnapcraftExtraFile for additional files
type SnapcraftExtraFile struct {
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
	Mode        uint32 `yaml:"mode,omitempty"`
}

// SnapcraftLayout for snap layouts
type SnapcraftLayout struct {
	Symlink string `yaml:"symlink,omitempty"`
	Bind    string `yaml:"bind,omitempty"`
}

// Docker represents Docker image configuration
type Docker struct {
	ID                 string   `yaml:"id,omitempty"`
	IDs                []string `yaml:"ids,omitempty"`
	Goos               string   `yaml:"goos,omitempty"`
	Goarch             string   `yaml:"goarch,omitempty"`
	Goarm              string   `yaml:"goarm,omitempty"`
	Goamd64            string   `yaml:"goamd64,omitempty"`
	Dockerfile         string   `yaml:"dockerfile,omitempty"`
	Use                string   `yaml:"use,omitempty"`
	ImageTemplates     []string `yaml:"image_templates,omitempty"`
	SkipPush           string   `yaml:"skip_push,omitempty"`
	BuildFlagTemplates []string `yaml:"build_flag_templates,omitempty"`
	PushFlags          []string `yaml:"push_flags,omitempty"`
	ExtraFiles         []string `yaml:"extra_files,omitempty"`
	BuildArgs          []string `yaml:"build_args,omitempty"`
	Skip               string   `yaml:"skip,omitempty"`
	SkipBuild          bool     `yaml:"skip_build,omitempty"`
	Buildx             bool     `yaml:"buildx,omitempty"`
	BuildxPlatforms    []string `yaml:"buildx_platforms,omitempty"`
	Push               bool     `yaml:"push,omitempty"`
}

// DockerManifest represents Docker manifest configuration
type DockerManifest struct {
	ID             string   `yaml:"id,omitempty"`
	NameTemplate   string   `yaml:"name_template,omitempty"`
	ImageTemplates []string `yaml:"image_templates,omitempty"`
	SkipPush       string   `yaml:"skip_push,omitempty"`
	Use            string   `yaml:"use,omitempty"`
	CreateFlags    []string `yaml:"create_flags,omitempty"`
	PushFlags      []string `yaml:"push_flags,omitempty"`
}

// Brew represents Homebrew configuration
type Brew struct {
	Name              string           `yaml:"name,omitempty"`
	Description       string           `yaml:"description,omitempty"`
	Homepage          string           `yaml:"homepage,omitempty"`
	License           string           `yaml:"license,omitempty"`
	SkipUpload        string           `yaml:"skip_upload,omitempty"`
	Caveats           string           `yaml:"caveats,omitempty"`
	Test              string           `yaml:"test,omitempty"`
	Install           string           `yaml:"install,omitempty"`
	PostInstall       string           `yaml:"post_install,omitempty"`
	Dependencies      []BrewDependency `yaml:"dependencies,omitempty"`
	Conflicts         []string         `yaml:"conflicts,omitempty"`
	Service           BrewService      `yaml:"service,omitempty"`
	ExtraInstall      string           `yaml:"extra_install,omitempty"`
	Repository        RepoRef          `yaml:"repository,omitempty"`
	Tap               RepoRef          `yaml:"tap,omitempty"`
	URLTemplate       string           `yaml:"url_template,omitempty"`
	DownloadStrategy  string           `yaml:"download_strategy,omitempty"`
	Goarm             string           `yaml:"goarm,omitempty"`
	Goamd64           string           `yaml:"goamd64,omitempty"`
	IDs               []string         `yaml:"ids,omitempty"`
	CommitAuthor      CommitAuthor     `yaml:"commit_author,omitempty"`
	CommitMsgTemplate string           `yaml:"commit_msg_template,omitempty"`
	Directory         string           `yaml:"directory,omitempty"`
}

// BrewDependency for Homebrew dependencies
type BrewDependency struct {
	Name string `yaml:"name"`
	Type string `yaml:"type,omitempty"`
	OS   string `yaml:"os,omitempty"`
}

// BrewService for Homebrew service configuration
type BrewService struct {
	Run                  []string          `yaml:"run,omitempty"`
	RunType              string            `yaml:"run_type,omitempty"`
	EnvironmentVariables map[string]string `yaml:"environment_variables,omitempty"`
	KeepAlive            bool              `yaml:"keep_alive,omitempty"`
	WorkingDir           string            `yaml:"working_dir,omitempty"`
}

// Scoop represents Scoop bucket configuration
type Scoop struct {
	Name              string       `yaml:"name,omitempty"`
	Description       string       `yaml:"description,omitempty"`
	Homepage          string       `yaml:"homepage,omitempty"`
	License           string       `yaml:"license,omitempty"`
	SkipUpload        string       `yaml:"skip_upload,omitempty"`
	URLTemplate       string       `yaml:"url_template,omitempty"`
	Repository        RepoRef      `yaml:"repository,omitempty"`
	Bucket            RepoRef      `yaml:"bucket,omitempty"`
	Persist           []string     `yaml:"persist,omitempty"`
	PreInstall        []string     `yaml:"pre_install,omitempty"`
	PostInstall       []string     `yaml:"post_install,omitempty"`
	Depends           []string     `yaml:"depends,omitempty"`
	Shortcuts         [][]string   `yaml:"shortcuts,omitempty"`
	IDs               []string     `yaml:"ids,omitempty"`
	Goarm             string       `yaml:"goarm,omitempty"`
	Goamd64           string       `yaml:"goamd64,omitempty"`
	CommitAuthor      CommitAuthor `yaml:"commit_author,omitempty"`
	CommitMsgTemplate string       `yaml:"commit_msg_template,omitempty"`
	Directory         string       `yaml:"directory,omitempty"`
}

// RepoRef represents a repository reference
type RepoRef struct {
	Owner       string            `yaml:"owner,omitempty"`
	Name        string            `yaml:"name,omitempty"`
	Token       string            `yaml:"token,omitempty"`
	Branch      string            `yaml:"branch,omitempty"`
	Git         GitRepoRef        `yaml:"git,omitempty"`
	PullRequest PullRequestConfig `yaml:"pull_request,omitempty"`
}

// GitRepoRef for Git repository references
type GitRepoRef struct {
	URL        string `yaml:"url,omitempty"`
	SSHCommand string `yaml:"ssh_command,omitempty"`
	PrivateKey string `yaml:"private_key,omitempty"`
}

// PullRequestConfig for PR creation
type PullRequestConfig struct {
	Enabled bool   `yaml:"enabled,omitempty"`
	Base    string `yaml:"base,omitempty"`
	Draft   bool   `yaml:"draft,omitempty"`
}

// CommitAuthor for commit authorship
type CommitAuthor struct {
	Name  string `yaml:"name,omitempty"`
	Email string `yaml:"email,omitempty"`
}

// NPM represents NPM package configuration
type NPM struct {
	Name             string                 `yaml:"name,omitempty"`
	Description      string                 `yaml:"description,omitempty"`
	Homepage         string                 `yaml:"homepage,omitempty"`
	License          string                 `yaml:"license,omitempty"`
	SkipUpload       string                 `yaml:"skip_upload,omitempty"`
	Registry         string                 `yaml:"registry,omitempty"`
	Scope            string                 `yaml:"scope,omitempty"`
	Access           string                 `yaml:"access,omitempty"`
	Token            string                 `yaml:"token,omitempty"`
	Tag              string                 `yaml:"tag,omitempty"`
	IDs              []string               `yaml:"ids,omitempty"`
	Dependencies     map[string]string      `yaml:"dependencies,omitempty"`
	DevDependencies  map[string]string      `yaml:"dev_dependencies,omitempty"`
	PeerDependencies map[string]string      `yaml:"peer_dependencies,omitempty"`
	Keywords         []string               `yaml:"keywords,omitempty"`
	Scripts          map[string]string      `yaml:"scripts,omitempty"`
	Bin              map[string]string      `yaml:"bin,omitempty"`
	Files            []string               `yaml:"files,omitempty"`
	ExtraFields      map[string]interface{} `yaml:"extra_fields,omitempty"`
}

// Chocolatey represents Chocolatey package configuration
type Chocolatey struct {
	Name                     string                 `yaml:"name,omitempty"`
	Title                    string                 `yaml:"title,omitempty"`
	Description              string                 `yaml:"description,omitempty"`
	Summary                  string                 `yaml:"summary,omitempty"`
	Authors                  string                 `yaml:"authors,omitempty"`
	ProjectURL               string                 `yaml:"project_url,omitempty"`
	IconURL                  string                 `yaml:"icon_url,omitempty"`
	Copyright                string                 `yaml:"copyright,omitempty"`
	LicenseURL               string                 `yaml:"license_url,omitempty"`
	RequireLicenseAcceptance bool                   `yaml:"require_license_acceptance,omitempty"`
	ProjectSourceURL         string                 `yaml:"project_source_url,omitempty"`
	DocsURL                  string                 `yaml:"docs_url,omitempty"`
	Tags                     string                 `yaml:"tags,omitempty"`
	BugTrackerURL            string                 `yaml:"bug_tracker_url,omitempty"`
	SkipPublish              string                 `yaml:"skip_publish,omitempty"`
	URLTemplate              string                 `yaml:"url_template,omitempty"`
	SourceRepo               string                 `yaml:"source_repo,omitempty"`
	APIKey                   string                 `yaml:"api_key,omitempty"`
	IDs                      []string               `yaml:"ids,omitempty"`
	Goarm                    string                 `yaml:"goarm,omitempty"`
	Goamd64                  string                 `yaml:"goamd64,omitempty"`
	Dependencies             []ChocolateyDependency `yaml:"dependencies,omitempty"`
}

// ChocolateyDependency for Chocolatey dependencies
type ChocolateyDependency struct {
	ID      string `yaml:"id"`
	Version string `yaml:"version,omitempty"`
}
