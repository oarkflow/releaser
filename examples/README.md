# Releaser Example Configurations

This directory contains comprehensive Releaser configuration files for virtually every programming language and project type.

## Available Examples

### Systems Programming Languages

| File | Language/Framework | Description |
|------|-------------------|-------------|
| [go-project.yaml](go-project.yaml) | Go | Complete Go CLI tool with cross-compilation, Docker, Homebrew, checksums |
| [rust-project.yaml](rust-project.yaml) | Rust | Rust CLI with Cargo, UPX compression, musl static builds, crates.io |
| [cpp-project.yaml](cpp-project.yaml) | C/C++ | CMake/Make builds, cross-compilation, static libraries, NSIS/Flatpak |
| [zig-project.yaml](zig-project.yaml) | Zig | Native Zig builds, WASM, musl static, cross-compilation |
| [crystal-project.yaml](crystal-project.yaml) | Crystal | Crystal builds, static binaries, Shards publishing |
| [nim-project.yaml](nim-project.yaml) | Nim | Nim native builds, Nimble publishing, cross-compilation |

### JVM Languages

| File | Language/Framework | Description |
|------|-------------------|-------------|
| [java-project.yaml](java-project.yaml) | Java/Spring Boot | Java with Maven/Gradle, Docker, GraalVM native-image |
| [scala-project.yaml](scala-project.yaml) | Scala | Scala with sbt, native images, Spark support |
| [kotlin-project.yaml](kotlin-project.yaml) | Kotlin | Kotlin with Gradle, GraalVM, multiplatform support |
| [clojure-project.yaml](clojure-project.yaml) | Clojure | Leiningen, GraalVM native-image, Clojars publishing |

### Scripting Languages

| File | Language/Framework | Description |
|------|-------------------|-------------|
| [python-project.yaml](python-project.yaml) | Python | PyInstaller, Nuitka, PyPI publishing |
| [ruby-project.yaml](ruby-project.yaml) | Ruby | Gem building, RubyGems publishing, Homebrew |
| [php-project.yaml](php-project.yaml) | PHP | Composer, Phar building, signing |
| [perl-project.yaml](perl-project.yaml) | Perl | PAR::Packer, CPAN publishing |
| [lua-project.yaml](lua-project.yaml) | Lua | LuaRocks publishing, Luastatic builds |

### JavaScript/TypeScript Runtimes

| File | Language/Framework | Description |
|------|-------------------|-------------|
| [typescript-nodejs.yaml](typescript-nodejs.yaml) | TypeScript/Node.js | pkg binaries, npm publish, SEA |
| [deno-project.yaml](deno-project.yaml) | Deno | Deno compile, JSR publishing, Deno Deploy |
| [bun-project.yaml](bun-project.yaml) | Bun | Bun compile, npm publishing |

### Frontend Frameworks

| File | Language/Framework | Description |
|------|-------------------|-------------|
| [react-project.yaml](react-project.yaml) | React | Component library with npm publish |
| [vue-project.yaml](vue-project.yaml) | Vue.js | Component library with Vite build |

### Mobile & Cross-Platform

| File | Language/Framework | Description |
|------|-------------------|-------------|
| [flutter-project.yaml](flutter-project.yaml) | Flutter/Dart | iOS/Android/Desktop/Web, pub.dev publishing |
| [swift-project.yaml](swift-project.yaml) | Swift | macOS/iOS apps, App Bundle, DMG, Homebrew |

### .NET Ecosystem

| File | Language/Framework | Description |
|------|-------------------|-------------|
| [dotnet-project.yaml](dotnet-project.yaml) | .NET/C# | Console apps, NuGet publishing, Docker |

### Functional Languages

| File | Language/Framework | Description |
|------|-------------------|-------------|
| [elixir-project.yaml](elixir-project.yaml) | Elixir/Erlang | OTP releases, Hex.pm publishing, Burrito |
| [haskell-project.yaml](haskell-project.yaml) | Haskell | Stack/Cabal, Hackage publishing |
| [ocaml-project.yaml](ocaml-project.yaml) | OCaml | Dune builds, OPAM publishing |

### Scientific Computing

| File | Language/Framework | Description |
|------|-------------------|-------------|
| [julia-project.yaml](julia-project.yaml) | Julia | PackageCompiler, Julia Registry publishing |

### Multi-Platform Packaging

| File | Language/Framework | Description |
|------|-------------------|-------------|
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

## Configuration Highlights by Language

### Systems Languages

#### Go Projects
- Cross-compilation for Linux, macOS, Windows (amd64/arm64)
- CGO disabled by default for maximum portability
- Docker multi-arch builds with manifest support
- Homebrew formula generation
- Linux packages (deb/rpm)

#### Rust Projects
- Cross-compilation with Cargo
- Musl static linking for portable Linux binaries
- UPX compression for smaller binaries
- Publish to crates.io
- Shell completions (bash, zsh, fish)

#### C/C++ Projects
- CMake and Make build system support
- Cross-compilation for multiple targets
- Static library packaging
- NSIS Windows installers
- Flatpak and AppImage for Linux

#### Zig Projects
- Native cross-compilation with Zig build system
- WebAssembly (WASM) output
- Musl static builds
- Freestanding/embedded builds

### JVM Languages

#### Java Projects
- Maven and Gradle support
- Fat JAR with Shadow/Assembly plugins
- GraalVM native-image for AOT compilation
- Spring Boot jar packaging
- Publish to Maven Central

#### Scala Projects
- sbt-assembly for fat JARs
- Scala Native support
- Scala.js for JavaScript output
- Spark application packaging

#### Kotlin Projects
- Gradle Shadow plugin for fat JARs
- GraalVM native-image
- Kotlin/Native multiplatform
- Spring Boot integration

#### Clojure Projects
- Leiningen uberjar builds
- GraalVM native-image compilation
- Babashka script support
- Clojars publishing

### Scripting Languages

#### Python Projects
- PyInstaller for standalone executables
- Nuitka for compiled Python (optional)
- Shiv/PEX for self-contained archives
- PyPI publishing with twine

#### Ruby Projects
- Gem building and packaging
- RubyGems publishing
- Bundler support
- Homebrew formula generation

#### PHP Projects
- Composer-based dependency management
- Phar archive creation and signing
- PHP 8.2+ support
- Docker images with PHP-FPM

#### Perl Projects
- PAR::Packer for standalone executables
- FatPacker for single-file scripts
- CPAN publishing

#### Lua Projects
- LuaRocks publishing
- Luastatic for compiled binaries
- LuaJIT compilation

### JavaScript Runtimes

#### TypeScript/Node.js Projects
- Uses `pkg` for creating standalone binaries
- Native Node.js SEA (Single Executable Application)
- NPM package publishing
- Docker images with Node.js runtime

#### Deno Projects
- Deno compile for standalone executables
- JSR (JavaScript Registry) publishing
- Deno Deploy integration
- NPM compatibility layer

#### Bun Projects
- Bun compile for standalone executables
- Minified builds
- NPM publishing support
- Browser bundle output

### Frontend Frameworks

#### React/Vue Projects
- Library building with Vite/esbuild
- NPM package publishing
- Component library documentation
- CDN-ready bundles

### Mobile & Cross-Platform

#### Flutter/Dart Projects
- Android APK and App Bundle (AAB)
- iOS IPA builds
- macOS, Windows, Linux desktop
- Web builds with CanvasKit
- pub.dev publishing

#### Swift Projects
- macOS App Bundle packaging
- DMG disk image creation
- iOS builds (on macOS)
- Homebrew formula

### .NET Ecosystem

#### .NET/C# Projects
- Console app single-file publishing
- NuGet package creation
- Cross-platform builds
- Docker multi-arch images

### Functional Languages

#### Elixir/Erlang Projects
- OTP release builds
- Hex.pm publishing
- Burrito for native executables
- Escript support

#### Haskell Projects
- Stack and Cabal support
- Hackage publishing
- Static musl builds
- Stackage integration

#### OCaml Projects
- Dune build system
- OPAM repository publishing
- Native compilation
- Cross-compilation support

### Scientific Computing

#### Julia Projects
- PackageCompiler standalone apps
- BinaryBuilder for cross-platform
- Julia General Registry publishing
- Jupyter integration

## Cross-Platform Packaging Features

All examples support comprehensive cross-platform packaging:

- **macOS**: App bundles (.app) and DMG disk images with background, icons, volume name
- **Windows**: MSI installers (via WiX) and NSIS installers with shortcuts, registry
- **Linux**: deb, rpm, apk, Arch packages via nfpm with systemd services
- **Flatpak**: Desktop app distribution for all Linux distros
- **AppImage**: Portable Linux applications
- **Snap**: Ubuntu/Canonical store distribution
- Architecture support: amd64, arm64, arm (32-bit), WASM
- Code signing for all platforms (Apple notarization, Windows Authenticode, GPG)

## Common Features

All examples demonstrate:

- **Versioning**: Automatic version detection from git tags
- **Changelog**: Automatic changelog generation with commit categorization
- **Checksums**: SHA256 checksums for all artifacts
- **Archives**: tar.gz for Unix, zip for Windows
- **Signing**: GPG or cosign signature support
- **Docker**: Multi-architecture Docker images with manifests
- **SBOM**: Software Bill of Materials generation (SPDX, CycloneDX)
- **Publishing**: GitHub Releases, language-specific registries

## Package Registry Support

| Registry | Languages |
|----------|-----------|
| NPM | JavaScript, TypeScript, Bun |
| PyPI | Python |
| Crates.io | Rust |
| RubyGems | Ruby |
| Maven Central | Java, Scala, Kotlin |
| NuGet | .NET, C# |
| Hex.pm | Elixir, Erlang |
| Clojars | Clojure |
| Hackage | Haskell |
| OPAM | OCaml |
| CPAN | Perl |
| LuaRocks | Lua |
| pub.dev | Dart, Flutter |
| JSR | Deno, JavaScript |
| Nimble | Nim |
| Julia Registry | Julia |

## OS Package Manager Support

| Package Manager | Platform |
|----------------|----------|
| Homebrew | macOS, Linux |
| AUR | Arch Linux |
| Scoop | Windows |
| Winget | Windows |
| Chocolatey | Windows |
| Snapcraft | Ubuntu/Linux |
| Flatpak | Linux |

## Environment Variables

Most examples expect these environment variables for publishing:

```bash
# GitHub
export GITHUB_TOKEN=ghp_xxxxx
export GITHUB_OWNER=myorg
export GITHUB_REPO=myproject

# NPM (for Node.js projects)
export NPM_TOKEN=npm_xxxxx

# PyPI (for Python projects)
export TWINE_USERNAME=__token__
export TWINE_PASSWORD=pypi-xxxxx

# Crates.io (for Rust projects)
export CARGO_REGISTRY_TOKEN=xxxxx

# RubyGems (for Ruby projects)
export GEM_HOST_API_KEY=xxxxx

# Maven/Sonatype (for JVM projects)
export SONATYPE_USERNAME=your_username
export SONATYPE_TOKEN=your_token

# NuGet (for .NET projects)
export NUGET_API_KEY=xxxxx

# Docker
export DOCKER_USERNAME=your_username
export DOCKER_PASSWORD=your_token

# GPG Signing
export GPG_FINGERPRINT=your_fingerprint
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
