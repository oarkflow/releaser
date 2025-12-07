/*
Package archive provides archive creation functionality for Releaser.
*/
package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/oarkflow/releaser/internal/artifact"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// Creator creates archives
type Creator struct {
	distDir string
	tmplCtx *tmpl.Context
}

// NewCreator creates a new archive creator
func NewCreator(distDir string, tmplCtx *tmpl.Context) *Creator {
	return &Creator{
		distDir: distDir,
		tmplCtx: tmplCtx,
	}
}

// Create creates an archive from artifacts
func (c *Creator) Create(cfg config.Archive, artifacts []artifact.Artifact) (*artifact.Artifact, error) {
	if len(artifacts) == 0 {
		return nil, fmt.Errorf("no artifacts to archive")
	}

	// Get target info from first artifact
	first := artifacts[0]
	goos := first.Goos
	goarch := first.Goarch

	// Determine format
	format := cfg.Format
	if format == "" {
		format = "tar.gz"
	}

	// Check for format overrides
	for _, override := range cfg.FormatOverrides {
		if override.Goos == goos {
			format = override.Format
			break
		}
	}

	// Apply name template
	nameTemplate := cfg.NameTemplate
	if nameTemplate == "" {
		nameTemplate = "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
	}

	// Create template context with artifact info
	ctx := c.tmplCtx.WithArtifact(first.Name, goos, goarch, first.Goarm, first.Goamd64)
	name, err := ctx.Apply(nameTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to apply name template: %w", err)
	}

	// Add extension
	var ext string
	switch format {
	case "tar.gz", "tgz":
		ext = ".tar.gz"
	case "tar.xz", "txz":
		ext = ".tar.xz"
	case "tar":
		ext = ".tar"
	case "zip":
		ext = ".zip"
	case "gz", "gzip":
		ext = ".gz"
	case "binary":
		ext = ""
	default:
		ext = "." + format
	}

	archivePath := filepath.Join(c.distDir, name+ext)

	log.Info("Creating archive", "path", archivePath, "format", format)

	// Run before hooks
	for _, hook := range cfg.Hooks.Before {
		log.Debug("Running archive before hook", "cmd", hook.Cmd)
		// TODO: Implement hook execution
	}

	// Create archive based on format
	switch format {
	case "tar.gz", "tgz":
		if err := c.createTarGz(archivePath, cfg, artifacts); err != nil {
			return nil, err
		}
	case "tar.xz", "txz":
		if err := c.createTarXz(archivePath, cfg, artifacts); err != nil {
			return nil, err
		}
	case "tar":
		if err := c.createTar(archivePath, cfg, artifacts); err != nil {
			return nil, err
		}
	case "zip":
		if err := c.createZip(archivePath, cfg, artifacts); err != nil {
			return nil, err
		}
	case "binary":
		if err := c.copyBinary(archivePath, artifacts); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported archive format: %s", format)
	}

	// Run after hooks
	for _, hook := range cfg.Hooks.After {
		log.Debug("Running archive after hook", "cmd", hook.Cmd)
		// TODO: Implement hook execution
	}

	return &artifact.Artifact{
		Name:    filepath.Base(archivePath),
		Path:    archivePath,
		Type:    artifact.TypeArchive,
		Goos:    goos,
		Goarch:  goarch,
		Goarm:   first.Goarm,
		Goamd64: first.Goamd64,
		Extra: map[string]interface{}{
			"format": format,
		},
	}, nil
}

// createTarGz creates a tar.gz archive
func (c *Creator) createTarGz(path string, cfg config.Archive, artifacts []artifact.Artifact) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Determine wrapper directory
	wrapDir := ""
	if cfg.WrapInDirectory != "" {
		wrapDir, _ = c.tmplCtx.Apply(cfg.WrapInDirectory)
	}

	// Add binaries
	for _, a := range artifacts {
		if err := c.addToTar(tw, a.Path, filepath.Join(wrapDir, a.Name), nil); err != nil {
			return err
		}
	}

	// Add extra files
	for _, f := range cfg.Files {
		matches, err := filepath.Glob(f.Src)
		if err != nil {
			continue
		}
		for _, match := range matches {
			dst := f.Dst
			if dst == "" {
				dst = filepath.Base(match)
			}
			if f.StripParent {
				dst = filepath.Base(match)
			}
			if err := c.addToTar(tw, match, filepath.Join(wrapDir, dst), &f.Info); err != nil {
				log.Warn("Failed to add file to archive", "file", match, "error", err)
			}
		}
	}

	return nil
}

// createTarXz creates a tar.xz archive
func (c *Creator) createTarXz(path string, cfg config.Archive, artifacts []artifact.Artifact) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use external xz command for compression
	// First create a tar in memory, then pipe through xz
	pr, pw := io.Pipe()

	// Start xz writer in goroutine
	errCh := make(chan error, 1)
	go func() {
		defer pw.Close()
		tw := tar.NewWriter(pw)
		defer tw.Close()

		// Determine wrapper directory
		wrapDir := ""
		if cfg.WrapInDirectory != "" {
			wrapDir, _ = c.tmplCtx.Apply(cfg.WrapInDirectory)
		}

		// Add binaries
		for _, a := range artifacts {
			if err := c.addToTar(tw, a.Path, filepath.Join(wrapDir, a.Name), nil); err != nil {
				errCh <- err
				return
			}
		}

		// Add extra files
		for _, f := range cfg.Files {
			matches, err := filepath.Glob(f.Src)
			if err != nil {
				continue
			}
			for _, match := range matches {
				dst := f.Dst
				if dst == "" {
					dst = filepath.Base(match)
				}
				if f.StripParent {
					dst = filepath.Base(match)
				}
				if err := c.addToTar(tw, match, filepath.Join(wrapDir, dst), &f.Info); err != nil {
					log.Warn("Failed to add file to archive", "file", match, "error", err)
				}
			}
		}
		errCh <- nil
	}()

	// Compress with xz using exec (Go stdlib doesn't have xz support)
	// Note: xz may not be available on Windows by default
	cmd := exec.Command("xz", "-c")
	cmd.Stdin = pr
	cmd.Stdout = file

	if err := cmd.Run(); err != nil {
		// Provide helpful error message for Windows users
		if strings.Contains(err.Error(), "executable file not found") || strings.Contains(err.Error(), "not recognized") {
			return fmt.Errorf("xz compression failed: xz tool not found. On Windows, install xz from https://tukaani.org/xz/ or use 'tar.gz' or 'zip' format instead: %w", err)
		}
		return fmt.Errorf("xz compression failed: %w", err)
	}

	// Wait for tar writer to finish
	if err := <-errCh; err != nil {
		return fmt.Errorf("tar creation failed: %w", err)
	}

	return nil
}

// createTar creates a tar archive
func (c *Creator) createTar(path string, cfg config.Archive, artifacts []artifact.Artifact) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	tw := tar.NewWriter(file)
	defer tw.Close()

	wrapDir := ""
	if cfg.WrapInDirectory != "" {
		wrapDir, _ = c.tmplCtx.Apply(cfg.WrapInDirectory)
	}

	for _, a := range artifacts {
		if err := c.addToTar(tw, a.Path, filepath.Join(wrapDir, a.Name), nil); err != nil {
			return err
		}
	}

	for _, f := range cfg.Files {
		matches, err := filepath.Glob(f.Src)
		if err != nil {
			continue
		}
		for _, match := range matches {
			dst := f.Dst
			if dst == "" {
				dst = filepath.Base(match)
			}
			if err := c.addToTar(tw, match, filepath.Join(wrapDir, dst), &f.Info); err != nil {
				log.Warn("Failed to add file to archive", "file", match, "error", err)
			}
		}
	}

	return nil
}

// createZip creates a zip archive
func (c *Creator) createZip(path string, cfg config.Archive, artifacts []artifact.Artifact) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	wrapDir := ""
	if cfg.WrapInDirectory != "" {
		wrapDir, _ = c.tmplCtx.Apply(cfg.WrapInDirectory)
	}

	for _, a := range artifacts {
		if err := c.addToZip(zw, a.Path, filepath.Join(wrapDir, a.Name)); err != nil {
			return err
		}
	}

	for _, f := range cfg.Files {
		matches, err := filepath.Glob(f.Src)
		if err != nil {
			continue
		}
		for _, match := range matches {
			dst := f.Dst
			if dst == "" {
				dst = filepath.Base(match)
			}
			if err := c.addToZip(zw, match, filepath.Join(wrapDir, dst)); err != nil {
				log.Warn("Failed to add file to archive", "file", match, "error", err)
			}
		}
	}

	return nil
}

// copyBinary copies the binary without archiving
func (c *Creator) copyBinary(path string, artifacts []artifact.Artifact) error {
	if len(artifacts) != 1 {
		return fmt.Errorf("binary format requires exactly one artifact")
	}

	src, err := os.Open(artifacts[0].Path)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(path)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	// Preserve permissions
	info, err := src.Stat()
	if err != nil {
		return err
	}
	return os.Chmod(path, info.Mode())
}

// addToTar adds a file to a tar archive
func (c *Creator) addToTar(tw *tar.Writer, src, dst string, info *config.ArchiveFileInfo) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(stat, "")
	if err != nil {
		return err
	}

	header.Name = strings.TrimPrefix(dst, "/")

	// Apply custom file info
	if info != nil {
		if info.Mode != 0 {
			header.Mode = int64(info.Mode)
		}
		if info.Owner != "" {
			header.Uname = info.Owner
		}
		if info.Group != "" {
			header.Gname = info.Group
		}
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if !stat.IsDir() {
		if _, err := io.Copy(tw, file); err != nil {
			return err
		}
	}

	return nil
}

// addToZip adds a file to a zip archive
func (c *Creator) addToZip(zw *zip.Writer, src, dst string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(stat)
	if err != nil {
		return err
	}

	header.Name = strings.TrimPrefix(dst, "/")
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	if !stat.IsDir() {
		if _, err := io.Copy(writer, file); err != nil {
			return err
		}
	}

	return nil
}
