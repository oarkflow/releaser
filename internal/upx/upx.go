// Package upx provides binary compression using UPX.
package upx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/oarkflow/releaser/internal/artifact"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// Compressor compresses binaries using UPX.
type Compressor struct {
	config  config.UPX
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewCompressor creates a new UPX compressor.
func NewCompressor(cfg config.UPX, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *Compressor {
	return &Compressor{
		config:  cfg,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// Run compresses binaries using UPX.
func (c *Compressor) Run(ctx context.Context) error {
	if c.config.Skip == "true" {
		log.Info("Skipping UPX compression")
		return nil
	}

	log.Info("Compressing binaries with UPX")

	// Check if UPX is available
	if _, err := exec.LookPath("upx"); err != nil {
		if c.config.FailOnError {
			return fmt.Errorf("UPX not found: %w", err)
		}
		log.Warn("UPX not found, skipping compression")
		return nil
	}

	// Get binaries to compress
	binaries := c.manager.Filter(artifact.ByType(artifact.TypeBinary))

	if len(binaries) == 0 {
		log.Warn("No binaries found for compression")
		return nil
	}

	for _, binary := range binaries {
		// Skip if binary is for an architecture UPX doesn't support well
		if !isUPXCompatible(binary.Goos, binary.Goarch) {
			log.Debug("Skipping UPX for incompatible platform", "goos", binary.Goos, "goarch", binary.Goarch)
			continue
		}

		if err := c.compress(ctx, binary); err != nil {
			if c.config.FailOnError {
				return fmt.Errorf("failed to compress %s: %w", binary.Name, err)
			}
			log.Warn("Failed to compress binary", "name", binary.Name, "error", err)
		}
	}

	log.Info("UPX compression complete")
	return nil
}

// compress compresses a single binary.
func (c *Compressor) compress(ctx context.Context, binary artifact.Artifact) error {
	log.Debug("Compressing binary", "name", binary.Name)

	// Get original size
	originalInfo, err := os.Stat(binary.Path)
	if err != nil {
		return fmt.Errorf("failed to stat binary: %w", err)
	}
	originalSize := originalInfo.Size()

	// Build UPX command
	args := []string{}

	// Compression level
	level := c.config.CompressionLevel
	if level == 0 {
		args = append(args, "--best")
	} else if level > 0 && level <= 9 {
		args = append(args, fmt.Sprintf("-%d", level))
	}

	// Add extra args
	args = append(args, c.config.ExtraArgs...)

	// Add binary path
	args = append(args, binary.Path)

	cmd := exec.CommandContext(ctx, "upx", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("UPX failed: %w", err)
	}

	// Get compressed size
	compressedInfo, err := os.Stat(binary.Path)
	if err != nil {
		return fmt.Errorf("failed to stat compressed binary: %w", err)
	}
	compressedSize := compressedInfo.Size()

	ratio := float64(compressedSize) / float64(originalSize) * 100
	log.Info("Binary compressed",
		"name", binary.Name,
		"original", formatSize(originalSize),
		"compressed", formatSize(compressedSize),
		"ratio", fmt.Sprintf("%.1f%%", ratio),
	)

	return nil
}

// isUPXCompatible checks if UPX supports the platform.
func isUPXCompatible(goos, goarch string) bool {
	// UPX works best with linux and windows
	// macOS binaries have issues with code signing after UPX
	switch goos {
	case "linux":
		return goarch == "amd64" || goarch == "386" || goarch == "arm" || goarch == "arm64"
	case "windows":
		return goarch == "amd64" || goarch == "386"
	default:
		return false
	}
}

// formatSize formats bytes as human readable size.
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// MultiCompressor compresses binaries for multiple configurations.
type MultiCompressor struct {
	configs []config.UPX
	tmplCtx *tmpl.Context
	manager *artifact.Manager
	distDir string
}

// NewMultiCompressor creates a multi-config UPX compressor.
func NewMultiCompressor(configs []config.UPX, tmplCtx *tmpl.Context, manager *artifact.Manager, distDir string) *MultiCompressor {
	return &MultiCompressor{
		configs: configs,
		tmplCtx: tmplCtx,
		manager: manager,
		distDir: distDir,
	}
}

// RunAll compresses binaries for all configurations.
func (m *MultiCompressor) RunAll(ctx context.Context) error {
	for i, cfg := range m.configs {
		log.Info("Running UPX compression", "index", i+1, "total", len(m.configs))
		compressor := NewCompressor(cfg, m.tmplCtx, m.manager, m.distDir)
		if err := compressor.Run(ctx); err != nil {
			return err
		}
	}
	return nil
}

// DecompressFile decompresses a UPX-compressed file.
func DecompressFile(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "upx", "-d", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// TestFile tests if a file is UPX-compressed.
func TestFile(ctx context.Context, path string) (bool, error) {
	cmd := exec.CommandContext(ctx, "upx", "-t", path)
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return false, nil // Not compressed or invalid
			}
		}
		return false, err
	}
	return true, nil
}

// BackupAndCompress creates a backup before compressing.
func BackupAndCompress(ctx context.Context, path string, level string) error {
	backupPath := path + ".uncompressed"

	// Copy original
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(backupPath, data, 0755); err != nil {
		return err
	}

	// Compress
	args := []string{level, path}
	cmd := exec.CommandContext(ctx, "upx", args...)
	if err := cmd.Run(); err != nil {
		// Restore on failure
		os.Rename(backupPath, path)
		return err
	}

	return nil
}

// GetVersion returns the UPX version.
func GetVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "upx", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse version from first line
	lines := filepath.SplitList(string(output))
	if len(lines) > 0 {
		return lines[0], nil
	}

	return string(output), nil
}
