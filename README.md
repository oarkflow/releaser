# Releaser

A powerful, multi-language release automation tool similar to GoReleaser, supporting various programming languages and extensive customization options.

## Features

### Multi-Language Support
- **Go**: Full support with CGO, cross-compilation, and ldflags
- **Rust**: Cargo integration with target triples
- **Node.js**: npm/yarn builds and publishing
- **Python**: pip/poetry builds and publishing
- **Generic**: Custom build commands for any language

### Packaging
- **Archives**: tar.gz, zip with customizable templates
- **Linux Packages**: deb, rpm, apk via nfpm/fpm
- **macOS**: App Bundles, DMG with notarization
- **Windows**: MSI, NSIS installers
- **Docker**: Multi-platform builds with buildx

### Signing & Security
- **Code Signing**: GPG, cosign, minisign
- **macOS Notarization**: Automated notarization workflow
- **Checksums**: SHA256, SHA512, MD5
- **SBOM**: Software Bill of Materials generation

### Publishing
- **GitHub Releases**: Automatic release creation
- **GitLab Releases**: Full GitLab integration
- **NPM Registry**: npm package publishing
- **Homebrew**: Tap formula generation
- **Cloudsmith/Gemfury**: Package hosting
- **Docker Hub/Registry**: Container publishing

### Release Notes
- **Conventional Commits**: Automatic grouping by type
- **AI Enhancement**: OpenAI-powered release notes
- **Template Support**: Fully customizable templates
- **Include/Exclude Patterns**: Commit filtering

### Announcements
- **Slack**: Webhook notifications
- **Discord**: Rich embeds support
- **Teams**: Adaptive cards
- **Telegram**: Bot integration
- **Mastodon**: Fediverse support
- **Webhooks**: Custom endpoints

### Other Features
- **Hooks**: Pre/post build hooks
- **Monorepo Support**: Multi-project releases
- **Split Builds**: Distributed building
- **UPX Compression**: Binary compression
- **Parallel Builds**: Configurable parallelism
- **Snapshot/Nightly**: Development releases

## Installation

```bash
# Using Go
go install github.com/oarkflow/releaser/cmd/releaser@latest

# From source
git clone https://github.com/oarkflow/releaser
cd releaser
go build -o releaser ./cmd/releaser
```

## Quick Start

1. Initialize a configuration file:
```bash
releaser init
```

2. Build and test locally:
```bash
releaser build --snapshot
```

3. Create a release:
```bash
git tag v1.0.0
releaser release
```

## Configuration

Create a `.releaser.yaml` file in your project root:

```yaml
version: 2

project_name: myapp

before:
  hooks:
    - go mod tidy

builds:
  - id: myapp
    main: ./cmd/myapp
    binary: myapp
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}

archives:
  - id: default
    format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: checksums.txt
  algorithm: sha256

changelog:
  sort: asc
  use: conventional
  groups:
    - title: Features
      regexp: "^feat"
    - title: Bug Fixes
      regexp: "^fix"
    - title: Documentation
      regexp: "^docs"

release:
  github:
    owner: myorg
    name: myrepo
```

## Commands

### `releaser release`
Create a full release with all configured steps.

```bash
releaser release                    # Full release
releaser release --snapshot         # Snapshot release (no git tag)
releaser release --prepare          # Prepare without publishing
releaser release --skip-publish     # Skip publishing step
releaser release --skip-sign        # Skip signing step
```

### `releaser build`
Build artifacts only without publishing.

```bash
releaser build                      # Build all targets
releaser build --snapshot           # Build snapshot
releaser build --single-target linux_amd64  # Single target
```

### `releaser changelog`
Generate or preview changelog.

```bash
releaser changelog                  # Generate changelog
releaser changelog --output md      # Markdown output
releaser changelog --use-ai         # AI-enhanced notes
```

### `releaser check`
Validate configuration file.

```bash
releaser check                      # Validate config
releaser check --strict             # Strict validation
```

### `releaser publish`
Publish prepared artifacts.

```bash
releaser publish                    # Publish all
releaser publish --github           # GitHub only
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GITHUB_TOKEN` | GitHub API token |
| `GITLAB_TOKEN` | GitLab API token |
| `NPM_TOKEN` | NPM registry token |
| `DOCKER_USERNAME` | Docker Hub username |
| `DOCKER_PASSWORD` | Docker Hub password |
| `SLACK_WEBHOOK_URL` | Slack webhook |
| `DISCORD_WEBHOOK_URL` | Discord webhook |
| `GPG_FINGERPRINT` | GPG signing key |
| `OPENAI_API_KEY` | OpenAI API key (for AI changelog) |
| `APPLE_ID` | Apple ID for notarization |
| `APPLE_PASSWORD` | App-specific password |

## Advanced Configuration

### Multiple Builds
```yaml
builds:
  - id: cli
    main: ./cmd/cli
    binary: myapp

  - id: server
    main: ./cmd/server
    binary: myapp-server
```

### Docker Builds
```yaml
dockers:
  - id: myapp
    dockerfile: Dockerfile
    image_templates:
      - "myorg/myapp:{{ .Version }}"
      - "myorg/myapp:latest"
    buildx: true
    buildx_platforms:
      - linux/amd64
      - linux/arm64
```

### macOS App Bundle
```yaml
app_bundles:
  - id: myapp
    name: MyApp
    identifier: com.myorg.myapp
    icon: assets/icon.icns

dmgs:
  - id: myapp
    name: MyApp
    applications_symlink: true
```

### Windows Installer
```yaml
msis:
  - id: myapp
    name: MyApp
    manufacturer: My Organization

nsiss:
  - id: myapp
    name: MyApp
```

### Announcements
```yaml
announce:
  slack:
    enabled: true
    channel: "#releases"

  discord:
    enabled: true

  telegram:
    enabled: true
    chat_id: "-123456789"
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) first.
