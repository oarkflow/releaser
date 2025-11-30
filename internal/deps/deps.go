/*
Package deps provides dependency detection and installation functionality.
It automatically detects missing tools and offers to install them.
*/
package deps

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/log"
)

// Tool represents a required tool/dependency
type Tool struct {
	Name        string   // Display name
	Binary      string   // Binary name to check
	Description string   // What the tool is for
	InstallCmds []string // Installation commands per OS
	Optional    bool     // If true, skip if not installable
}

// CommonTools defines commonly needed tools for building
var CommonTools = map[string]Tool{
	"zig": {
		Name:        "Zig",
		Binary:      "zig",
		Description: "Universal C/C++ cross-compiler",
		InstallCmds: []string{
			"linux:snap:sudo snap install zig --classic --beta",
			"linux:apt:sudo apt-get update && sudo apt-get install -y zig",
			"linux:curl -sL https://ziglang.org/download/0.13.0/zig-linux-x86_64-0.13.0.tar.xz | sudo tar -xJ -C /usr/local && sudo ln -sf /usr/local/zig-linux-x86_64-0.13.0/zig /usr/local/bin/zig",
			"darwin:brew install zig",
			"windows:choco install zig",
		},
	},
	"nfpm": {
		Name:        "nFPM",
		Binary:      "nfpm",
		Description: "Package builder for deb, rpm, apk",
		InstallCmds: []string{
			"linux:go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest",
			"darwin:brew install nfpm",
			"darwin:go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest",
			"windows:go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest",
		},
	},
	"upx": {
		Name:        "UPX",
		Binary:      "upx",
		Description: "Binary compression tool",
		InstallCmds: []string{
			"linux:apt:sudo apt-get update && sudo apt-get install -y upx",
			"linux:yum:sudo yum install -y upx",
			"darwin:brew install upx",
			"windows:choco install upx",
		},
		Optional: true,
	},
	"cosign": {
		Name:        "Cosign",
		Binary:      "cosign",
		Description: "Container/artifact signing tool",
		InstallCmds: []string{
			"linux:go install github.com/sigstore/cosign/v2/cmd/cosign@latest",
			"darwin:brew install cosign",
			"darwin:go install github.com/sigstore/cosign/v2/cmd/cosign@latest",
			"windows:go install github.com/sigstore/cosign/v2/cmd/cosign@latest",
		},
		Optional: true,
	},
	"gpg": {
		Name:        "GPG",
		Binary:      "gpg",
		Description: "GNU Privacy Guard for signing",
		InstallCmds: []string{
			"linux:apt:sudo apt-get update && sudo apt-get install -y gnupg",
			"linux:yum:sudo yum install -y gnupg2",
			"darwin:brew install gnupg",
			"windows:choco install gpg4win",
		},
		Optional: true,
	},
	"docker": {
		Name:        "Docker",
		Binary:      "docker",
		Description: "Container runtime",
		InstallCmds: []string{
			"linux:curl -fsSL https://get.docker.com | sh",
			"darwin:brew install --cask docker",
			"windows:choco install docker-desktop",
		},
		Optional: true,
	},
	"mingw-w64": {
		Name:        "MinGW-w64",
		Binary:      "x86_64-w64-mingw32-gcc",
		Description: "Windows cross-compiler",
		InstallCmds: []string{
			"linux:apt:sudo apt-get update && sudo apt-get install -y mingw-w64",
			"linux:yum:sudo yum install -y mingw64-gcc mingw64-gcc-c++",
			"darwin:brew install mingw-w64",
		},
		Optional: true,
	},
	"gcc-aarch64": {
		Name:        "GCC ARM64 Cross-Compiler",
		Binary:      "aarch64-linux-gnu-gcc",
		Description: "ARM64 Linux cross-compiler",
		InstallCmds: []string{
			"linux:apt:sudo apt-get update && sudo apt-get install -y gcc-aarch64-linux-gnu g++-aarch64-linux-gnu",
			"linux:yum:sudo yum install -y gcc-aarch64-linux-gnu gcc-c++-aarch64-linux-gnu",
		},
		Optional: true,
	},
	"fyne-cross": {
		Name:        "Fyne Cross",
		Binary:      "fyne-cross",
		Description: "Cross-compilation tool for Fyne GUI apps",
		InstallCmds: []string{
			"linux:go install github.com/fyne-io/fyne-cross@latest",
			"darwin:go install github.com/fyne-io/fyne-cross@latest",
			"windows:go install github.com/fyne-io/fyne-cross@latest",
		},
		Optional: true,
	},
	"syft": {
		Name:        "Syft",
		Binary:      "syft",
		Description: "SBOM generator",
		InstallCmds: []string{
			"linux:curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin",
			"darwin:brew install syft",
			"windows:choco install syft",
		},
		Optional: true,
	},
}

// AutoInstall controls whether to auto-install missing dependencies
var AutoInstall = false

// PromptForInstall controls whether to prompt user for installation
var PromptForInstall = true

// CheckAndInstall checks if a tool is available and offers to install it if missing
func CheckAndInstall(toolName string) error {
	tool, ok := CommonTools[toolName]
	if !ok {
		return fmt.Errorf("unknown tool: %s", toolName)
	}

	return CheckAndInstallTool(tool)
}

// CheckAndInstallTool checks if a tool is available and offers to install it
func CheckAndInstallTool(tool Tool) error {
	if IsAvailable(tool.Binary) {
		return nil
	}

	log.Warn("Tool not found", "tool", tool.Name, "binary", tool.Binary)

	if tool.Optional && !AutoInstall && !PromptForInstall {
		log.Info("Skipping optional tool", "tool", tool.Name)
		return nil
	}

	// Find installation command for current OS
	installCmd := findInstallCommand(tool.InstallCmds)
	if installCmd == "" {
		if tool.Optional {
			log.Warn("No installation method available", "tool", tool.Name, "os", runtime.GOOS)
			return nil
		}
		return fmt.Errorf("no installation method available for %s on %s", tool.Name, runtime.GOOS)
	}

	// Prompt user if needed
	if !AutoInstall && PromptForInstall {
		fmt.Printf("\n🔧 %s (%s) is required but not installed.\n", tool.Name, tool.Description)
		fmt.Printf("   Install command: %s\n", installCmd)
		fmt.Print("   Install now? [Y/n]: ")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "" && response != "y" && response != "yes" {
			if tool.Optional {
				log.Info("Skipping installation", "tool", tool.Name)
				return nil
			}
			return fmt.Errorf("installation declined for required tool: %s", tool.Name)
		}
	}

	// Install the tool
	log.Info("Installing tool", "tool", tool.Name)
	if err := runInstallCommand(installCmd); err != nil {
		if tool.Optional {
			log.Warn("Installation failed", "tool", tool.Name, "error", err)
			return nil
		}
		return fmt.Errorf("failed to install %s: %w", tool.Name, err)
	}

	// Verify installation
	if !IsAvailable(tool.Binary) {
		// Try updating PATH
		updatePath()
		if !IsAvailable(tool.Binary) {
			if tool.Optional {
				log.Warn("Tool not available after installation", "tool", tool.Name)
				return nil
			}
			return fmt.Errorf("%s installed but not found in PATH", tool.Name)
		}
	}

	log.Info("Tool installed successfully", "tool", tool.Name)
	return nil
}

// IsAvailable checks if a binary is available in PATH
func IsAvailable(binary string) bool {
	_, err := exec.LookPath(binary)
	return err == nil
}

// findInstallCommand finds the best installation command for the current OS
func findInstallCommand(cmds []string) string {
	os := runtime.GOOS

	// Detect package manager on Linux
	var pkgManager string
	if os == "linux" {
		if IsAvailable("apt-get") || IsAvailable("apt") {
			pkgManager = "apt"
		} else if IsAvailable("yum") {
			pkgManager = "yum"
		} else if IsAvailable("dnf") {
			pkgManager = "dnf"
		} else if IsAvailable("pacman") {
			pkgManager = "pacman"
		} else if IsAvailable("snap") {
			pkgManager = "snap"
		}
	}

	// Find matching command
	var fallback string
	for _, cmd := range cmds {
		parts := strings.SplitN(cmd, ":", 2)
		if len(parts) < 2 {
			continue
		}

		cmdOS := parts[0]
		if cmdOS != os {
			continue
		}

		// Check for package manager specific command
		remaining := parts[1]
		subParts := strings.SplitN(remaining, ":", 2)
		if len(subParts) == 2 {
			// Has package manager specification
			if subParts[0] == pkgManager {
				return subParts[1]
			}
			continue
		}

		// Generic command for this OS
		if fallback == "" {
			fallback = remaining
		}
	}

	return fallback
}

// runInstallCommand runs an installation command
func runInstallCommand(cmdStr string) error {
	log.Debug("Running installation command", "cmd", cmdStr)

	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// updatePath updates the PATH to include common installation locations
func updatePath() {
	paths := []string{
		"/usr/local/bin",
		"/usr/local/go/bin",
		os.Getenv("HOME") + "/go/bin",
		os.Getenv("HOME") + "/.local/bin",
		"/snap/bin",
	}

	currentPath := os.Getenv("PATH")
	for _, p := range paths {
		if !strings.Contains(currentPath, p) {
			currentPath = p + ":" + currentPath
		}
	}
	os.Setenv("PATH", currentPath)
}

// EnsureCrossCompilers ensures cross-compilers are available for the given targets
func EnsureCrossCompilers(targets []string) error {
	neededTools := make(map[string]bool)

	for _, target := range targets {
		parts := strings.Split(target, "_")
		if len(parts) != 2 {
			continue
		}
		goos, goarch := parts[0], parts[1]

		// Skip native target
		if goos == runtime.GOOS && goarch == runtime.GOARCH {
			continue
		}

		// Determine needed cross-compiler
		switch goos {
		case "windows":
			neededTools["mingw-w64"] = true
		case "linux":
			if goarch == "arm64" {
				neededTools["gcc-aarch64"] = true
			}
		case "darwin":
			// macOS cross-compilation is complex, suggest zig
			neededTools["zig"] = true
		}
	}

	// Zig is the universal fallback
	if len(neededTools) > 0 {
		// Try zig first as it's universal
		if !IsAvailable("zig") {
			neededTools["zig"] = true
		}
	}

	// Install needed tools
	for toolName := range neededTools {
		if err := CheckAndInstall(toolName); err != nil {
			log.Warn("Could not install cross-compiler", "tool", toolName, "error", err)
		}
	}

	return nil
}

// EnsureBuildTools ensures required build tools are available
func EnsureBuildTools(needsPackaging, needsSigning, needsDocker, needsSBOM bool) error {
	if needsPackaging {
		if err := CheckAndInstall("nfpm"); err != nil {
			return err
		}
	}

	if needsSigning {
		// Try cosign first, fall back to gpg
		if !IsAvailable("cosign") && !IsAvailable("gpg") {
			if err := CheckAndInstall("cosign"); err != nil {
				if err := CheckAndInstall("gpg"); err != nil {
					log.Warn("No signing tool available")
				}
			}
		}
	}

	if needsDocker {
		if err := CheckAndInstall("docker"); err != nil {
			return err
		}
	}

	if needsSBOM {
		if err := CheckAndInstall("syft"); err != nil {
			log.Warn("SBOM generation will be skipped", "error", err)
		}
	}

	return nil
}

// DetectAndInstallForFyne ensures Fyne GUI dependencies are available
func DetectAndInstallForFyne() error {
	log.Info("Checking Fyne GUI build dependencies")

	// Fyne requires various system dependencies
	if runtime.GOOS == "linux" {
		// Check for required X11/GL libraries
		deps := []string{
			"libgl1-mesa-dev",
			"xorg-dev",
			"libxcursor-dev",
			"libxrandr-dev",
			"libxinerama-dev",
			"libxi-dev",
			"libxxf86vm-dev",
		}

		// Check if we need to install
		needsInstall := false
		for _, dep := range deps {
			cmd := exec.Command("dpkg", "-s", dep)
			if err := cmd.Run(); err != nil {
				needsInstall = true
				break
			}
		}

		if needsInstall {
			if PromptForInstall {
				fmt.Printf("\n🔧 Fyne GUI requires system dependencies.\n")
				fmt.Printf("   Packages: %s\n", strings.Join(deps, " "))
				fmt.Print("   Install now? [Y/n]: ")

				reader := bufio.NewReader(os.Stdin)
				response, _ := reader.ReadString('\n')
				response = strings.TrimSpace(strings.ToLower(response))

				if response == "" || response == "y" || response == "yes" {
					cmd := exec.Command("sudo", "apt-get", "update")
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					cmd.Run()

					args := append([]string{"apt-get", "install", "-y"}, deps...)
					cmd = exec.Command("sudo", args...)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					if err := cmd.Run(); err != nil {
						return fmt.Errorf("failed to install Fyne dependencies: %w", err)
					}
				}
			}
		}
	}

	return nil
}

// GetInstallInstructions returns installation instructions for a tool
func GetInstallInstructions(toolName string) string {
	tool, ok := CommonTools[toolName]
	if !ok {
		return fmt.Sprintf("Unknown tool: %s", toolName)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Installation instructions for %s:\n", tool.Name))
	sb.WriteString(fmt.Sprintf("  %s\n\n", tool.Description))

	for _, cmd := range tool.InstallCmds {
		parts := strings.SplitN(cmd, ":", 2)
		if len(parts) >= 2 {
			os := parts[0]
			remaining := parts[1]

			subParts := strings.SplitN(remaining, ":", 2)
			if len(subParts) == 2 {
				sb.WriteString(fmt.Sprintf("  %s (%s):\n    %s\n", os, subParts[0], subParts[1]))
			} else {
				sb.WriteString(fmt.Sprintf("  %s:\n    %s\n", os, remaining))
			}
		}
	}

	return sb.String()
}
