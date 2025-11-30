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

// CosignSigner provides cosign-based signing (including keyless with OIDC)
type CosignSigner struct {
	distDir string
	tmplCtx *tmpl.Context
}

// NewCosignSigner creates a new cosign signer
func NewCosignSigner(distDir string, tmplCtx *tmpl.Context) *CosignSigner {
	return &CosignSigner{
		distDir: distDir,
		tmplCtx: tmplCtx,
	}
}

// CosignConfig represents cosign signing configuration
type CosignConfig struct {
	// Keyless uses OIDC-based keyless signing (Sigstore)
	Keyless bool
	// KeyRef is the path to the private key (for key-based signing)
	KeyRef string
	// Certificate is the path to the certificate
	Certificate string
	// CertificateChain is the path to the certificate chain
	CertificateChain string
	// OIDC options for keyless signing
	OIDCIssuer   string
	OIDCClientID string
	// Rekor URL for transparency log
	RekorURL string
	// FulcioURL for certificate authority
	FulcioURL string
	// Annotations to add to the signature
	Annotations map[string]string
	// RecursiveSign for signing all images in a multi-arch manifest
	Recursive bool
}

// SignContainer signs a container image using cosign
func (s *CosignSigner) SignContainer(ctx context.Context, cfg CosignConfig, imageRef string) error {
	log.Info("Signing container image with cosign", "image", imageRef)

	// Check if cosign is available
	if _, err := exec.LookPath("cosign"); err != nil {
		return fmt.Errorf("cosign not found in PATH: %w", err)
	}

	args := []string{"sign"}

	if cfg.Keyless {
		// Keyless signing using OIDC
		args = append(args, "--yes") // Skip confirmation prompts
		if cfg.OIDCIssuer != "" {
			args = append(args, "--oidc-issuer", cfg.OIDCIssuer)
		}
		if cfg.OIDCClientID != "" {
			args = append(args, "--oidc-client-id", cfg.OIDCClientID)
		}
		if cfg.FulcioURL != "" {
			args = append(args, "--fulcio-url", cfg.FulcioURL)
		}
		if cfg.RekorURL != "" {
			args = append(args, "--rekor-url", cfg.RekorURL)
		}
	} else if cfg.KeyRef != "" {
		// Key-based signing
		args = append(args, "--key", cfg.KeyRef)
		if cfg.Certificate != "" {
			args = append(args, "--certificate", cfg.Certificate)
		}
		if cfg.CertificateChain != "" {
			args = append(args, "--certificate-chain", cfg.CertificateChain)
		}
	}

	if cfg.Recursive {
		args = append(args, "--recursive")
	}

	// Add annotations
	for k, v := range cfg.Annotations {
		args = append(args, "-a", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, imageRef)

	cmd := exec.CommandContext(ctx, "cosign", args...)
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cosign sign failed: %w\nstderr: %s", err, stderr.String())
	}

	log.Info("Container image signed successfully", "image", imageRef)
	return nil
}

// SignBlob signs a blob (file) using cosign
func (s *CosignSigner) SignBlob(ctx context.Context, cfg CosignConfig, blobPath string) (string, error) {
	log.Info("Signing blob with cosign", "path", blobPath)

	// Check if cosign is available
	if _, err := exec.LookPath("cosign"); err != nil {
		return "", fmt.Errorf("cosign not found in PATH: %w", err)
	}

	sigPath := blobPath + ".sig"
	certPath := blobPath + ".pem"

	args := []string{"sign-blob"}

	if cfg.Keyless {
		args = append(args, "--yes")
		args = append(args, "--output-signature", sigPath)
		args = append(args, "--output-certificate", certPath)
		if cfg.OIDCIssuer != "" {
			args = append(args, "--oidc-issuer", cfg.OIDCIssuer)
		}
		if cfg.OIDCClientID != "" {
			args = append(args, "--oidc-client-id", cfg.OIDCClientID)
		}
		if cfg.FulcioURL != "" {
			args = append(args, "--fulcio-url", cfg.FulcioURL)
		}
		if cfg.RekorURL != "" {
			args = append(args, "--rekor-url", cfg.RekorURL)
		}
	} else if cfg.KeyRef != "" {
		args = append(args, "--key", cfg.KeyRef)
		args = append(args, "--output-signature", sigPath)
	}

	args = append(args, blobPath)

	cmd := exec.CommandContext(ctx, "cosign", args...)
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("cosign sign-blob failed: %w\nstderr: %s", err, stderr.String())
	}

	log.Info("Blob signed successfully", "signature", sigPath)
	return sigPath, nil
}

// VerifyContainer verifies a container image signature
func (s *CosignSigner) VerifyContainer(ctx context.Context, cfg CosignConfig, imageRef string) error {
	log.Info("Verifying container image with cosign", "image", imageRef)

	args := []string{"verify"}

	if cfg.Keyless {
		// For keyless, we need to specify the expected issuer/subject
		if cfg.OIDCIssuer != "" {
			args = append(args, "--certificate-oidc-issuer", cfg.OIDCIssuer)
		}
	} else if cfg.KeyRef != "" {
		args = append(args, "--key", cfg.KeyRef)
	}

	args = append(args, imageRef)

	cmd := exec.CommandContext(ctx, "cosign", args...)
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cosign verify failed: %w\nstderr: %s", err, stderr.String())
	}

	log.Info("Container image verified successfully", "image", imageRef)
	return nil
}

// AttachSBOM attaches an SBOM to a container image
func (s *CosignSigner) AttachSBOM(ctx context.Context, sbomPath, sbomType, imageRef string) error {
	log.Info("Attaching SBOM to container image", "image", imageRef, "sbom", sbomPath)

	args := []string{
		"attach", "sbom",
		"--sbom", sbomPath,
		"--type", sbomType, // spdx, cyclonedx, etc.
		imageRef,
	}

	cmd := exec.CommandContext(ctx, "cosign", args...)
	cmd.Env = os.Environ()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cosign attach sbom failed: %w\nstderr: %s", err, stderr.String())
	}

	log.Info("SBOM attached successfully", "image", imageRef)
	return nil
}

// AttestContainer creates an in-toto attestation for a container image
func (s *CosignSigner) AttestContainer(ctx context.Context, cfg CosignConfig, predicatePath, predicateType, imageRef string) error {
	log.Info("Creating attestation for container image", "image", imageRef)

	args := []string{
		"attest",
		"--predicate", predicatePath,
		"--type", predicateType,
	}

	if cfg.Keyless {
		args = append(args, "--yes")
		if cfg.OIDCIssuer != "" {
			args = append(args, "--oidc-issuer", cfg.OIDCIssuer)
		}
	} else if cfg.KeyRef != "" {
		args = append(args, "--key", cfg.KeyRef)
	}

	args = append(args, imageRef)

	cmd := exec.CommandContext(ctx, "cosign", args...)
	cmd.Env = os.Environ()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cosign attest failed: %w\nstderr: %s", err, stderr.String())
	}

	log.Info("Attestation created successfully", "image", imageRef)
	return nil
}
