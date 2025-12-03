# Multi-Binary Application Example

This example demonstrates how to build and package a project with multiple binaries:

1. **CLI Binary** (`myapp-cli`) - A command-line Hello World tool installed to `/usr/bin`
2. **GUI Binary** (`myapp-gui`) - A graphical Hello World application with desktop integration using [Fyne](https://fyne.io)

## Features

### CLI Application
- Interactive and non-interactive modes
- Multi-language greetings
- System information display
- Beautiful terminal UI with Unicode borders

### GUI Application
- Full Fyne-based graphical interface
- Real-time clock display
- Multi-language greeting dialog
- Menu bar with About dialog
- Cross-platform native look and feel

## Quick Start

```bash
# Run the CLI
go run ./cmd
go run ./cmd hello John
go run ./cmd --interactive

# Run the GUI
go run ./gui/cmd
```

## Project Structure

```
multi-binary/
├── .releaser.yaml          # Releaser configuration
├── go.mod
├── cmd/
│   └── main.go            # CLI Hello World application
├── gui/
│   └── cmd/
│       └── main.go        # GUI Hello World application (Fyne)
├── icons/                  # Application icons
│   ├── myapp.png          # Linux icon
│   ├── myapp.ico          # Windows icon
│   └── myapp.icns         # macOS icon
├── scripts/
│   ├── postinstall.sh     # Post-installation script
│   └── preremove.sh       # Pre-removal script
└── config.example.yaml    # Example configuration
```

## Build Types

### CLI Application (`type: cli`)
- Installed to system PATH (`/usr/bin`, `/usr/local/bin`)
- Available via terminal/command prompt
- No desktop integration needed

### GUI Application (`type: gui`)
- Installed as a desktop application
- **Linux**: `.desktop` file in `/usr/share/applications/`
- **macOS**: `.app` bundle in `/Applications/`
- **Windows**: Start Menu shortcut, optional Desktop shortcut

## Building

```bash
# Build all packages
releaser build --snapshot

# Build release
releaser release
```

## Generated Packages

### Linux
- **DEB** (Debian/Ubuntu): Installs both CLI and GUI
- **RPM** (Fedora/RHEL): Installs both CLI and GUI
- **APK** (Alpine): Installs both CLI and GUI
- **Flatpak**: GUI with sandbox
- **AppImage**: Portable GUI
- **Snap**: Confined GUI

### macOS
- **CLI via Homebrew**: `brew install myapp`
- **GUI via DMG**: Drag and drop installation
- **GUI via PKG**: Installer package

### Windows
- **CLI via Scoop**: `scoop install myapp`
- **GUI via MSI**: Windows Installer
- **GUI via NSIS**: Self-extracting installer
- **Chocolatey**: Package manager

## Installation Locations

| OS      | CLI Location        | GUI Location              |
|---------|---------------------|---------------------------|
| Linux   | /usr/bin/myapp      | /usr/share/applications/  |
| macOS   | /usr/local/bin/myapp| /Applications/MyApp.app   |
| Windows | %PATH%\myapp.exe    | Start Menu & Desktop      |

## Configuration

### Desktop integration

The `gui` section in the build config controls desktop integration:

```yaml
builds:
  - id: gui
    type: gui
    gui:
      name: "MyApp"
      icon: "icons/myapp.png"
      categories:
        - Utility
        - Development
      macos:
        bundle_id: "com.example.myapp"
      windows:
        start_menu_folder: "MyApp"
        desktop_shortcut: true
```

### Secure Go builds

Go builds can enable obfuscation/garbling directly from `.releaser.yaml` without custom scripts:

```yaml
builds:
  - id: cli
    main: ./cmd
    obfuscation:
      enabled: true
      tool: garble
      flags:
        - -literals
        - -tiny
        - -debug=false
        - -seed=random
      env:
        - GARBLE_SEED=random
```

Releaser will invoke `garble build` with the configured flags and reuse the same Go arguments it would normally pass to `go build`, matching the behavior of `secure-build.sh`.

### Version naming templates

You can tweak how `{{ .Version }}` renders everywhere (artifact names, package metadata, etc.) via `versioning.template`. The example strips the automatic `-SNAPSHOT` suffix so snapshot builds reuse the underlying git describe string:

```yaml
versioning:
  template: '{{ if .IsSnapshot }}{{ .OriginalVersion }}{{ else }}{{ .Version }}{{ end }}'
```

Templating has access to the entire context (`OriginalVersion`, `IsSnapshot`, `Branch`, etc.) so you can inject custom suffixes/prefixes or date stamps without editing every archive name by hand.

### Default icons

If you omit `gui.icon`, Releaser now ships a built-in placeholder icon (PNG/ICO/ICNS). During packaging it writes the default assets under `.releaser-icons/` inside the dist directory and wires them into NFPM/AppImage/macOS bundles automatically, so GUI builds no longer need to keep `icons/` in the repository unless you want branded artwork.
