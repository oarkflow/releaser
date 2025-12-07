// Package hook provides lifecycle hook execution.
package hook

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// Runner executes lifecycle hooks.
type Runner struct {
	tmplCtx *tmpl.Context
	workDir string
}

// NewRunner creates a new hook runner.
func NewRunner(tmplCtx *tmpl.Context, workDir string) *Runner {
	return &Runner{
		tmplCtx: tmplCtx,
		workDir: workDir,
	}
}

// Run executes a hook.
func (r *Runner) Run(ctx context.Context, hook config.Hook) error {
	// Check condition
	if hook.If != "" {
		condition, err := r.tmplCtx.Apply(hook.If)
		if err != nil {
			return fmt.Errorf("failed to evaluate condition: %w", err)
		}
		if condition != "true" && condition != "1" {
			log.Debug("Skipping hook due to condition", "condition", hook.If)
			return nil
		}
	}

	// Determine the command to run
	cmd := hook.Cmd
	if cmd == "" {
		return nil
	}

	// Apply template
	cmd, err := r.tmplCtx.Apply(cmd)
	if err != nil {
		return fmt.Errorf("failed to apply template to command: %w", err)
	}

	log.Info("Running hook", "cmd", cmd)

	// Create command
	var c *exec.Cmd
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		if runtime.GOOS == "windows" {
			shellPath = "powershell.exe"
		} else {
			shellPath = "/bin/sh"
		}
	}

	if hook.Shell {
		if runtime.GOOS == "windows" {
			c = exec.CommandContext(ctx, shellPath, "-Command", cmd)
		} else {
			c = exec.CommandContext(ctx, shellPath, "-c", cmd)
		}
	} else {
		// Parse command into args
		parts := strings.Fields(cmd)
		if len(parts) == 0 {
			return nil
		}
		c = exec.CommandContext(ctx, parts[0], parts[1:]...)
	}
	c.Dir = r.workDir

	// Set environment
	c.Env = os.Environ()
	for key, value := range hook.Env {
		expandedValue, _ := r.tmplCtx.Apply(value)
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", key, expandedValue))
	}

	// Handle output
	if hook.Output == "true" || hook.Output == "1" {
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
	}

	// Run command
	if err := c.Run(); err != nil {
		if hook.FailFast {
			return fmt.Errorf("hook failed: %w", err)
		}
		log.Warn("Hook failed but continuing", "cmd", cmd, "error", err)
	}

	return nil
}

// RunCommand executes a simple command string.
func (r *Runner) RunCommand(ctx context.Context, cmd string) error {
	if cmd == "" {
		return nil
	}

	// Apply template
	cmd, err := r.tmplCtx.Apply(cmd)
	if err != nil {
		return fmt.Errorf("failed to apply template to command: %w", err)
	}

	log.Info("Running command", "cmd", cmd)

	shell := os.Getenv("SHELL")
	if shell == "" {
		if runtime.GOOS == "windows" {
			shell = "powershell.exe"
		} else {
			shell = "/bin/sh"
		}
	}

	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.CommandContext(ctx, shell, "-Command", cmd)
	} else {
		c = exec.CommandContext(ctx, shell, "-c", cmd)
	}
	c.Dir = r.workDir
	c.Env = os.Environ()
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	if err := c.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// RunHooks executes multiple hooks.
func (r *Runner) RunHooks(ctx context.Context, hooks []config.Hook) error {
	for _, hook := range hooks {
		if err := r.Run(ctx, hook); err != nil {
			return err
		}
	}
	return nil
}

// ParseCommand parses a command string into a hook.
func ParseCommand(cmd string) config.Hook {
	return config.Hook{
		Cmd:      cmd,
		FailFast: true,
		Output:   "true",
	}
}

// Executor is a simple hook executor for use in pipelines.
type Executor struct {
	hooks  []config.Hook
	runner *Runner
}

// NewExecutor creates a new hook executor.
func NewExecutor(hooks []config.Hook, tmplCtx *tmpl.Context, workDir string) *Executor {
	return &Executor{
		hooks:  hooks,
		runner: NewRunner(tmplCtx, workDir),
	}
}

// Execute runs all hooks.
func (e *Executor) Execute(ctx context.Context) error {
	return e.runner.RunHooks(ctx, e.hooks)
}

// GlobalHooks manages global lifecycle hooks.
type GlobalHooks struct {
	Before []config.Hook
	After  []config.Hook
	runner *Runner
}

// NewGlobalHooks creates global hook manager.
func NewGlobalHooks(before, after []config.Hook, tmplCtx *tmpl.Context, workDir string) *GlobalHooks {
	return &GlobalHooks{
		Before: before,
		After:  after,
		runner: NewRunner(tmplCtx, workDir),
	}
}

// RunBefore executes before hooks.
func (g *GlobalHooks) RunBefore(ctx context.Context) error {
	log.Debug("Running global before hooks", "count", len(g.Before))
	return g.runner.RunHooks(ctx, g.Before)
}

// RunAfter executes after hooks.
func (g *GlobalHooks) RunAfter(ctx context.Context) error {
	log.Debug("Running global after hooks", "count", len(g.After))
	return g.runner.RunHooks(ctx, g.After)
}

// FromStrings converts a slice of command strings to hooks.
func FromStrings(commands []string) []config.Hook {
	hooks := make([]config.Hook, 0, len(commands))
	for _, cmd := range commands {
		hooks = append(hooks, config.Hook{
			Cmd:      cmd,
			FailFast: true,
			Output:   "true",
		})
	}
	return hooks
}

// ToEnvironment converts a map to environment variable format.
func ToEnvironment(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

// MergeEnvironment merges multiple environment maps.
func MergeEnvironment(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// ExpandVariables expands environment variables in a string.
func ExpandVariables(s string, env map[string]string) string {
	for k, v := range env {
		s = strings.ReplaceAll(s, "${"+k+"}", v)
		s = strings.ReplaceAll(s, "$"+k, v)
	}
	return os.ExpandEnv(s)
}
