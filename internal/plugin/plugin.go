// Package plugin provides a plugin system for custom builders and publishers.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"runtime"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/oarkflow/releaser/internal/artifact"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// Plugin interface for all plugin types
type Plugin interface {
	// Name returns the plugin name
	Name() string
	// Version returns the plugin version
	Version() string
	// Type returns the plugin type (builder, publisher, hook)
	Type() string
}

// BuilderPlugin interface for custom builders
type BuilderPlugin interface {
	Plugin
	// Build executes the build
	Build(ctx context.Context, cfg map[string]interface{}) ([]artifact.Artifact, error)
}

// PublisherPlugin interface for custom publishers
type PublisherPlugin interface {
	Plugin
	// Publish publishes artifacts
	Publish(ctx context.Context, artifacts []artifact.Artifact, cfg map[string]interface{}) error
}

// HookPlugin interface for lifecycle hooks
type HookPlugin interface {
	Plugin
	// Execute runs the hook
	Execute(ctx context.Context, event string, data map[string]interface{}) error
}

// Manager manages plugin loading and execution
type Manager struct {
	plugins    map[string]Plugin
	builders   map[string]BuilderPlugin
	publishers map[string]PublisherPlugin
	hooks      map[string][]HookPlugin
	pluginDir  string
	tmplCtx    *tmpl.Context
}

// NewManager creates a new plugin manager
func NewManager(pluginDir string, tmplCtx *tmpl.Context) *Manager {
	if pluginDir == "" {
		homeDir, _ := os.UserHomeDir()
		pluginDir = filepath.Join(homeDir, ".releaser", "plugins")
	}

	return &Manager{
		plugins:    make(map[string]Plugin),
		builders:   make(map[string]BuilderPlugin),
		publishers: make(map[string]PublisherPlugin),
		hooks:      make(map[string][]HookPlugin),
		pluginDir:  pluginDir,
		tmplCtx:    tmplCtx,
	}
}

// LoadAll loads all plugins from the plugin directory
func (m *Manager) LoadAll() error {
	if _, err := os.Stat(m.pluginDir); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		return fmt.Errorf("failed to read plugin directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		path := filepath.Join(m.pluginDir, name)

		// Load based on extension
		if strings.HasSuffix(name, ".so") || strings.HasSuffix(name, ".dylib") {
			if err := m.loadGoPlugin(path); err != nil {
				log.Warn("Failed to load Go plugin", "path", path, "error", err)
			}
		} else if isExecutable(path) {
			if err := m.loadExecPlugin(path); err != nil {
				log.Warn("Failed to load exec plugin", "path", path, "error", err)
			}
		}
	}

	log.Info("Loaded plugins", "count", len(m.plugins))
	return nil
}

// loadGoPlugin loads a Go plugin (.so file)
func (m *Manager) loadGoPlugin(path string) error {
	p, err := plugin.Open(path)
	if err != nil {
		return err
	}

	// Look for Plugin symbol
	sym, err := p.Lookup("Plugin")
	if err != nil {
		return fmt.Errorf("plugin missing Plugin symbol: %w", err)
	}

	plug, ok := sym.(Plugin)
	if !ok {
		return fmt.Errorf("Plugin symbol is not a Plugin interface")
	}

	return m.register(plug)
}

// loadExecPlugin loads an executable plugin
func (m *Manager) loadExecPlugin(path string) error {
	// Query plugin info
	cmd := exec.Command(path, "info")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to query plugin info: %w", err)
	}

	var info struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Type    string `json:"type"`
	}
	if err := json.Unmarshal(output, &info); err != nil {
		return fmt.Errorf("failed to parse plugin info: %w", err)
	}

	plug := &execPlugin{
		name:    info.Name,
		version: info.Version,
		typ:     info.Type,
		path:    path,
	}

	return m.register(plug)
}

// register registers a plugin
func (m *Manager) register(plug Plugin) error {
	name := plug.Name()
	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("plugin already registered: %s", name)
	}

	m.plugins[name] = plug

	switch p := plug.(type) {
	case BuilderPlugin:
		m.builders[name] = p
	case PublisherPlugin:
		m.publishers[name] = p
	case HookPlugin:
		m.hooks[p.Type()] = append(m.hooks[p.Type()], p)
	}

	log.Debug("Registered plugin", "name", name, "type", plug.Type(), "version", plug.Version())
	return nil
}

// GetBuilder returns a builder plugin by name
func (m *Manager) GetBuilder(name string) (BuilderPlugin, bool) {
	b, ok := m.builders[name]
	return b, ok
}

// GetPublisher returns a publisher plugin by name
func (m *Manager) GetPublisher(name string) (PublisherPlugin, bool) {
	p, ok := m.publishers[name]
	return p, ok
}

// RunHooks runs all hooks for an event
func (m *Manager) RunHooks(ctx context.Context, event string, data map[string]interface{}) error {
	hooks := m.hooks[event]
	for _, hook := range hooks {
		if err := hook.Execute(ctx, event, data); err != nil {
			return fmt.Errorf("hook %s failed: %w", hook.Name(), err)
		}
	}
	return nil
}

// List returns all registered plugins
func (m *Manager) List() []Plugin {
	plugins := make([]Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		plugins = append(plugins, p)
	}
	return plugins
}

// execPlugin wraps an executable as a plugin
type execPlugin struct {
	name    string
	version string
	typ     string
	path    string
}

func (p *execPlugin) Name() string    { return p.name }
func (p *execPlugin) Version() string { return p.version }
func (p *execPlugin) Type() string    { return p.typ }

func (p *execPlugin) Build(ctx context.Context, cfg map[string]interface{}) ([]artifact.Artifact, error) {
	input, _ := json.Marshal(cfg)
	cmd := exec.CommandContext(ctx, p.path, "build")
	cmd.Stdin = strings.NewReader(string(input))

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var artifacts []artifact.Artifact
	if err := json.Unmarshal(output, &artifacts); err != nil {
		return nil, err
	}

	return artifacts, nil
}

func (p *execPlugin) Publish(ctx context.Context, artifacts []artifact.Artifact, cfg map[string]interface{}) error {
	input, _ := json.Marshal(map[string]interface{}{
		"artifacts": artifacts,
		"config":    cfg,
	})

	cmd := exec.CommandContext(ctx, p.path, "publish")
	cmd.Stdin = strings.NewReader(string(input))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (p *execPlugin) Execute(ctx context.Context, event string, data map[string]interface{}) error {
	input, _ := json.Marshal(data)
	cmd := exec.CommandContext(ctx, p.path, "hook", event)
	cmd.Stdin = strings.NewReader(string(input))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// isExecutable checks if a file is executable
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&0111 != 0
}

// PluginConfig defines plugin configuration in releaser.yaml
type PluginConfig struct {
	Name    string                 `yaml:"name"`
	Path    string                 `yaml:"path,omitempty"`
	Enabled bool                   `yaml:"enabled,omitempty"`
	Config  map[string]interface{} `yaml:"config,omitempty"`
}

// ScriptPlugin allows running scripts as plugins
type ScriptPlugin struct {
	name   string
	script string
	shell  string
	env    map[string]string
}

// NewScriptPlugin creates a script-based plugin
func NewScriptPlugin(name, script, shell string, env map[string]string) *ScriptPlugin {
	if shell == "" {
		shell = "bash"
	}
	return &ScriptPlugin{
		name:   name,
		script: script,
		shell:  shell,
		env:    env,
	}
}

func (p *ScriptPlugin) Name() string    { return p.name }
func (p *ScriptPlugin) Version() string { return "1.0.0" }
func (p *ScriptPlugin) Type() string    { return "script" }

func (p *ScriptPlugin) Execute(ctx context.Context, event string, data map[string]interface{}) error {
	// Write script to temp file with appropriate extension
	ext := ".sh"
	if runtime.GOOS == "windows" {
		// Use .ps1 for PowerShell or .bat for cmd
		if strings.Contains(p.shell, "powershell") {
			ext = ".ps1"
		} else {
			ext = ".bat"
		}
	}

	tmpFile, err := os.CreateTemp("", "releaser-script-*"+ext)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(p.script); err != nil {
		return err
	}
	tmpFile.Close()

	cmd := exec.CommandContext(ctx, p.shell, tmpFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range p.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Pass data as JSON in RELEASER_DATA env var
	if data != nil {
		jsonData, _ := json.Marshal(data)
		cmd.Env = append(cmd.Env, "RELEASER_DATA="+string(jsonData))
	}
	cmd.Env = append(cmd.Env, "RELEASER_EVENT="+event)

	return cmd.Run()
}

// BuiltinPlugins provides built-in plugin implementations
type BuiltinPlugins struct{}

// CreateCustomBuilder creates a builder from configuration
func (bp *BuiltinPlugins) CreateCustomBuilder(cfg config.CustomBuilder) BuilderPlugin {
	return &customBuilder{cfg: cfg}
}

type customBuilder struct {
	cfg config.CustomBuilder
}

func (b *customBuilder) Name() string    { return b.cfg.ID }
func (b *customBuilder) Version() string { return "1.0.0" }
func (b *customBuilder) Type() string    { return "builder" }

func (b *customBuilder) Build(ctx context.Context, cfg map[string]interface{}) ([]artifact.Artifact, error) {
	// Execute command
	parts := strings.Fields(b.cfg.Command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("no command specified")
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = b.cfg.Dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range b.cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	// Collect output artifacts
	var artifacts []artifact.Artifact
	for _, out := range b.cfg.Outputs {
		matches, err := filepath.Glob(out)
		if err != nil {
			continue
		}
		for _, m := range matches {
			artifacts = append(artifacts, artifact.Artifact{
				Name: filepath.Base(m),
				Path: m,
				Type: artifact.TypeArchive,
			})
		}
	}

	return artifacts, nil
}
