# Windows Build Quick Start

## Prerequisites
- Go 1.20+ installed
- PowerShell (included with Windows)

## Building the Releaser

```powershell
# Clone and build
git clone <repository-url>
cd releaser
go build -o releaser.exe cmd/releaser/main.go
```

## Using Releaser on Windows

### Build Your Project
```powershell
# Create a .releaser.yaml configuration file
# Then build
.\releaser.exe build --config .releaser.yaml
```

### Build for Multiple Platforms
```powershell
# Cross-compile for Windows, Linux, macOS
.\releaser.exe build --config .releaser.yaml --clean
```

### Build for Single Platform (faster testing)
```powershell
.\releaser.exe build --single-target windows_amd64
```

## Key Windows Compatibility Features

✅ **Native PowerShell support** - No WSL or Bash required  
✅ **Cross-platform checksums** - Pure Go implementation  
✅ **Path handling** - Automatically handles Windows path separators  
✅ **Build hooks** - Pre/post build commands work with PowerShell  

## Common Commands

```powershell
# Build only
.\releaser.exe build

# Build with cleanup
.\releaser.exe build --clean

# Full release (build + publish)
.\releaser.exe release

# Snapshot release (no git tag required)
.\releaser.exe release --snapshot

# Get help
.\releaser.exe --help
.\releaser.exe build --help
```

## Configuration Tips for Windows

### Hook Commands
Use PowerShell syntax in your hooks:
```yaml
builds:
  - id: myapp
    hooks:
      pre: Write-Host "Building..."
      post: Get-Item .\dist\*.exe
```

### Environment Variables
```yaml
env:
  - CGO_ENABLED=0
  - GOOS=windows
  - GOARCH=amd64
```

## Troubleshooting

### "executable not found" errors
Make sure to use Windows-compatible commands or PowerShell cmdlets in hooks.

### Path issues
The build system automatically handles Windows path separators (`\`).

### CGO builds
For CGO-enabled builds on Windows, install MinGW-w64 or use WSL for cross-compilation.

## Support

All core features work natively on Windows without requiring Unix tools or WSL.
