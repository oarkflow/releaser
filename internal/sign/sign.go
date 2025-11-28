/*
Package sign provides artifact signing functionality for Releaser.
*/
package sign

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/oarkflow/releaser/internal/artifact"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// Signer provides artifact signing capabilities
type Signer struct {
	distDir string
	tmplCtx *tmpl.Context
}

// NewSigner creates a new signer
func NewSigner(distDir string, tmplCtx *tmpl.Context) *Signer {
	return &Signer{
		distDir: distDir,
		tmplCtx: tmplCtx,
	}
}

// Sign signs artifacts based on configuration
func (s *Signer) Sign(ctx context.Context, cfg config.Sign, artifacts []artifact.Artifact) ([]*artifact.Artifact, error) {
	var signed []*artifact.Artifact

	// Filter artifacts based on configuration
	toSign := s.filterArtifacts(cfg, artifacts)
	if len(toSign) == 0 {
		log.Debug("No artifacts to sign")
		return nil, nil
	}

	for _, a := range toSign {
		sig, err := s.signArtifact(ctx, cfg, a)
		if err != nil {
			return nil, fmt.Errorf("failed to sign %s: %w", a.Name, err)
		}
		if sig != nil {
			signed = append(signed, sig)
		}
	}

	return signed, nil
}

// signArtifact signs a single artifact
func (s *Signer) signArtifact(ctx context.Context, cfg config.Sign, a artifact.Artifact) (*artifact.Artifact, error) {
	log.Info("Signing artifact", "name", a.Name)

	// Determine signature file path
	sigPath := cfg.Signature
	if sigPath == "" {
		sigPath = a.Path + ".sig"
	} else {
		sigPath = filepath.Join(s.distDir, sigPath)
	}

	// Apply template to signature path
	tmplCtx := s.tmplCtx.WithArtifact(a.Name, a.Goos, a.Goarch, a.Goarm, a.Goamd64)
	sigPath, err := tmplCtx.Apply(sigPath)
	if err != nil {
		return nil, err
	}

	// Determine signing command
	cmd := cfg.Cmd
	if cmd == "" {
		cmd = "gpg"
	}

	// Build arguments
	args := cfg.Args
	if len(args) == 0 {
		args = []string{"--detach-sign", "--armor", "--output", "${signature}", "${artifact}"}
	}

	// Expand argument templates
	expandedArgs := make([]string, len(args))
	for i, arg := range args {
		expanded := arg
		expanded = strings.ReplaceAll(expanded, "${artifact}", a.Path)
		expanded = strings.ReplaceAll(expanded, "${signature}", sigPath)
		expanded = strings.ReplaceAll(expanded, "${certificate}", cfg.Certificate)

		// Apply template
		expanded, err = tmplCtx.Apply(expanded)
		if err != nil {
			return nil, fmt.Errorf("failed to expand arg %s: %w", arg, err)
		}
		expandedArgs[i] = expanded
	}

	// Prepare environment
	env := os.Environ()
	for _, e := range cfg.Env {
		expanded, err := tmplCtx.Apply(e)
		if err != nil {
			return nil, fmt.Errorf("failed to expand env %s: %w", e, err)
		}
		env = append(env, expanded)
	}

	// Run signing command
	log.Debug("Running sign command", "cmd", cmd, "args", expandedArgs)
	execCmd := exec.CommandContext(ctx, cmd, expandedArgs...)
	execCmd.Env = env

	// Handle stdin
	if cfg.Stdin != "" {
		execCmd.Stdin = strings.NewReader(cfg.Stdin)
	} else if cfg.StdinFile != "" {
		stdinData, err := os.ReadFile(cfg.StdinFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read stdin file: %w", err)
		}
		execCmd.Stdin = bytes.NewReader(stdinData)
	}

	var stdout, stderr bytes.Buffer
	if cfg.Output {
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
	} else {
		execCmd.Stdout = &stdout
		execCmd.Stderr = &stderr
	}

	if err := execCmd.Run(); err != nil {
		return nil, fmt.Errorf("sign command failed: %w\n%s", err, stderr.String())
	}

	return &artifact.Artifact{
		Name:   filepath.Base(sigPath),
		Path:   sigPath,
		Type:   artifact.TypeSignature,
		Goos:   a.Goos,
		Goarch: a.Goarch,
		Extra: map[string]interface{}{
			"signed_artifact": a.Name,
		},
	}, nil
}

// filterArtifacts filters artifacts based on signing configuration
func (s *Signer) filterArtifacts(cfg config.Sign, artifacts []artifact.Artifact) []artifact.Artifact {
	var result []artifact.Artifact

	for _, a := range artifacts {
		// Check artifact type filter
		switch cfg.Artifacts {
		case "all":
			result = append(result, a)
		case "checksum", "checksums":
			if a.Type == artifact.TypeChecksum {
				result = append(result, a)
			}
		case "source":
			if a.Type == artifact.TypeSourceArchive {
				result = append(result, a)
			}
		case "archive", "archives":
			if a.Type == artifact.TypeArchive {
				result = append(result, a)
			}
		case "binary", "binaries":
			if a.Type == artifact.TypeBinary {
				result = append(result, a)
			}
		case "package", "packages":
			if a.Type == artifact.TypePackage || a.Type == artifact.TypeLinuxPackage {
				result = append(result, a)
			}
		default:
			// Default to archives and binaries
			if a.Type == artifact.TypeArchive || a.Type == artifact.TypeBinary {
				result = append(result, a)
			}
		}

		// Check ID filter
		if len(cfg.IDs) > 0 {
			found := false
			for _, id := range cfg.IDs {
				if a.BuildID == id {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
	}

	return result
}

// MacOSSigner provides macOS code signing
type MacOSSigner struct {
	distDir string
	tmplCtx *tmpl.Context
}

// NewMacOSSigner creates a new macOS signer
func NewMacOSSigner(distDir string, tmplCtx *tmpl.Context) *MacOSSigner {
	return &MacOSSigner{
		distDir: distDir,
		tmplCtx: tmplCtx,
	}
}

// SignApp signs a macOS app bundle
func (s *MacOSSigner) SignApp(ctx context.Context, cfg config.AppBundleSign, appPath string) error {
	log.Info("Signing macOS app bundle", "path", appPath)

	args := []string{
		"--sign", cfg.Identity,
		"--timestamp",
	}

	if cfg.Keychain != "" {
		args = append(args, "--keychain", cfg.Keychain)
	}

	if cfg.Entitlements != "" {
		args = append(args, "--entitlements", cfg.Entitlements)
	}

	if cfg.Hardened {
		args = append(args, "--options", "runtime")
	}

	for _, opt := range cfg.Options {
		args = append(args, "--options", opt)
	}

	args = append(args, appPath)

	cmd := exec.CommandContext(ctx, "codesign", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("codesign failed: %w\n%s", err, stderr.String())
	}

	log.Info("App bundle signed successfully")
	return nil
}

// Notarize notarizes a macOS app or DMG
func (s *MacOSSigner) Notarize(ctx context.Context, cfg config.DMGNotarize, path string) error {
	log.Info("Notarizing macOS artifact", "path", path)

	// Submit for notarization
	args := []string{
		"notarytool", "submit",
		path,
		"--apple-id", cfg.AppleID,
		"--password", cfg.Password,
		"--team-id", cfg.TeamID,
		"--wait",
	}

	cmd := exec.CommandContext(ctx, "xcrun", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("notarization failed: %w\n%s", err, stderr.String())
	}

	// Staple the notarization ticket
	if cfg.Staple {
		stapleCmd := exec.CommandContext(ctx, "xcrun", "stapler", "staple", path)
		if err := stapleCmd.Run(); err != nil {
			return fmt.Errorf("stapling failed: %w", err)
		}
	}

	log.Info("Notarization completed successfully")
	return nil
}

// WindowsSigner provides Windows code signing
type WindowsSigner struct {
	distDir string
	tmplCtx *tmpl.Context
}

// NewWindowsSigner creates a new Windows signer
func NewWindowsSigner(distDir string, tmplCtx *tmpl.Context) *WindowsSigner {
	return &WindowsSigner{
		distDir: distDir,
		tmplCtx: tmplCtx,
	}
}

// Sign signs a Windows executable
func (s *WindowsSigner) Sign(ctx context.Context, cert, password, timestampServer, path string) error {
	log.Info("Signing Windows executable", "path", path)

	args := []string{
		"sign",
		"/f", cert,
		"/p", password,
		"/t", timestampServer,
		"/fd", "sha256",
		path,
	}

	cmd := exec.CommandContext(ctx, "signtool", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("signtool failed: %w\n%s", err, stderr.String())
	}

	log.Info("Executable signed successfully")
	return nil
}
