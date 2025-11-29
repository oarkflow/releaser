/*
Package cmd provides plugin commands for Releaser.
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/plugin"
	"github.com/oarkflow/releaser/internal/tmpl"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage plugins",
	Long: `Manage releaser plugins for custom builders and publishers.

Plugins can be:
- Go plugins (.so files) implementing the Plugin interface
- Executables that communicate via JSON stdin/stdout
- Scripts with shell commands

Plugin directory: ~/.releaser/plugins/`,
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	Long:  `List all installed plugins and their details.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		homeDir, _ := os.UserHomeDir()
		pluginDir := filepath.Join(homeDir, ".releaser", "plugins")

		// Create minimal config for template context
		cfg := &config.Config{ProjectName: "plugins"}
		tmplCtx := tmpl.New(cfg, nil, false, false)
		mgr := plugin.NewManager(pluginDir, tmplCtx)

		if err := mgr.LoadAll(); err != nil {
			log.Warn("Error loading plugins", "error", err)
		}

		plugins := mgr.List()
		if len(plugins) == 0 {
			fmt.Println("No plugins installed")
			fmt.Printf("\nPlugin directory: %s\n", pluginDir)
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTYPE\tVERSION")
		fmt.Fprintln(w, "----\t----\t-------")

		for _, p := range plugins {
			fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name(), p.Type(), p.Version())
		}
		w.Flush()

		return nil
	},
}

var pluginInfoCmd = &cobra.Command{
	Use:   "info [name]",
	Short: "Show plugin details",
	Long:  `Show detailed information about a specific plugin.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		homeDir, _ := os.UserHomeDir()
		pluginDir := filepath.Join(homeDir, ".releaser", "plugins")

		cfg := &config.Config{ProjectName: "plugins"}
		tmplCtx := tmpl.New(cfg, nil, false, false)
		mgr := plugin.NewManager(pluginDir, tmplCtx)

		if err := mgr.LoadAll(); err != nil {
			log.Warn("Error loading plugins", "error", err)
		}

		found := false
		for _, p := range mgr.List() {
			if p.Name() == args[0] {
				found = true
				fmt.Printf("Name:    %s\n", p.Name())
				fmt.Printf("Type:    %s\n", p.Type())
				fmt.Printf("Version: %s\n", p.Version())
				break
			}
		}

		if !found {
			return fmt.Errorf("plugin not found: %s", args[0])
		}

		return nil
	},
}

var pluginDirCmd = &cobra.Command{
	Use:   "dir",
	Short: "Show plugin directory",
	Long:  `Show the plugin directory path.`,
	Run: func(cmd *cobra.Command, args []string) {
		homeDir, _ := os.UserHomeDir()
		pluginDir := filepath.Join(homeDir, ".releaser", "plugins")
		fmt.Println(pluginDir)
	},
}

var pluginNewCmd = &cobra.Command{
	Use:   "new [name]",
	Short: "Create a new plugin template",
	Long: `Create a new plugin template with boilerplate code.

Examples:
  releaser plugin new my-builder --type builder
  releaser plugin new my-publisher --type publisher
  releaser plugin new my-hook --type hook
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		pluginType, _ := cmd.Flags().GetString("type")

		// Create plugin directory
		pluginDir := name
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Generate plugin template based on type
		var template string
		switch pluginType {
		case "builder":
			template = builderTemplate(name)
		case "publisher":
			template = publisherTemplate(name)
		case "hook":
			template = hookTemplate(name)
		default:
			template = builderTemplate(name)
		}

		// Write main.go
		mainPath := filepath.Join(pluginDir, "main.go")
		if err := os.WriteFile(mainPath, []byte(template), 0644); err != nil {
			return fmt.Errorf("failed to write template: %w", err)
		}

		// Write go.mod
		goMod := fmt.Sprintf(`module %s

go 1.21
`, name)
		modPath := filepath.Join(pluginDir, "go.mod")
		if err := os.WriteFile(modPath, []byte(goMod), 0644); err != nil {
			return fmt.Errorf("failed to write go.mod: %w", err)
		}

		log.Info("Plugin template created", "path", pluginDir)
		fmt.Printf("\nBuild with:\n  cd %s && go build -buildmode=plugin -o %s.so\n", name, name)

		return nil
	},
}

func init() {
	pluginNewCmd.Flags().String("type", "builder", "Plugin type: builder, publisher, hook")

	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginInfoCmd)
	pluginCmd.AddCommand(pluginDirCmd)
	pluginCmd.AddCommand(pluginNewCmd)
	rootCmd.AddCommand(pluginCmd)
}

func builderTemplate(name string) string {
	return fmt.Sprintf(`package main

import (
	"context"
	"fmt"
)

// Artifact represents a build artifact
type Artifact struct {
	Name   string
	Path   string
	Type   string
	Goos   string
	Goarch string
	Extra  map[string]interface{}
}

// %sPlugin is a custom builder plugin
type %sPlugin struct{}

// Plugin is the exported symbol
var Plugin = &%sPlugin{}

func (p *%sPlugin) Name() string    { return "%s" }
func (p *%sPlugin) Version() string { return "1.0.0" }
func (p *%sPlugin) Type() string    { return "builder" }

// Build executes the custom build
func (p *%sPlugin) Build(ctx context.Context, cfg map[string]interface{}) ([]Artifact, error) {
	fmt.Println("Running custom builder: %s")

	// Add your build logic here

	return []Artifact{
		{
			Name: "example",
			Path: "./dist/example",
			Type: "binary",
		},
	}, nil
}
`, name, name, name, name, name, name, name, name, name)
}

func publisherTemplate(name string) string {
	return fmt.Sprintf(`package main

import (
	"context"
	"fmt"
)

// Artifact represents a build artifact
type Artifact struct {
	Name   string
	Path   string
	Type   string
	Goos   string
	Goarch string
	Extra  map[string]interface{}
}

// %sPlugin is a custom publisher plugin
type %sPlugin struct{}

// Plugin is the exported symbol
var Plugin = &%sPlugin{}

func (p *%sPlugin) Name() string    { return "%s" }
func (p *%sPlugin) Version() string { return "1.0.0" }
func (p *%sPlugin) Type() string    { return "publisher" }

// Publish publishes artifacts
func (p *%sPlugin) Publish(ctx context.Context, artifacts []Artifact, cfg map[string]interface{}) error {
	fmt.Println("Running custom publisher: %s")

	for _, a := range artifacts {
		fmt.Printf("Publishing: %%s\n", a.Name)
		// Add your publish logic here
	}

	return nil
}
`, name, name, name, name, name, name, name, name)
}

func hookTemplate(name string) string {
	return fmt.Sprintf(`package main

import (
	"context"
	"fmt"
)

// %sPlugin is a custom hook plugin
type %sPlugin struct{}

// Plugin is the exported symbol
var Plugin = &%sPlugin{}

func (p *%sPlugin) Name() string    { return "%s" }
func (p *%sPlugin) Version() string { return "1.0.0" }
func (p *%sPlugin) Type() string    { return "hook" }

// Execute runs the hook
func (p *%sPlugin) Execute(ctx context.Context, event string, data map[string]interface{}) error {
	fmt.Printf("Hook %s triggered for event: %%s\n", event)

	// Available events: before_build, after_build, before_publish, after_publish
	// Add your hook logic here

	return nil
}
`, name, name, name, name, name, name, name)
}
