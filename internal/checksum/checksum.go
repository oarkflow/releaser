// Package checksum provides checksum generation for artifacts.
package checksum

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/oarkflow/releaser/internal/artifact"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// Algorithm represents a checksum algorithm.
type Algorithm string

const (
	AlgorithmMD5    Algorithm = "md5"
	AlgorithmSHA1   Algorithm = "sha1"
	AlgorithmSHA256 Algorithm = "sha256"
	AlgorithmSHA512 Algorithm = "sha512"
)

// Generator generates checksums for artifacts.
type Generator struct {
	config      config.Checksum
	distDir     string
	manager     *artifact.Manager
	templateCtx *tmpl.Context
}

// NewGenerator creates a new checksum generator.
func NewGenerator(cfg config.Checksum, distDir string, manager *artifact.Manager, templateCtx *tmpl.Context) *Generator {
	return &Generator{
		config:      cfg,
		distDir:     distDir,
		manager:     manager,
		templateCtx: templateCtx,
	}
}

// Run generates checksums for all artifacts.
func (g *Generator) Run() error {
	if g.config.Disable {
		log.Info("Skipping checksum generation")
		return nil
	}

	log.Info("Generating checksums")

	algorithm := Algorithm(g.config.Algorithm)
	if algorithm == "" {
		algorithm = AlgorithmSHA256
	}

	// Get all artifacts that should have checksums
	artifacts := g.manager.List()
	if len(artifacts) == 0 {
		log.Warn("No artifacts to checksum")
		return nil
	}

	// Filter artifacts by type
	var checksumArtifacts []artifact.Artifact
	for _, a := range artifacts {
		if shouldChecksum(a) {
			checksumArtifacts = append(checksumArtifacts, a)
		}
	}

	if len(checksumArtifacts) == 0 {
		log.Warn("No artifacts match checksum criteria")
		return nil
	}

	// Generate checksums
	checksums := make(map[string]string)
	for _, a := range checksumArtifacts {
		sum, err := g.calculateChecksum(a.Path, algorithm)
		if err != nil {
			return fmt.Errorf("failed to calculate checksum for %s: %w", a.Name, err)
		}
		checksums[a.Name] = sum
		log.Debug("Generated checksum", "artifact", a.Name, "algorithm", algorithm, "checksum", sum[:16]+"...")
	}

	// Write checksum file
	checksumFile := g.config.NameTemplate
	if checksumFile == "" {
		checksumFile = "checksums.txt"
	}

	// Apply template to filename
	if g.templateCtx != nil {
		expandedFile, err := g.templateCtx.Apply(checksumFile)
		if err != nil {
			log.Warn("Failed to apply template to checksum filename, using as-is", "template", checksumFile, "error", err)
		} else {
			checksumFile = expandedFile
		}
	}

	checksumPath := filepath.Join(g.distDir, checksumFile)
	if err := g.writeChecksumFile(checksumPath, checksums, algorithm); err != nil {
		return fmt.Errorf("failed to write checksum file: %w", err)
	}

	// Add checksum file as artifact
	g.manager.Add(artifact.Artifact{
		Name: checksumFile,
		Path: checksumPath,
		Type: artifact.TypeChecksum,
		Extra: map[string]interface{}{
			"algorithm": string(algorithm),
		},
	})

	log.Info("Checksums generated", "file", checksumFile, "count", len(checksums))
	return nil
}

// calculateChecksum calculates checksum for a file.
func (g *Generator) calculateChecksum(path string, algorithm Algorithm) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var h hash.Hash
	switch algorithm {
	case AlgorithmMD5:
		h = md5.New()
	case AlgorithmSHA1:
		h = sha1.New()
	case AlgorithmSHA256:
		h = sha256.New()
	case AlgorithmSHA512:
		h = sha512.New()
	default:
		return "", fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// writeChecksumFile writes checksums to a file.
func (g *Generator) writeChecksumFile(path string, checksums map[string]string, algorithm Algorithm) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Sort keys for consistent output
	var names []string
	for name := range checksums {
		names = append(names, name)
	}

	// Write in format: checksum  filename
	for _, name := range names {
		sum := checksums[name]
		_, err := fmt.Fprintf(f, "%s  %s\n", sum, name)
		if err != nil {
			return err
		}
	}

	return nil
}

// shouldChecksum returns true if the artifact should have a checksum.
func shouldChecksum(a artifact.Artifact) bool {
	switch a.Type {
	case artifact.TypeArchive,
		artifact.TypeBinary,
		artifact.TypeLinuxPackage,
		artifact.TypeDockerImage,
		artifact.TypeChecksum:
		return true
	default:
		return false
	}
}

// CalculateForFile calculates checksum for a single file.
func CalculateForFile(path string, algorithm Algorithm) (string, error) {
	g := &Generator{}
	return g.calculateChecksum(path, algorithm)
}

// VerifyChecksum verifies a file against an expected checksum.
func VerifyChecksum(path string, expected string, algorithm Algorithm) (bool, error) {
	actual, err := CalculateForFile(path, algorithm)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(actual, expected), nil
}
