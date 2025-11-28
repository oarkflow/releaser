# Releaser Example Configurations

This directory contains sample Releaser configuration files for various programming languages and project types.

## Available Examples

| File | Language/Framework | Description |
|------|-------------------|-------------|
| [go-project.yaml](go-project.yaml) | Go | Complete Go CLI tool with cross-compilation, Docker, Homebrew, checksums |
| [rust-project.yaml](rust-project.yaml) | Rust | Rust CLI with Cargo, UPX compression, musl static builds |
| [python-project.yaml](python-project.yaml) | Python | Python CLI with PyInstaller, Nuitka, PyPI publishing |
| [php-project.yaml](php-project.yaml) | PHP | PHP CLI with Composer, Phar building, signing |
| [typescript-nodejs.yaml](typescript-nodejs.yaml) | TypeScript/Node.js | TypeScript CLI using pkg for binaries, npm publish |
| [react-project.yaml](react-project.yaml) | React | React component library with npm publish |
| [vue-project.yaml](vue-project.yaml) | Vue.js | Vue.js component library with Vite build |
| [java-project.yaml](java-project.yaml) | Java/Spring Boot | Java with Maven/Gradle, Docker, GraalVM native-image |
| [scala-project.yaml](scala-project.yaml) | Scala | Scala with sbt, native images, Spark support |
| [kotlin-project.yaml](kotlin-project.yaml) | Kotlin | Kotlin with Gradle, GraalVM, multiplatform support |
| [cross-platform-packaging.yaml](cross-platform-packaging.yaml) | Multi-Platform | Complete OS packaging: macOS DMG/App, Windows MSI/NSIS, Linux deb/rpm |

## Usage

To use any of these examples as a starting point for your project:

1. Copy the relevant example to your project root:
   ```bash
   cp examples/go-project.yaml .releaser.yaml
   ```

2. Customize the configuration for your project:
   - Update `project_name`
   - Set your GitHub owner/repo in `release.github`
   - Adjust build targets, archives, and publishing options

3. Validate your configuration:
   ```bash
   releaser check
   ```

4. Run a release:
   ```bash
   releaser release
   ```

## Configuration Highlights

### Go Projects
- Cross-compilation for Linux, macOS, Windows (amd64/arm64)
- CGO disabled by default for maximum portability
- Docker multi-arch builds with manifest support
- Homebrew formula generation
- Linux packages (deb/rpm)

### Rust Projects
- Cross-compilation with Cargo
- Musl static linking for portable Linux binaries
- UPX compression for smaller binaries
- Distroless Docker images
- Shell completions (bash, zsh, fish)
- Publish to crates.io

### Python Projects
- PyInstaller for standalone executables
- Nuitka for compiled Python (optional)
- Shiv/PEX for self-contained archives
- PyPI publishing with twine
- Slim and distroless Docker images

### PHP Projects
- Composer-based dependency management
- Phar archive creation and signing
- PHP 8.2+ support
- Docker images with PHP-FPM

### TypeScript/Node.js Projects
- Uses `pkg` for creating standalone binaries
- NPM package publishing
- Native Node.js SEA (Single Executable Application) support
- Docker images with Node.js runtime

### React/Vue Projects
- Library building with Vite/esbuild
- NPM package publishing
- Component library documentation

### Java Projects
- Maven and Gradle support
- Fat JAR with Shadow/Assembly plugins
- GraalVM native-image for AOT compilation
- Spring Boot jar packaging
- Publish to Maven Central

### Scala Projects
- sbt-assembly for fat JARs
- Scala Native support
- Scala.js for JavaScript output
- Spark application packaging

### Kotlin Projects
- Gradle Shadow plugin for fat JARs
- GraalVM native-image
- Kotlin/Native multiplatform
- Spring Boot integration

### Cross-Platform Packaging
- **macOS**: App bundles (.app) and DMG disk images with background, icons, volume name
- **Windows**: MSI installers (via WiX) and NSIS installers with shortcuts, registry
- **Linux**: deb, rpm, apk packages via nfpm with systemd services, config files
- Architecture support: amd64, arm64
- Code signing for all platforms (Apple notarization, Windows Authenticode, GPG)

## Common Features

All examples demonstrate:

- **Versioning**: Automatic version detection from git tags
- **Changelog**: Automatic changelog generation with commit categorization
- **Checksums**: SHA256 checksums for all artifacts
- **Archives**: tar.gz for Unix, zip for Windows
- **Signing**: GPG or cosign signature support
- **Docker**: Multi-architecture Docker images
- **Publishing**: GitHub Releases, Homebrew, NPM, Maven Central

## Environment Variables

Most examples expect these environment variables for publishing:

```bash
# GitHub
export GITHUB_TOKEN=ghp_xxxxx
export GITHUB_OWNER=myorg
export GITHUB_REPO=myproject

# NPM (for Node.js projects)
export NPM_TOKEN=npm_xxxxx

# GPG Signing
export GPG_FINGERPRINT=your_fingerprint

# Maven/Sonatype (for JVM projects)
export SONATYPE_USERNAME=your_username
export SONATYPE_TOKEN=your_token
```

## Customization

Each example can be customized:

1. **Add more build targets**: Extend `goos` and `goarch` arrays
2. **Skip components**: Set `skip: true` on any section
3. **Change archive format**: Use `tar.gz`, `tar.xz`, or `zip`
4. **Add custom hooks**: Use `before` and `after` sections
5. **Enable AI changelog**: Set `ai.enabled: true` with OpenAI/Anthropic config

## Validation

All examples have been tested with:

```bash
releaser check -c examples/<file>.yaml
```

## Contributing

Feel free to submit PRs with additional examples for other languages or frameworks!
