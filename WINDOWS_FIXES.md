# Windows Compatibility Fixes

This document summarizes all Windows compatibility fixes applied to the releaser build system.

## Issues Fixed

### 1. PKG_CONFIG_PATH Separator (builder.go)
**Problem:** Used Unix colon (`:`) separator for PKG_CONFIG_PATH environment variable  
**Location:** `internal/builder/builder.go` line ~174  
**Fix:** Use OS-specific separator (`;` on Windows, `:` on Unix)
```go
// Use OS-specific path separator (: on Unix, ; on Windows)
separator := ":"
if runtime.GOOS == "windows" {
    separator = ";"
}
pkgConfigPath := strings.Join(build.Cgo.PKGConfig, separator)
```

### 2. SHA256 Checksum Generation (builder.go)
**Problem:** Used Unix-only `sha256sum` command  
**Location:** `internal/builder/builder.go` line ~727  
**Fix:** Use Go's native `crypto/sha256` package for cross-platform compatibility
```go
// Read the binary file
data, err := os.ReadFile(output)
if err != nil {
    return fmt.Errorf("failed to read binary for checksum: %w", err)
}

// Calculate SHA256 using Go's native crypto
hash := sha256.Sum256(data)
checksumOutput := fmt.Sprintf("%x  %s\n", hash, filepath.Base(output))
```

### 3. Hook Shell Execution (hook.go)
**Problem:** Hardcoded `/bin/sh` shell with `-c` flag  
**Location:** `internal/hook/hook.go` lines 62 and 117  
**Fix:** Detect OS and use PowerShell on Windows with `-Command` flag
```go
shellPath := os.Getenv("SHELL")
if shellPath == "" {
    if runtime.GOOS == "windows" {
        shellPath = "powershell.exe"
    } else {
        shellPath = "/bin/sh"
    }
}

if runtime.GOOS == "windows" {
    c = exec.CommandContext(ctx, shellPath, "-Command", cmd)
} else {
    c = exec.CommandContext(ctx, shellPath, "-c", cmd)
}
```

### 4. Pipeline Shell Execution (pipeline.go)
**Problem:** Hardcoded `/bin/sh` shell  
**Location:** `internal/pipeline/pipeline.go` line ~554  
**Fix:** Same OS detection and PowerShell support as hook.go

### 5. Builder Hook Command Execution (builder.go)
**Problem:** Hardcoded `sh -c` for pre/post build hooks  
**Location:** `internal/builder/builder.go` line ~1277 (runHookCmd function)  
**Fix:** Detect OS and use appropriate shell
```go
// Determine appropriate shell for platform
shell := "sh"
shellArg := "-c"
if runtime.GOOS == "windows" {
    shell = "powershell.exe"
    shellArg = "-Command"
}

cmd := exec.CommandContext(ctx, shell, shellArg, expanded)
```

### 6. Checksum Filename Template Expansion (checksum.go)
**Problem:** Template placeholders like `{{ .ProjectName }}` not replaced in checksum filename  
**Location:** `internal/checksum/checksum.go` line ~97  
**Fix:** 
- Added `templateCtx *tmpl.Context` field to Generator struct
- Updated NewGenerator to accept template context parameter
- Apply template expansion to checksum filename before creating file
```go
// Apply template to filename
if g.templateCtx != nil {
    expandedFile, err := g.templateCtx.Apply(checksumFile)
    if err != nil {
        log.Warn("Failed to apply template to checksum filename, using as-is", "template", checksumFile, "error", err)
    } else {
        checksumFile = expandedFile
    }
}
```

## Testing

All fixes were tested on Windows with PowerShell:

✅ Simple Go binary build  
✅ Multi-platform cross-compilation (Windows, Linux, Darwin × amd64/arm64)  
✅ Pre/post build hooks with PowerShell commands  
✅ Checksum generation with proper template expansion  
✅ Native SHA256 calculation without external tools  

## Files Modified

1. `internal/builder/builder.go` - 3 fixes (PKG_CONFIG_PATH, SHA256, hook execution)
2. `internal/hook/hook.go` - 2 fixes (shell detection for hooks and commands)
3. `internal/pipeline/pipeline.go` - 2 fixes (shell detection, template context passing)
4. `internal/checksum/checksum.go` - 1 fix (template expansion)

## Verification

Build and test with:
```powershell
# Build the releaser itself
go build -o releaser.exe cmd/releaser/main.go

# Test with your project
.\releaser.exe build --config .releaser.yaml
```

All Windows-specific issues have been resolved. The build system now works seamlessly on Windows without requiring Unix tools or WSL.
