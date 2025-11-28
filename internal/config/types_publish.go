package config

// Blob represents cloud storage configuration
type Blob struct {
	ID                 string      `yaml:"id,omitempty"`
	Provider           string      `yaml:"provider,omitempty"`
	Bucket             string      `yaml:"bucket,omitempty"`
	Region             string      `yaml:"region,omitempty"`
	Directory          string      `yaml:"directory,omitempty"`
	IDs                []string    `yaml:"ids,omitempty"`
	ExtraFiles         []ExtraFile `yaml:"extra_files,omitempty"`
	Endpoint           string      `yaml:"endpoint,omitempty"`
	DisableSSL         bool        `yaml:"disable_ssl,omitempty"`
	ACL                string      `yaml:"acl,omitempty"`
	CacheControl       []string    `yaml:"cache_control,omitempty"`
	ContentDisposition string      `yaml:"content_disposition,omitempty"`
}

// Upload represents custom HTTP upload configuration
type Upload struct {
	ID                 string            `yaml:"id,omitempty"`
	Name               string            `yaml:"name,omitempty"`
	Target             string            `yaml:"target,omitempty"`
	Username           string            `yaml:"username,omitempty"`
	Mode               string            `yaml:"mode,omitempty"`
	Method             string            `yaml:"method,omitempty"`
	ChecksumHeader     string            `yaml:"checksum_header,omitempty"`
	TrustedCerts       string            `yaml:"trusted_certs,omitempty"`
	ClientCert         string            `yaml:"client_cert,omitempty"`
	ClientKey          string            `yaml:"client_key,omitempty"`
	IDs                []string          `yaml:"ids,omitempty"`
	ExtraFiles         []ExtraFile       `yaml:"extra_files,omitempty"`
	CustomArtifactName bool              `yaml:"custom_artifact_name,omitempty"`
	CustomHeaders      map[string]string `yaml:"custom_headers,omitempty"`
}

// Publisher represents custom publisher configuration
type Publisher struct {
	Name       string      `yaml:"name,omitempty"`
	IDs        []string    `yaml:"ids,omitempty"`
	Checksum   bool        `yaml:"checksum,omitempty"`
	Signature  bool        `yaml:"signature,omitempty"`
	Dir        string      `yaml:"dir,omitempty"`
	Cmd        string      `yaml:"cmd,omitempty"`
	Env        []string    `yaml:"env,omitempty"`
	ExtraFiles []ExtraFile `yaml:"extra_files,omitempty"`
	Disable    string      `yaml:"disable,omitempty"`
}

// Source represents source archive configuration
type Source struct {
	Enabled        bool         `yaml:"enabled,omitempty"`
	NameTemplate   string       `yaml:"name_template,omitempty"`
	Format         string       `yaml:"format,omitempty"`
	PrefixTemplate string       `yaml:"prefix_template,omitempty"`
	Files          []SourceFile `yaml:"files,omitempty"`
}

// SourceFile for source archive files
type SourceFile struct {
	Src         string `yaml:"src"`
	Dst         string `yaml:"dst,omitempty"`
	StripParent bool   `yaml:"strip_parent,omitempty"`
}

// SBOM represents Software Bill of Materials configuration
type SBOM struct {
	ID        string   `yaml:"id,omitempty"`
	Cmd       string   `yaml:"cmd,omitempty"`
	Args      []string `yaml:"args,omitempty"`
	Documents []string `yaml:"documents,omitempty"`
	Artifacts string   `yaml:"artifacts,omitempty"`
	IDs       []string `yaml:"ids,omitempty"`
	Env       []string `yaml:"env,omitempty"`
	Skip      string   `yaml:"skip,omitempty"`
	Method    string   `yaml:"method,omitempty"`
	Format    string   `yaml:"format,omitempty"`
	Target    string   `yaml:"target,omitempty"`
}

// Milestone represents milestone configuration
type Milestone struct {
	Repo         RepoRef `yaml:"repo,omitempty"`
	Close        bool    `yaml:"close,omitempty"`
	FailOnError  bool    `yaml:"fail_on_error,omitempty"`
	NameTemplate string  `yaml:"name_template,omitempty"`
}

// UniversalBinary represents macOS universal binary configuration
type UniversalBinary struct {
	ID           string     `yaml:"id,omitempty"`
	IDs          []string   `yaml:"ids,omitempty"`
	NameTemplate string     `yaml:"name_template,omitempty"`
	Replace      bool       `yaml:"replace,omitempty"`
	ModTimestamp string     `yaml:"mod_timestamp,omitempty"`
	Hooks        BuildHooks `yaml:"hooks,omitempty"`
}

// UPX represents UPX compression configuration
type UPX struct {
	ID               string   `yaml:"id,omitempty"`
	IDs              []string `yaml:"ids,omitempty"`
	Enabled          bool     `yaml:"enabled,omitempty"`
	Goos             []string `yaml:"goos,omitempty"`
	Goarch           []string `yaml:"goarch,omitempty"`
	Goarm            []string `yaml:"goarm,omitempty"`
	Goamd64          []string `yaml:"goamd64,omitempty"`
	Binary           string   `yaml:"binary,omitempty"`
	Compress         string   `yaml:"compress,omitempty"`
	LZMA             bool     `yaml:"lzma,omitempty"`
	Brute            bool     `yaml:"brute,omitempty"`
	Skip             string   `yaml:"skip,omitempty"`
	FailOnError      bool     `yaml:"fail_on_error,omitempty"`
	CompressionLevel int      `yaml:"compression_level,omitempty"`
	ExtraArgs        []string `yaml:"extra_args,omitempty"`
	OutputTemplate   string   `yaml:"output_template,omitempty"`
}

// Winget represents Windows Package Manager configuration
type Winget struct {
	Name                string       `yaml:"name,omitempty"`
	PackageIdentifier   string       `yaml:"package_identifier,omitempty"`
	Publisher           string       `yaml:"publisher,omitempty"`
	PublisherURL        string       `yaml:"publisher_url,omitempty"`
	PublisherSupportURL string       `yaml:"publisher_support_url,omitempty"`
	Copyright           string       `yaml:"copyright,omitempty"`
	CopyrightURL        string       `yaml:"copyright_url,omitempty"`
	ShortDescription    string       `yaml:"short_description,omitempty"`
	Description         string       `yaml:"description,omitempty"`
	Homepage            string       `yaml:"homepage,omitempty"`
	License             string       `yaml:"license,omitempty"`
	LicenseURL          string       `yaml:"license_url,omitempty"`
	ReleaseNotes        string       `yaml:"release_notes,omitempty"`
	ReleaseNotesURL     string       `yaml:"release_notes_url,omitempty"`
	Tags                []string     `yaml:"tags,omitempty"`
	SkipUpload          string       `yaml:"skip_upload,omitempty"`
	URLTemplate         string       `yaml:"url_template,omitempty"`
	Repository          RepoRef      `yaml:"repository,omitempty"`
	IDs                 []string     `yaml:"ids,omitempty"`
	Goarm               string       `yaml:"goarm,omitempty"`
	Goamd64             string       `yaml:"goamd64,omitempty"`
	CommitAuthor        CommitAuthor `yaml:"commit_author,omitempty"`
	CommitMsgTemplate   string       `yaml:"commit_msg_template,omitempty"`
	Path                string       `yaml:"path,omitempty"`
}

// AUR represents Arch User Repository configuration
type AUR struct {
	Name              string       `yaml:"name,omitempty"`
	Description       string       `yaml:"description,omitempty"`
	Homepage          string       `yaml:"homepage,omitempty"`
	License           string       `yaml:"license,omitempty"`
	Maintainers       []string     `yaml:"maintainers,omitempty"`
	Contributors      []string     `yaml:"contributors,omitempty"`
	PrivateKey        string       `yaml:"private_key,omitempty"`
	GitURL            string       `yaml:"git_url,omitempty"`
	GitSSHCommand     string       `yaml:"git_ssh_command,omitempty"`
	SkipUpload        string       `yaml:"skip_upload,omitempty"`
	URLTemplate       string       `yaml:"url_template,omitempty"`
	Depends           []string     `yaml:"depends,omitempty"`
	OptDepends        []string     `yaml:"optdepends,omitempty"`
	Conflicts         []string     `yaml:"conflicts,omitempty"`
	Provides          []string     `yaml:"provides,omitempty"`
	Replaces          []string     `yaml:"replaces,omitempty"`
	Package           string       `yaml:"package,omitempty"`
	IDs               []string     `yaml:"ids,omitempty"`
	Goarm             string       `yaml:"goarm,omitempty"`
	Goamd64           string       `yaml:"goamd64,omitempty"`
	CommitAuthor      CommitAuthor `yaml:"commit_author,omitempty"`
	CommitMsgTemplate string       `yaml:"commit_msg_template,omitempty"`
	Directory         string       `yaml:"directory,omitempty"`
}

// Krew represents kubectl krew plugin configuration
type Krew struct {
	Name              string       `yaml:"name,omitempty"`
	Description       string       `yaml:"description,omitempty"`
	ShortDescription  string       `yaml:"short_description,omitempty"`
	Homepage          string       `yaml:"homepage,omitempty"`
	Caveats           string       `yaml:"caveats,omitempty"`
	SkipUpload        string       `yaml:"skip_upload,omitempty"`
	URLTemplate       string       `yaml:"url_template,omitempty"`
	Repository        RepoRef      `yaml:"repository,omitempty"`
	IDs               []string     `yaml:"ids,omitempty"`
	Goarm             string       `yaml:"goarm,omitempty"`
	Goamd64           string       `yaml:"goamd64,omitempty"`
	CommitAuthor      CommitAuthor `yaml:"commit_author,omitempty"`
	CommitMsgTemplate string       `yaml:"commit_msg_template,omitempty"`
	Index             RepoRef      `yaml:"index,omitempty"`
}

// Ko represents Ko container image builder configuration
type Ko struct {
	ID            string            `yaml:"id,omitempty"`
	Build         string            `yaml:"build,omitempty"`
	Main          string            `yaml:"main,omitempty"`
	WorkingDir    string            `yaml:"working_dir,omitempty"`
	BaseImage     string            `yaml:"base_image,omitempty"`
	Repository    string            `yaml:"repository,omitempty"`
	Platforms     []string          `yaml:"platforms,omitempty"`
	Tags          []string          `yaml:"tags,omitempty"`
	SBOM          string            `yaml:"sbom,omitempty"`
	Bare          bool              `yaml:"bare,omitempty"`
	PreservePaths bool              `yaml:"preserve_import_paths,omitempty"`
	BaseInToto    bool              `yaml:"base_in_toto,omitempty"`
	Env           []string          `yaml:"env,omitempty"`
	Flags         []string          `yaml:"flags,omitempty"`
	Ldflags       []string          `yaml:"ldflags,omitempty"`
	Labels        map[string]string `yaml:"labels,omitempty"`
}

// Nix represents Nix package configuration
type Nix struct {
	Name              string       `yaml:"name,omitempty"`
	Description       string       `yaml:"description,omitempty"`
	Homepage          string       `yaml:"homepage,omitempty"`
	License           string       `yaml:"license,omitempty"`
	SkipUpload        string       `yaml:"skip_upload,omitempty"`
	URLTemplate       string       `yaml:"url_template,omitempty"`
	Repository        RepoRef      `yaml:"repository,omitempty"`
	IDs               []string     `yaml:"ids,omitempty"`
	Goarm             string       `yaml:"goarm,omitempty"`
	Goamd64           string       `yaml:"goamd64,omitempty"`
	CommitAuthor      CommitAuthor `yaml:"commit_author,omitempty"`
	CommitMsgTemplate string       `yaml:"commit_msg_template,omitempty"`
	Path              string       `yaml:"path,omitempty"`
	PostInstall       string       `yaml:"post_install,omitempty"`
	Install           string       `yaml:"install,omitempty"`
	ExtraInstall      string       `yaml:"extra_install,omitempty"`
	Dependencies      []string     `yaml:"dependencies,omitempty"`
}

// Fury represents Fury.io configuration
type Fury struct {
	Account    string   `yaml:"account,omitempty"`
	SkipUpload string   `yaml:"skip_upload,omitempty"`
	IDs        []string `yaml:"ids,omitempty"`
}

// CloudSmith represents CloudSmith configuration
type CloudSmith struct {
	Owner        string   `yaml:"owner,omitempty"`
	Repository   string   `yaml:"repository,omitempty"`
	SkipUpload   string   `yaml:"skip_upload,omitempty"`
	IDs          []string `yaml:"ids,omitempty"`
	Distribution string   `yaml:"distribution,omitempty"`
}

// TemplateFile represents template file configuration
type TemplateFile struct {
	Src     string `yaml:"src"`
	Dst     string `yaml:"dst,omitempty"`
	Content string `yaml:"content,omitempty"`
}

// Metadata represents project metadata
type Metadata struct {
	ModTimestamp string `yaml:"mod_timestamp,omitempty"`
}

// Monorepo represents monorepo configuration
type Monorepo struct {
	Enabled      bool              `yaml:"enabled,omitempty"`
	TagPrefix    string            `yaml:"tag_prefix,omitempty"`
	TagFormat    string            `yaml:"tag_format,omitempty"`
	Dir          string            `yaml:"dir,omitempty"`
	Projects     []MonorepoProject `yaml:"projects,omitempty"`
	AutoDiscover bool              `yaml:"auto_discover,omitempty"`
}

// MonorepoProject represents a project within a monorepo
type MonorepoProject struct {
	Name      string   `yaml:"name"`
	Path      string   `yaml:"path"`
	DependsOn []string `yaml:"depends_on,omitempty"`
}

// Nightly represents nightly build configuration
type Nightly struct {
	TagName      string `yaml:"tag_name,omitempty"`
	NameTemplate string `yaml:"name_template,omitempty"`
	Publish      bool   `yaml:"publish,omitempty"`
	KeepN        int    `yaml:"keep_n,omitempty"`
}

// Split represents build splitting for distributed builds
type Split struct {
	ID      string `yaml:"id,omitempty"`
	Enabled bool   `yaml:"enabled,omitempty"`
	Count   int    `yaml:"count,omitempty"`
}

// Prebuilt represents prebuilt binary configuration
type Prebuilt struct {
	ID           string `yaml:"id,omitempty"`
	Path         string `yaml:"path,omitempty"`
	NameTemplate string `yaml:"name_template,omitempty"`
}
