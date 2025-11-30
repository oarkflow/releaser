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
go run ./cmd/cli
go run ./cmd/cli hello John
go run ./cmd/cli --interactive

# Run the GUI
go run ./cmd/gui
```

## Project Structure

```
multi-binary/
├── .releaser.yaml          # Releaser configuration
├── go.mod
├── cmd/
│   ├── cli/
│   │   └── main.go        # CLI Hello World application
│   └── gui/
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
