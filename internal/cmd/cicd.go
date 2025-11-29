/*
Package cmd provides CI/CD generation commands for Releaser.
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/oarkflow/releaser/internal/cicd"
)

var cicdCmd = &cobra.Command{
	Use:     "cicd",
	Aliases: []string{"ci"},
	Short:   "Generate CI/CD pipeline configurations",
	Long: `Generate CI/CD pipeline configurations for various platforms.

Supported platforms:
  - github    GitHub Actions
  - gitlab    GitLab CI
  - circleci  CircleCI
  - travis    Travis CI
  - jenkins   Jenkinsfile
  - azure     Azure Pipelines
  - bitbucket Bitbucket Pipelines
  - drone     Drone CI

Examples:
  releaser cicd github
  releaser cicd gitlab --docker
  releaser cicd github --go-version 1.22
`,
}

var cicdGenerateCmd = &cobra.Command{
	Use:   "generate [platform]",
	Short: "Generate CI/CD configuration",
	Long: `Generate a CI/CD configuration file for the specified platform.

Examples:
  releaser cicd generate github
  releaser cicd generate gitlab
  releaser cicd generate circleci --docker
`,
	ValidArgs: []string{"github", "gitlab", "circleci", "travis", "jenkins", "azure", "bitbucket", "drone"},
	Args:      cobra.ExactArgs(1),
	RunE:      runCICDGenerate,
}

var (
	cicdGoVersion     string
	cicdNodeVersion   string
	cicdPythonVersion string
	cicdDockerEnabled bool
	cicdDockerImage   string
	cicdOutputDir     string
	cicdTestCommand   string
	cicdBuildCommand  string
	cicdBranch        string
)

func init() {
	cicdGenerateCmd.Flags().StringVar(&cicdGoVersion, "go-version", "1.22", "Go version")
	cicdGenerateCmd.Flags().StringVar(&cicdNodeVersion, "node-version", "20", "Node.js version")
	cicdGenerateCmd.Flags().StringVar(&cicdPythonVersion, "python-version", "3.11", "Python version")
	cicdGenerateCmd.Flags().BoolVar(&cicdDockerEnabled, "docker", false, "Enable Docker build and push")
	cicdGenerateCmd.Flags().StringVar(&cicdDockerImage, "docker-image", "", "Docker image name")
	cicdGenerateCmd.Flags().StringVarP(&cicdOutputDir, "output", "o", ".", "Output directory")
	cicdGenerateCmd.Flags().StringVar(&cicdTestCommand, "test-command", "go test ./...", "Test command")
	cicdGenerateCmd.Flags().StringVar(&cicdBuildCommand, "build-command", "releaser build --snapshot", "Build command")
	cicdGenerateCmd.Flags().StringVar(&cicdBranch, "branch", "main", "Main branch name")

	cicdCmd.AddCommand(cicdGenerateCmd)

	// Add shortcuts for common platforms
	for _, platform := range []string{"github", "gitlab", "circleci", "travis", "jenkins", "azure", "bitbucket", "drone"} {
		p := platform // capture
		cicdCmd.AddCommand(&cobra.Command{
			Use:   platform,
			Short: fmt.Sprintf("Generate %s configuration", platformName(platform)),
			RunE: func(cmd *cobra.Command, args []string) error {
				return generateCICD(cicd.Platform(p))
			},
		})
	}

	rootCmd.AddCommand(cicdCmd)
}

func runCICDGenerate(cmd *cobra.Command, args []string) error {
	platform := cicd.Platform(strings.ToLower(args[0]))
	return generateCICD(platform)
}

func generateCICD(platform cicd.Platform) error {
	opts := cicd.Options{
		Platform:      platform,
		GoVersion:     cicdGoVersion,
		NodeVersion:   cicdNodeVersion,
		PythonVersion: cicdPythonVersion,
		DockerEnabled: cicdDockerEnabled,
		DockerImage:   cicdDockerImage,
		TestCommand:   cicdTestCommand,
		BuildCommand:  cicdBuildCommand,
		PublishBranch: cicdBranch,
	}

	// Try to get project name from go.mod or package.json
	if opts.ProjectName == "" {
		opts.ProjectName = detectProjectName()
	}

	gen := cicd.NewGenerator(opts)

	outputDir := cicdOutputDir
	if outputDir == "" {
		outputDir = "."
	}

	if err := gen.Generate(outputDir); err != nil {
		return fmt.Errorf("failed to generate CI/CD config: %w", err)
	}

	log.Info("CI/CD configuration generated", "platform", platform, "dir", outputDir)
	fmt.Printf("\nGenerated %s configuration in %s\n", platformName(string(platform)), outputDir)
	printPlatformInstructions(platform)

	return nil
}

func platformName(p string) string {
	names := map[string]string{
		"github":    "GitHub Actions",
		"gitlab":    "GitLab CI",
		"circleci":  "CircleCI",
		"travis":    "Travis CI",
		"jenkins":   "Jenkins",
		"azure":     "Azure Pipelines",
		"bitbucket": "Bitbucket Pipelines",
		"drone":     "Drone CI",
	}
	if name, ok := names[p]; ok {
		return name
	}
	return p
}

func detectProjectName() string {
	// Try go.mod
	if data, err := os.ReadFile("go.mod"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "module ") {
				parts := strings.Split(strings.TrimPrefix(line, "module "), "/")
				return parts[len(parts)-1]
			}
		}
	}

	// Try package.json
	if data, err := os.ReadFile("package.json"); err == nil {
		if strings.Contains(string(data), `"name"`) {
			// Simple extraction
			for _, line := range strings.Split(string(data), "\n") {
				if strings.Contains(line, `"name"`) {
					parts := strings.Split(line, ":")
					if len(parts) >= 2 {
						name := strings.Trim(parts[1], ` ",`)
						return name
					}
				}
			}
		}
	}

	// Fallback to directory name
	if wd, err := os.Getwd(); err == nil {
		return strings.ToLower(strings.ReplaceAll(wd[strings.LastIndex(wd, "/")+1:], " ", "-"))
	}

	return "project"
}

func printPlatformInstructions(platform cicd.Platform) {
	fmt.Println("\nNext steps:")
	switch platform {
	case cicd.PlatformGitHubActions:
		fmt.Println("  1. Commit .github/workflows/release.yml")
		fmt.Println("  2. Add GITHUB_TOKEN to repository secrets (auto-provided)")
		fmt.Println("  3. Push a tag (git tag v1.0.0 && git push --tags)")
	case cicd.PlatformGitLabCI:
		fmt.Println("  1. Commit .gitlab-ci.yml")
		fmt.Println("  2. Configure CI/CD variables in GitLab settings")
		fmt.Println("  3. Push a tag to trigger release")
	case cicd.PlatformCircleCI:
		fmt.Println("  1. Commit .circleci/config.yml")
		fmt.Println("  2. Connect repository to CircleCI")
		fmt.Println("  3. Add environment variables in CircleCI settings")
	case cicd.PlatformJenkinsfile:
		fmt.Println("  1. Commit Jenkinsfile")
		fmt.Println("  2. Create pipeline job in Jenkins")
		fmt.Println("  3. Add github-token credential")
	case cicd.PlatformAzurePipelines:
		fmt.Println("  1. Commit azure-pipelines.yml")
		fmt.Println("  2. Create pipeline in Azure DevOps")
		fmt.Println("  3. Add GITHUB_TOKEN variable")
	default:
		fmt.Println("  1. Commit the configuration file")
		fmt.Println("  2. Configure secrets/variables as needed")
		fmt.Println("  3. Push a tag to trigger release")
	}
}
