/*
Package cmd provides shell completion commands for Releaser.
*/
package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// completionCmd generates shell completions
var completionCmd = &cobra.Command{
	Use:   "completion [shell]",
	Short: "Generate shell completions",
	Long: `Generate shell completion scripts for various shells.

The completion script must be sourced in your shell's configuration file.

Bash:
  Add the following to ~/.bashrc:
    source <(releaser completion bash)

  Or save to a file:
    releaser completion bash > /etc/bash_completion.d/releaser

Zsh:
  Add the following to ~/.zshrc:
    source <(releaser completion zsh)

  Or save to completion directory:
    releaser completion zsh > "${fpath[1]}/_releaser"

Fish:
  Add the following to ~/.config/fish/config.fish:
    releaser completion fish | source

  Or save to completion directory:
    releaser completion fish > ~/.config/fish/completions/releaser.fish

PowerShell:
  Add the following to your PowerShell profile:
    releaser completion powershell | Out-String | Invoke-Expression
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

// completionInstallCmd installs shell completions
var completionInstallCmd = &cobra.Command{
	Use:   "install [shell]",
	Short: "Install shell completions",
	Long: `Install shell completion scripts to the appropriate location.

Automatically detects and installs completion scripts for the specified shell.

Examples:
  releaser completion install bash
  releaser completion install zsh
  releaser completion install fish
  releaser completion install powershell
`,
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		return installCompletion(cmd.Root(), args[0])
	},
}

func init() {
	completionCmd.AddCommand(completionInstallCmd)
	rootCmd.AddCommand(completionCmd)
}

// installCompletion installs shell completion to the appropriate location
func installCompletion(rootCmd *cobra.Command, shell string) error {
	var (
		content bytes.Buffer
		path    string
	)

	switch shell {
	case "bash":
		rootCmd.GenBashCompletion(&content)
		// Try multiple locations
		locations := []string{
			"/etc/bash_completion.d/releaser",
			filepath.Join(os.Getenv("HOME"), ".local/share/bash-completion/completions/releaser"),
			filepath.Join(os.Getenv("HOME"), ".bash_completion.d/releaser"),
		}
		for _, loc := range locations {
			dir := filepath.Dir(loc)
			if _, err := os.Stat(dir); err == nil {
				path = loc
				break
			}
			// Try to create directory
			if err := os.MkdirAll(dir, 0755); err == nil {
				path = loc
				break
			}
		}
		if path == "" {
			path = filepath.Join(os.Getenv("HOME"), ".bash_completion.d/releaser")
			os.MkdirAll(filepath.Dir(path), 0755)
		}

	case "zsh":
		rootCmd.GenZshCompletion(&content)
		// Try to find fpath directory
		locations := []string{
			"/usr/local/share/zsh/site-functions/_releaser",
			filepath.Join(os.Getenv("HOME"), ".zsh/completions/_releaser"),
			filepath.Join(os.Getenv("HOME"), ".local/share/zsh/site-functions/_releaser"),
		}
		for _, loc := range locations {
			dir := filepath.Dir(loc)
			if _, err := os.Stat(dir); err == nil {
				path = loc
				break
			}
		}
		if path == "" {
			path = filepath.Join(os.Getenv("HOME"), ".zsh/completions/_releaser")
			os.MkdirAll(filepath.Dir(path), 0755)
		}

	case "fish":
		rootCmd.GenFishCompletion(&content, true)
		// Fish completion directory
		locations := []string{
			filepath.Join(os.Getenv("HOME"), ".config/fish/completions/releaser.fish"),
			"/usr/share/fish/completions/releaser.fish",
		}
		for _, loc := range locations {
			dir := filepath.Dir(loc)
			if _, err := os.Stat(dir); err == nil {
				path = loc
				break
			}
		}
		if path == "" {
			path = filepath.Join(os.Getenv("HOME"), ".config/fish/completions/releaser.fish")
			os.MkdirAll(filepath.Dir(path), 0755)
		}

	case "powershell":
		rootCmd.GenPowerShellCompletionWithDesc(&content)
		// PowerShell profile location
		psDir := filepath.Join(os.Getenv("HOME"), ".config/powershell")
		if _, err := os.Stat(psDir); os.IsNotExist(err) {
			// Try Windows-style path
			psDir = filepath.Join(os.Getenv("USERPROFILE"), "Documents", "PowerShell")
		}
		os.MkdirAll(psDir, 0755)
		path = filepath.Join(psDir, "releaser.ps1")

	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}

	if err := os.WriteFile(path, content.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write completion file: %w", err)
	}

	fmt.Printf("Completion script installed to: %s\n", path)

	// Print additional instructions
	switch shell {
	case "bash":
		fmt.Println("\nTo enable completions, run:")
		fmt.Printf("  source %s\n", path)
		fmt.Println("\nOr add to ~/.bashrc:")
		fmt.Printf("  source %s\n", path)
	case "zsh":
		fmt.Println("\nTo enable completions, add to ~/.zshrc:")
		fmt.Printf("  fpath=(%s $fpath)\n", filepath.Dir(path))
		fmt.Println("  autoload -Uz compinit && compinit")
	case "fish":
		fmt.Println("\nFish completions are automatically loaded.")
	case "powershell":
		fmt.Println("\nTo enable completions, add to your PowerShell profile:")
		fmt.Printf("  . %s\n", path)
	}

	return nil
}
