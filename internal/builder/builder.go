/*
Package builder provides build functionality for different languages.
*/
package builder

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/deps"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// Builder interface for language-specific builders
type Builder interface {
	Build(ctx context.Context, build config.Build, target Target, output string, tmplCtx *tmpl.Context) error
	Supports(builder string) bool
}

// Target represents a build target
type Target struct {
	OS    string
	Arch  string
	Arm   string
	Amd64 string
	Mips  string
}

// String returns the target as a string
func (t Target) String() string {
	s := t.OS + "_" + t.Arch
	if t.Arm != "" {
		s += "_v" + t.Arm
	}
	if t.Amd64 != "" {
		s += "_" + t.Amd64
	}
	return s
}

// GoBuilder builds Go binaries
type GoBuilder struct{}

// NewGoBuilder creates a new Go builder
func NewGoBuilder() *GoBuilder {
	return &GoBuilder{}
}

// Supports returns true if this builder supports the given builder type
func (b *GoBuilder) Supports(builder string) bool {
	return builder == "" || builder == "go"
}

// Build builds a Go binary
func (b *GoBuilder) Build(ctx context.Context, build config.Build, target Target, output string, tmplCtx *tmpl.Context) error {
	log.Debug("Building Go binary", "target", target.String(), "output", output)

	// Determine Go binary
	goBinary := build.GoBinary
	if goBinary == "" {
		goBinary = "go"
	}

	// Prepare environment
	env := os.Environ()
	env = append(env, fmt.Sprintf("GOOS=%s", target.OS))
	env = append(env, fmt.Sprintf("GOARCH=%s", target.Arch))
	if target.Arm != "" {
		env = append(env, fmt.Sprintf("GOARM=%s", target.Arm))
	}
	if target.Amd64 != "" {
		env = append(env, fmt.Sprintf("GOAMD64=%s", target.Amd64))
	}
	if target.Mips != "" {
		env = append(env, fmt.Sprintf("GOMIPS=%s", target.Mips))
	}

	// Handle CGO configuration
	if build.Cgo.Enabled {
		env = append(env, "CGO_ENABLED=1")

		// Get cross-compiler for this target
		targetKey := target.OS + "_" + target.Arch
		cc, cxx, err := b.getCrossCompiler(build.Cgo, targetKey, target.OS, target.Arch)
		if err != nil {
			return fmt.Errorf("CGO cross-compilation not available for %s: %w", targetKey, err)
		}

		if cc != "" {
			env = append(env, fmt.Sprintf("CC=%s", cc))
		}
		if cxx != "" {
			env = append(env, fmt.Sprintf("CXX=%s", cxx))
		}

		// Add CGO flags
		if len(build.Cgo.CFlags) > 0 {
			env = append(env, fmt.Sprintf("CGO_CFLAGS=%s", strings.Join(build.Cgo.CFlags, " ")))
		}
		if len(build.Cgo.CXXFlags) > 0 {
			env = append(env, fmt.Sprintf("CGO_CXXFLAGS=%s", strings.Join(build.Cgo.CXXFlags, " ")))
		}
		if len(build.Cgo.LDFlags) > 0 {
			env = append(env, fmt.Sprintf("CGO_LDFLAGS=%s", strings.Join(build.Cgo.LDFlags, " ")))
		}

		// Handle pkg-config
		if len(build.Cgo.PKGConfig) > 0 {
			env = append(env, fmt.Sprintf("PKG_CONFIG_PATH=%s", strings.Join(build.Cgo.PKGConfig, ":")))
		}
	}

	// Add build-specific environment
	for _, e := range build.Env {
		expanded, err := tmplCtx.Apply(e)
		if err != nil {
			return fmt.Errorf("failed to expand env %s: %w", e, err)
		}
		env = append(env, expanded)
	}

	// Add obfuscation-specific environment
	if build.Obfuscation.Enabled {
		for _, e := range build.Obfuscation.Env {
			expanded, err := tmplCtx.Apply(e)
			if err != nil {
				return fmt.Errorf("failed to expand obfuscation env %s: %w", e, err)
			}
			env = append(env, expanded)
		}
	}

	// Prepare ldflags
	var ldflags []string
	for _, ldflag := range build.Ldflags {
		expanded, err := tmplCtx.Apply(ldflag)
		if err != nil {
			return fmt.Errorf("failed to expand ldflag %s: %w", ldflag, err)
		}
		ldflags = append(ldflags, expanded)
	}

	// Prepare tags
	var tags []string
	for _, tag := range build.Tags {
		expanded, err := tmplCtx.Apply(tag)
		if err != nil {
			return fmt.Errorf("failed to expand tag %s: %w", tag, err)
		}
		tags = append(tags, expanded)
	}

	// Build arguments
	args := []string{"build"}

	// Add mod flag
	if build.Mod != "" {
		args = append(args, "-mod="+build.Mod)
	}

	// Add buildmode
	if build.Buildmode != "" {
		args = append(args, "-buildmode="+build.Buildmode)
	}

	// Add flags
	for _, flag := range build.Flags {
		expanded, err := tmplCtx.Apply(flag)
		if err != nil {
			return fmt.Errorf("failed to expand flag %s: %w", flag, err)
		}
		args = append(args, expanded)
	}

	// Add ldflags
	if len(ldflags) > 0 {
		args = append(args, "-ldflags="+strings.Join(ldflags, " "))
	}

	// Add tags
	if len(tags) > 0 {
		args = append(args, "-tags="+strings.Join(tags, ","))
	}

	// Add asmflags
	if len(build.Asmflags) > 0 {
		var asmflags []string
		for _, af := range build.Asmflags {
			expanded, err := tmplCtx.Apply(af)
			if err != nil {
				return fmt.Errorf("failed to expand asmflag %s: %w", af, err)
			}
			asmflags = append(asmflags, expanded)
		}
		args = append(args, "-asmflags="+strings.Join(asmflags, " "))
	}

	// Add gcflags
	if len(build.Gcflags) > 0 {
		var gcflags []string
		for _, gf := range build.Gcflags {
			expanded, err := tmplCtx.Apply(gf)
			if err != nil {
				return fmt.Errorf("failed to expand gcflag %s: %w", gf, err)
			}
			gcflags = append(gcflags, expanded)
		}
		args = append(args, "-gcflags="+strings.Join(gcflags, " "))
	}

	// Add output
	args = append(args, "-o", output)

	// Add main package
	main := build.Main
	if main == "" {
		main = "."
	}
	args = append(args, main)

	// Determine working directory
	dir := build.Dir
	if dir == "" {
		dir, _ = os.Getwd()
	}

	// Create output directory
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Run pre-build hooks
	if build.Hooks.Pre != "" {
		if err := runHookCmd(ctx, build.Hooks.Pre, dir, env, tmplCtx); err != nil {
			return fmt.Errorf("pre-build hook failed: %w", err)
		}
	}

	cmdName := goBinary
	cmdArgs := args

	if build.Obfuscation.Enabled {
		cmdName = build.Obfuscation.Tool
		if cmdName == "" {
			cmdName = "garble"
		}

		var toolFlags []string
		for _, flag := range build.Obfuscation.Flags {
			expanded, err := tmplCtx.Apply(flag)
			if err != nil {
				return fmt.Errorf("failed to expand obfuscation flag %s: %w", flag, err)
			}
			if strings.TrimSpace(expanded) != "" {
				toolFlags = append(toolFlags, expanded)
			}
		}

		cmdArgs = append([]string{}, toolFlags...)
		if !build.Obfuscation.SkipSubcommand {
			subcommand := build.Obfuscation.Subcommand
			if subcommand == "" {
				subcommand = "build"
			}
			cmdArgs = append(cmdArgs, subcommand)
		}
		cmdArgs = append(cmdArgs, args...)
	}

	// Run build
	log.Debug("Running Go compiler", "cmd", cmdName, "args", cmdArgs)
	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	cmd.Dir = dir
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build failed: %w\n%s", err, stderr.String())
	}

	// Run post-build hooks
	if build.Hooks.Post != "" {
		if err := runHookCmd(ctx, build.Hooks.Post, dir, env, tmplCtx); err != nil {
			return fmt.Errorf("post-build hook failed: %w", err)
		}
	}

	log.Info("Built binary", "output", output)
	return nil
}

// getCrossCompiler returns the appropriate C/C++ compiler for a target
func (b *GoBuilder) getCrossCompiler(cgo config.CgoConfig, targetKey, goos, goarch string) (cc, cxx string, err error) {
	hostOS := runtime.GOOS
	hostArch := runtime.GOARCH

	// Native build - no cross-compiler needed
	if goos == hostOS && goarch == hostArch {
		return "", "", nil
	}

	// Check for explicit cross-compiler configuration
	if crossCompiler, ok := cgo.CrossCompilers[targetKey]; ok {
		cc = crossCompiler.CC
		cxx = crossCompiler.CXX

		// Validate configured cross-compilers exist
		if cc != "" {
			// Extract just the binary name for validation
			ccParts := strings.Fields(cc)
			if len(ccParts) > 0 {
				if _, err := exec.LookPath(ccParts[0]); err != nil {
					return "", "", fmt.Errorf("configured CC %q not found", ccParts[0])
				}
			}
		}
		if cxx != "" {
			cxxParts := strings.Fields(cxx)
			if len(cxxParts) > 0 {
				if _, err := exec.LookPath(cxxParts[0]); err != nil {
					return "", "", fmt.Errorf("configured CXX %q not found", cxxParts[0])
				}
			}
		}
		return cc, cxx, nil
	}

	// Use explicitly configured CC/CXX
	if cgo.CC != "" {
		cc = cgo.CC
	}
	if cgo.CXX != "" {
		cxx = cgo.CXX
	}

	// Auto-detect cross-compilers
	if cc == "" {
		cc, err = findCrossCompiler(goos, goarch, "gcc")
		if err != nil {
			return "", "", err
		}
	}
	if cxx == "" {
		cxx, _ = findCrossCompiler(goos, goarch, "g++")
	}

	return cc, cxx, nil
}

// findCrossCompiler looks for an available cross-compiler
func findCrossCompiler(goos, goarch, compiler string) (string, error) {
	// Common cross-compiler prefixes
	crossPrefixes := map[string]map[string]string{
		"linux": {
			"amd64": "x86_64-linux-gnu-",
			"arm64": "aarch64-linux-gnu-",
			"arm":   "arm-linux-gnueabihf-",
			"386":   "i686-linux-gnu-",
		},
		"windows": {
			"amd64": "x86_64-w64-mingw32-",
			"386":   "i686-w64-mingw32-",
		},
		"darwin": {
			// macOS cross-compilation typically uses osxcross
			"amd64": "o64-clang",
			"arm64": "oa64-clang",
		},
	}

	// Look up cross-compiler prefix
	if osPrefixes, ok := crossPrefixes[goos]; ok {
		if prefix, ok := osPrefixes[goarch]; ok {
			// Check if the cross-compiler exists
			crossCC := prefix + compiler
			if _, err := exec.LookPath(crossCC); err == nil {
				return crossCC, nil
			}

			// For macOS with osxcross, clang doesn't have the suffix
			if goos == "darwin" {
				if _, err := exec.LookPath(prefix); err == nil {
					return prefix, nil
				}
			}
		}
	}

	// Try zig as a universal cross-compiler
	if _, err := exec.LookPath("zig"); err == nil {
		zigTarget := getZigTarget(goos, goarch)
		if zigTarget != "" {
			// Create a wrapper script for zig since CC can't have args
			wrapperPath, err := createZigWrapper(zigTarget, compiler == "g++")
			if err == nil {
				return wrapperPath, nil
			}
		}
	}

	// Try to install zig as the universal cross-compiler
	log.Info("No cross-compiler found, attempting to install zig", "target", goos+"/"+goarch)
	if err := deps.CheckAndInstall("zig"); err == nil {
		// Retry with zig
		if _, err := exec.LookPath("zig"); err == nil {
			zigTarget := getZigTarget(goos, goarch)
			if zigTarget != "" {
				wrapperPath, err := createZigWrapper(zigTarget, compiler == "g++")
				if err == nil {
					return wrapperPath, nil
				}
			}
		}
	}

	// Try platform-specific cross-compilers
	switch goos {
	case "windows":
		if err := deps.CheckAndInstall("mingw-w64"); err == nil {
			crossCC := "x86_64-w64-mingw32-" + compiler
			if goarch == "386" {
				crossCC = "i686-w64-mingw32-" + compiler
			}
			if _, err := exec.LookPath(crossCC); err == nil {
				return crossCC, nil
			}
		}
	case "linux":
		if goarch == "arm64" {
			if err := deps.CheckAndInstall("gcc-aarch64"); err == nil {
				crossCC := "aarch64-linux-gnu-" + compiler
				if _, err := exec.LookPath(crossCC); err == nil {
					return crossCC, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no cross-compiler available for %s/%s. Install zig, gcc cross-compilers, or osxcross", goos, goarch)
}

// createZigWrapper creates a wrapper script for zig cc/c++
func createZigWrapper(target string, isCxx bool) (string, error) {
	// Create a temporary wrapper script
	tmpDir := os.TempDir()
	wrapperName := fmt.Sprintf("zig-cc-%s", strings.ReplaceAll(target, "/", "-"))
	if isCxx {
		wrapperName = fmt.Sprintf("zig-cxx-%s", strings.ReplaceAll(target, "/", "-"))
	}
	wrapperPath := filepath.Join(tmpDir, wrapperName)

	// Check if wrapper already exists
	if _, err := os.Stat(wrapperPath); err == nil {
		return wrapperPath, nil
	}

	cmd := "cc"
	if isCxx {
		cmd = "c++"
	}

	script := fmt.Sprintf("#!/bin/sh\nexec zig %s -target %s \"$@\"\n", cmd, target)
	if err := os.WriteFile(wrapperPath, []byte(script), 0755); err != nil {
		return "", err
	}

	return wrapperPath, nil
}

// getZigTarget returns the zig target triple for a platform
func getZigTarget(goos, goarch string) string {
	targets := map[string]map[string]string{
		"linux": {
			"amd64": "x86_64-linux-gnu",
			"arm64": "aarch64-linux-gnu",
			"arm":   "arm-linux-gnueabihf",
			"386":   "i386-linux-gnu",
		},
		"darwin": {
			"amd64": "x86_64-macos",
			"arm64": "aarch64-macos",
		},
		"windows": {
			"amd64": "x86_64-windows-gnu",
			"386":   "i386-windows-gnu",
		},
	}

	if osTargets, ok := targets[goos]; ok {
		if target, ok := osTargets[goarch]; ok {
			return target
		}
	}
	return ""
}

// RustBuilder builds Rust binaries
type RustBuilder struct{}

// NewRustBuilder creates a new Rust builder
func NewRustBuilder() *RustBuilder {
	return &RustBuilder{}
}

// Supports returns true if this builder supports the given builder type
func (b *RustBuilder) Supports(builder string) bool {
	return builder == "rust" || builder == "cargo"
}

// Build builds a Rust binary
func (b *RustBuilder) Build(ctx context.Context, build config.Build, target Target, output string, tmplCtx *tmpl.Context) error {
	log.Debug("Building Rust binary", "target", target.String(), "output", output)

	// Determine Rust target triple
	triple := rustTarget(target)
	if triple == "" {
		return fmt.Errorf("unsupported Rust target: %s", target.String())
	}

	// Prepare environment
	env := os.Environ()
	for _, e := range build.Env {
		expanded, err := tmplCtx.Apply(e)
		if err != nil {
			return fmt.Errorf("failed to expand env %s: %w", e, err)
		}
		env = append(env, expanded)
	}

	// Build arguments
	args := []string{"build", "--release", "--target", triple}

	// Add flags
	for _, flag := range build.Flags {
		expanded, err := tmplCtx.Apply(flag)
		if err != nil {
			return fmt.Errorf("failed to expand flag %s: %w", flag, err)
		}
		args = append(args, expanded)
	}

	// Determine working directory
	dir := build.Dir
	if dir == "" {
		dir, _ = os.Getwd()
	}

	// Run build
	log.Debug("Running cargo build", "args", args)
	cmd := exec.CommandContext(ctx, "cargo", args...)
	cmd.Dir = dir
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cargo build failed: %w\n%s", err, stderr.String())
	}

	// Copy binary to output location
	binaryName := build.Binary
	if binaryName == "" {
		binaryName = filepath.Base(dir)
	}
	if target.OS == "windows" {
		binaryName += ".exe"
	}

	srcPath := filepath.Join(dir, "target", triple, "release", binaryName)
	if err := copyFile(srcPath, output); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	log.Info("Built binary", "output", output)
	return nil
}

// NodeBuilder builds Node.js packages
type NodeBuilder struct{}

// NewNodeBuilder creates a new Node.js builder
func NewNodeBuilder() *NodeBuilder {
	return &NodeBuilder{}
}

// Supports returns true if this builder supports the given builder type
func (b *NodeBuilder) Supports(builder string) bool {
	return builder == "node" || builder == "npm" || builder == "yarn" || builder == "pnpm"
}

// Build builds a Node.js package
func (b *NodeBuilder) Build(ctx context.Context, build config.Build, target Target, output string, tmplCtx *tmpl.Context) error {
	log.Debug("Building Node.js package", "target", target.String(), "output", output)

	// Determine package manager
	pm := "npm"
	if build.Builder == "yarn" {
		pm = "yarn"
	} else if build.Builder == "pnpm" {
		pm = "pnpm"
	}

	// Prepare environment
	env := os.Environ()
	env = append(env, fmt.Sprintf("npm_config_target_platform=%s", target.OS))
	env = append(env, fmt.Sprintf("npm_config_target_arch=%s", nodeArch(target.Arch)))

	for _, e := range build.Env {
		expanded, err := tmplCtx.Apply(e)
		if err != nil {
			return fmt.Errorf("failed to expand env %s: %w", e, err)
		}
		env = append(env, expanded)
	}

	// Determine working directory
	dir := build.Dir
	if dir == "" {
		dir, _ = os.Getwd()
	}

	// Install dependencies
	installArgs := []string{"install"}
	if pm == "npm" {
		installArgs = append(installArgs, "--prefer-offline")
	}

	installCmd := exec.CommandContext(ctx, pm, installArgs...)
	installCmd.Dir = dir
	installCmd.Env = env
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install dependencies: %w", err)
	}

	// Run build
	buildArgs := []string{"run", "build"}
	for _, flag := range build.Flags {
		expanded, err := tmplCtx.Apply(flag)
		if err != nil {
			return fmt.Errorf("failed to expand flag %s: %w", flag, err)
		}
		buildArgs = append(buildArgs, expanded)
	}

	cmd := exec.CommandContext(ctx, pm, buildArgs...)
	cmd.Dir = dir
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm build failed: %w\n%s", err, stderr.String())
	}

	log.Info("Built package", "output", output)
	return nil
}

// PythonBuilder builds Python packages
type PythonBuilder struct{}

// NewPythonBuilder creates a new Python builder
func NewPythonBuilder() *PythonBuilder {
	return &PythonBuilder{}
}

// Supports returns true if this builder supports the given builder type
func (b *PythonBuilder) Supports(builder string) bool {
	return builder == "python" || builder == "pip" || builder == "poetry" || builder == "pyinstaller"
}

// Build builds a Python package
func (b *PythonBuilder) Build(ctx context.Context, build config.Build, target Target, output string, tmplCtx *tmpl.Context) error {
	log.Debug("Building Python package", "target", target.String(), "output", output)

	// Prepare environment
	env := os.Environ()
	for _, e := range build.Env {
		expanded, err := tmplCtx.Apply(e)
		if err != nil {
			return fmt.Errorf("failed to expand env %s: %w", e, err)
		}
		env = append(env, expanded)
	}

	// Determine working directory
	dir := build.Dir
	if dir == "" {
		dir, _ = os.Getwd()
	}

	// Determine build tool
	if build.Builder == "poetry" {
		// Poetry build
		args := []string{"build"}
		for _, flag := range build.Flags {
			expanded, err := tmplCtx.Apply(flag)
			if err != nil {
				return fmt.Errorf("failed to expand flag %s: %w", flag, err)
			}
			args = append(args, expanded)
		}

		cmd := exec.CommandContext(ctx, "poetry", args...)
		cmd.Dir = dir
		cmd.Env = env
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("poetry build failed: %w", err)
		}
	} else if build.Builder == "pyinstaller" {
		// PyInstaller build
		args := []string{"--onefile", "--distpath", filepath.Dir(output)}
		if target.OS == "windows" {
			args = append(args, "--name", strings.TrimSuffix(filepath.Base(output), ".exe"))
		} else {
			args = append(args, "--name", filepath.Base(output))
		}

		for _, flag := range build.Flags {
			expanded, err := tmplCtx.Apply(flag)
			if err != nil {
				return fmt.Errorf("failed to expand flag %s: %w", flag, err)
			}
			args = append(args, expanded)
		}

		main := build.Main
		if main == "" {
			main = "main.py"
		}
		args = append(args, main)

		cmd := exec.CommandContext(ctx, "pyinstaller", args...)
		cmd.Dir = dir
		cmd.Env = env
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pyinstaller build failed: %w", err)
		}
	} else {
		// Standard Python build
		args := []string{"setup.py", "build"}
		for _, flag := range build.Flags {
			expanded, err := tmplCtx.Apply(flag)
			if err != nil {
				return fmt.Errorf("failed to expand flag %s: %w", flag, err)
			}
			args = append(args, expanded)
		}

		cmd := exec.CommandContext(ctx, "python", args...)
		cmd.Dir = dir
		cmd.Env = env
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("python build failed: %w", err)
		}
	}

	log.Info("Built package", "output", output)
	return nil
}

// PrebuiltBuilder copies prebuilt binaries
type PrebuiltBuilder struct{}

// NewPrebuiltBuilder creates a new prebuilt builder
func NewPrebuiltBuilder() *PrebuiltBuilder {
	return &PrebuiltBuilder{}
}

// Supports returns true if this builder supports the given builder type
func (b *PrebuiltBuilder) Supports(builder string) bool {
	return builder == "prebuilt"
}

// Build copies a prebuilt binary
func (b *PrebuiltBuilder) Build(ctx context.Context, build config.Build, target Target, output string, tmplCtx *tmpl.Context) error {
	log.Debug("Copying prebuilt binary", "target", target.String(), "output", output)

	// Determine source path
	srcPath := build.Main
	if srcPath == "" {
		return fmt.Errorf("prebuilt builder requires 'main' to specify source path")
	}

	// Apply template to source path
	srcPath, err := tmplCtx.Apply(srcPath)
	if err != nil {
		return fmt.Errorf("failed to expand source path: %w", err)
	}

	// Copy file
	if err := copyFile(srcPath, output); err != nil {
		return fmt.Errorf("failed to copy prebuilt binary: %w", err)
	}

	log.Info("Copied prebuilt binary", "output", output)
	return nil
}

// runHookCmd runs a simple hook command string
func runHookCmd(ctx context.Context, cmdStr, dir string, env []string, tmplCtx *tmpl.Context) error {
	if cmdStr == "" {
		return nil
	}

	// Expand templates in command
	expanded, err := tmplCtx.Apply(cmdStr)
	if err != nil {
		return fmt.Errorf("failed to expand hook command: %w", err)
	}

	log.Debug("Running hook", "cmd", expanded)

	cmd := exec.CommandContext(ctx, "sh", "-c", expanded)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// runHook runs a build hook
func runHook(ctx context.Context, hook config.Hook, dir string, env []string, tmplCtx *tmpl.Context) error {
	cmdStr, err := tmplCtx.Apply(hook.Cmd)
	if err != nil {
		return err
	}

	hookDir := hook.Dir
	if hookDir == "" {
		hookDir = dir
	}

	// Parse command
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = hookDir

	// Add environment variables
	for k, v := range hook.Env {
		cmd.Env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	if hook.Output != "" {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}

// rustTarget returns the Rust target triple for a target
func rustTarget(target Target) string {
	targets := map[string]map[string]string{
		"linux": {
			"amd64": "x86_64-unknown-linux-gnu",
			"arm64": "aarch64-unknown-linux-gnu",
			"arm":   "armv7-unknown-linux-gnueabihf",
			"386":   "i686-unknown-linux-gnu",
		},
		"darwin": {
			"amd64": "x86_64-apple-darwin",
			"arm64": "aarch64-apple-darwin",
		},
		"windows": {
			"amd64": "x86_64-pc-windows-msvc",
			"arm64": "aarch64-pc-windows-msvc",
			"386":   "i686-pc-windows-msvc",
		},
	}

	if osTargets, ok := targets[target.OS]; ok {
		if triple, ok := osTargets[target.Arch]; ok {
			return triple
		}
	}
	return ""
}

// nodeArch returns the Node.js architecture name
func nodeArch(arch string) string {
	switch arch {
	case "amd64":
		return "x64"
	case "386":
		return "ia32"
	default:
		return arch
	}
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	return os.WriteFile(dst, data, 0755)
}

// JavaBuilder builds Java projects (Maven/Gradle)
type JavaBuilder struct{}

// NewJavaBuilder creates a new Java builder
func NewJavaBuilder() *JavaBuilder {
	return &JavaBuilder{}
}

// Supports returns true if this builder supports the given builder type
func (b *JavaBuilder) Supports(builder string) bool {
	return builder == "java" || builder == "maven" || builder == "mvn" || builder == "gradle"
}

// Build builds a Java project
func (b *JavaBuilder) Build(ctx context.Context, build config.Build, target Target, output string, tmplCtx *tmpl.Context) error {
	log.Debug("Building Java project", "target", target.String(), "output", output)

	// Prepare environment
	env := os.Environ()
	for _, e := range build.Env {
		expanded, err := tmplCtx.Apply(e)
		if err != nil {
			return fmt.Errorf("failed to expand env %s: %w", e, err)
		}
		env = append(env, expanded)
	}

	// Determine working directory
	dir := build.Dir
	if dir == "" {
		dir, _ = os.Getwd()
	}

	// Detect build tool
	buildTool := build.Builder
	if buildTool == "java" {
		// Auto-detect
		if _, err := os.Stat(filepath.Join(dir, "pom.xml")); err == nil {
			buildTool = "maven"
		} else if _, err := os.Stat(filepath.Join(dir, "build.gradle")); err == nil {
			buildTool = "gradle"
		} else if _, err := os.Stat(filepath.Join(dir, "build.gradle.kts")); err == nil {
			buildTool = "gradle"
		}
	}

	var cmd *exec.Cmd
	switch buildTool {
	case "maven", "mvn":
		args := []string{"package", "-DskipTests"}
		for _, flag := range build.Flags {
			expanded, _ := tmplCtx.Apply(flag)
			args = append(args, expanded)
		}
		cmd = exec.CommandContext(ctx, "mvn", args...)
	case "gradle":
		args := []string{"build", "-x", "test"}
		for _, flag := range build.Flags {
			expanded, _ := tmplCtx.Apply(flag)
			args = append(args, expanded)
		}
		// Use wrapper if available
		if _, err := os.Stat(filepath.Join(dir, "gradlew")); err == nil {
			cmd = exec.CommandContext(ctx, "./gradlew", args...)
		} else {
			cmd = exec.CommandContext(ctx, "gradle", args...)
		}
	default:
		return fmt.Errorf("unknown Java build tool: %s", buildTool)
	}

	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Java build failed: %w", err)
	}

	// Copy JAR to output location
	jarPath := build.Main
	if jarPath == "" {
		// Try to find the JAR
		if buildTool == "maven" || buildTool == "mvn" {
			jarPath = "target/*.jar"
		} else {
			jarPath = "build/libs/*.jar"
		}
	}

	// Expand glob
	matches, err := filepath.Glob(filepath.Join(dir, jarPath))
	if err != nil {
		return fmt.Errorf("failed to find JAR: %w", err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("no JAR found matching %s", jarPath)
	}

	// Copy the first match (usually the main JAR)
	for _, match := range matches {
		if strings.Contains(match, "-sources") || strings.Contains(match, "-javadoc") {
			continue
		}
		if err := copyFile(match, output); err != nil {
			return fmt.Errorf("failed to copy JAR: %w", err)
		}
		break
	}

	log.Info("Built JAR", "output", output)
	return nil
}

// PHPBuilder builds PHP projects (Composer/Phar)
type PHPBuilder struct{}

// NewPHPBuilder creates a new PHP builder
func NewPHPBuilder() *PHPBuilder {
	return &PHPBuilder{}
}

// Supports returns true if this builder supports the given builder type
func (b *PHPBuilder) Supports(builder string) bool {
	return builder == "php" || builder == "composer" || builder == "phar"
}

// Build builds a PHP project
func (b *PHPBuilder) Build(ctx context.Context, build config.Build, target Target, output string, tmplCtx *tmpl.Context) error {
	log.Debug("Building PHP project", "target", target.String(), "output", output)

	// Prepare environment
	env := os.Environ()
	for _, e := range build.Env {
		expanded, err := tmplCtx.Apply(e)
		if err != nil {
			return fmt.Errorf("failed to expand env %s: %w", e, err)
		}
		env = append(env, expanded)
	}

	// Determine working directory
	dir := build.Dir
	if dir == "" {
		dir, _ = os.Getwd()
	}

	// Install dependencies
	installCmd := exec.CommandContext(ctx, "composer", "install", "--no-dev", "--optimize-autoloader")
	installCmd.Dir = dir
	installCmd.Env = env
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		log.Warn("Composer install failed, continuing anyway", "error", err)
	}

	// Build Phar if configured
	if build.Builder == "phar" || strings.HasSuffix(output, ".phar") {
		// Check if box is available
		boxPath := "box"
		if _, err := exec.LookPath("box"); err != nil {
			// Try php box.phar
			boxPath = "php"
		}

		var cmd *exec.Cmd
		if boxPath == "box" {
			cmd = exec.CommandContext(ctx, "box", "compile")
		} else {
			// Use humbug/box or custom phar builder
			cmd = exec.CommandContext(ctx, "php", "-d", "phar.readonly=0", "box.phar", "compile")
		}
		cmd.Dir = dir
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			// Fallback: create phar manually using PHP script
			log.Debug("Box not available, using manual phar creation")
			if err := b.createPharManually(ctx, dir, output, env); err != nil {
				return fmt.Errorf("failed to create Phar: %w", err)
			}
		} else {
			// Copy phar to output
			pharPath := build.Main
			if pharPath == "" {
				pharPath = filepath.Base(dir) + ".phar"
			}
			if err := copyFile(filepath.Join(dir, pharPath), output); err != nil {
				return fmt.Errorf("failed to copy Phar: %w", err)
			}
		}
	} else {
		// Just copy the source
		if err := copyFile(build.Main, output); err != nil {
			return fmt.Errorf("failed to copy PHP source: %w", err)
		}
	}

	log.Info("Built PHP project", "output", output)
	return nil
}

// createPharManually creates a Phar archive using PHP
func (b *PHPBuilder) createPharManually(ctx context.Context, dir, output string, env []string) error {
	pharScript := `<?php
$phar = new Phar($argv[1], 0, basename($argv[1]));
$phar->buildFromDirectory($argv[2]);
$phar->setDefaultStub('index.php');
$phar->compressFiles(Phar::GZ);
echo "Phar created: " . $argv[1] . "\n";
`

	// Create temp script
	scriptPath := filepath.Join(dir, ".create-phar.php")
	if err := os.WriteFile(scriptPath, []byte(pharScript), 0644); err != nil {
		return err
	}
	defer os.Remove(scriptPath)

	cmd := exec.CommandContext(ctx, "php", "-d", "phar.readonly=0", scriptPath, output, dir)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// GetBuilder returns the appropriate builder for a build configuration
func GetBuilder(builderType string) Builder {
	builders := []Builder{
		NewGoBuilder(),
		NewRustBuilder(),
		NewNodeBuilder(),
		NewPythonBuilder(),
		NewJavaBuilder(),
		NewPHPBuilder(),
		NewPrebuiltBuilder(),
	}

	for _, b := range builders {
		if b.Supports(builderType) {
			return b
		}
	}

	return NewGoBuilder() // Default to Go
}
