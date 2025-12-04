package docker

import (
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/oarkflow/releaser/internal/config"
)

func ExportAll(exports []config.DockerExportConfig) error {
	for _, e := range exports {
		if err := exportOne(e); err != nil {
			return fmt.Errorf("docker export %s: %w", e.ID, err)
		}
	}
	return nil
}

func exportOne(e config.DockerExportConfig) error {
	if e.Image == "" {
		return fmt.Errorf("image is required")
	}
	if e.Output == "" {
		return fmt.Errorf("output is required")
	}
	format := formatOrDefault(e.Format)
	if err := os.MkdirAll(filepath.Dir(e.Output), 0o755); err != nil {
		return err
	}
	fmt.Printf("→ Exporting docker image %s to %s (%s)\n", e.Image, e.Output, format)

	cmd := exec.Command("docker", "save", e.Image)
	cmd.Stderr = os.Stderr

	switch format {
	case "tar":
		out, err := os.Create(e.Output)
		if err != nil {
			return err
		}
		defer out.Close()
		cmd.Stdout = out
	case "tar.gz":
		out, err := os.Create(e.Output)
		if err != nil {
			return err
		}
		defer out.Close()
		gz := gzip.NewWriter(out)
		defer gz.Close()
		cmd.Stdout = gz
	default:
		return fmt.Errorf("unsupported docker export format: %s", format)
	}
	return cmd.Run()
}

func formatOrDefault(f string) string {
	if f == "" {
		return "tar"
	}
	return strings.ToLower(f)
}
