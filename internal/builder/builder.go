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
	"strings"

	"github.com/charmbracelet/log"

	"github.com/oarkflow/releaser/internal/config"
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

	// Add build-specific environment
	for _, e := range build.Env {
		expanded, err := tmplCtx.Apply(e)
		if err != nil {
			return fmt.Errorf("failed to expand env %s: %w", e, err)
		}
		env = append(env, expanded)
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

	// Run build
	log.Debug("Running go build", "args", args)
	cmd := exec.CommandContext(ctx, goBinary, args...)
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

// GetBuilder returns the appropriate builder for a build configuration
func GetBuilder(builderType string) Builder {
	builders := []Builder{
		NewGoBuilder(),
		NewRustBuilder(),
		NewNodeBuilder(),
		NewPythonBuilder(),
		NewPrebuiltBuilder(),
	}

	for _, b := range builders {
		if b.Supports(builderType) {
			return b
		}
	}

	return NewGoBuilder() // Default to Go
}
