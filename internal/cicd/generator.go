// Package cicd provides CI/CD pipeline template generation.
package cicd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// Platform represents a CI/CD platform
type Platform string

const (
	PlatformGitHubActions  Platform = "github"
	PlatformGitLabCI       Platform = "gitlab"
	PlatformCircleCI       Platform = "circleci"
	PlatformTravisCI       Platform = "travis"
	PlatformJenkinsfile    Platform = "jenkins"
	PlatformAzurePipelines Platform = "azure"
	PlatformBitbucket      Platform = "bitbucket"
	PlatformDrone          Platform = "drone"
)

// Options for CI/CD template generation
type Options struct {
	Platform      Platform
	ProjectName   string
	GoVersion     string
	NodeVersion   string
	PythonVersion string
	RustVersion   string
	DockerEnabled bool
	DockerImage   string
	Artifacts     []string
	TestCommand   string
	BuildCommand  string
	PublishBranch string
}

// DefaultOptions returns default options
func DefaultOptions() Options {
	return Options{
		Platform:      PlatformGitHubActions,
		GoVersion:     "1.22",
		NodeVersion:   "20",
		PythonVersion: "3.11",
		RustVersion:   "stable",
		PublishBranch: "main",
		TestCommand:   "go test ./...",
		BuildCommand:  "releaser build --snapshot",
	}
}

// Generator generates CI/CD pipeline configurations
type Generator struct {
	opts Options
}

// NewGenerator creates a new CI/CD generator
func NewGenerator(opts Options) *Generator {
	return &Generator{opts: opts}
}

// Generate creates the CI/CD configuration file
func (g *Generator) Generate(outputDir string) error {
	switch g.opts.Platform {
	case PlatformGitHubActions:
		return g.generateGitHubActions(outputDir)
	case PlatformGitLabCI:
		return g.generateGitLabCI(outputDir)
	case PlatformCircleCI:
		return g.generateCircleCI(outputDir)
	case PlatformTravisCI:
		return g.generateTravisCI(outputDir)
	case PlatformJenkinsfile:
		return g.generateJenkinsfile(outputDir)
	case PlatformAzurePipelines:
		return g.generateAzurePipelines(outputDir)
	case PlatformBitbucket:
		return g.generateBitbucket(outputDir)
	case PlatformDrone:
		return g.generateDrone(outputDir)
	default:
		return fmt.Errorf("unsupported platform: %s", g.opts.Platform)
	}
}

// generateGitHubActions creates GitHub Actions workflow
func (g *Generator) generateGitHubActions(outputDir string) error {
	workflowDir := filepath.Join(outputDir, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		return err
	}

	tmpl := `name: Release

on:
  push:
    tags:
      - 'v*'
  pull_request:
    branches: [{{ .PublishBranch }}]

permissions:
  contents: write
  packages: write

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '{{ .GoVersion }}'
          cache: true

      - name: Run tests
        run: {{ .TestCommand }}

      - name: Build snapshot
        if: "!startsWith(github.ref, 'refs/tags/')"
        run: {{ .BuildCommand }}

  release:
    needs: build
    runs-on: ubuntu-latest
    if: startsWith(github.ref, 'refs/tags/')
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '{{ .GoVersion }}'
          cache: true
{{ if .DockerEnabled }}
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{"{{"}} secrets.DOCKERHUB_USERNAME {{"}}"}}
          password: ${{"{{"}} secrets.DOCKERHUB_TOKEN {{"}}"}}

      - name: Login to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{"{{"}} github.actor {{"}}"}}
          password: ${{"{{"}} secrets.GITHUB_TOKEN {{"}}"}}
{{ end }}
      - name: Run Releaser
        run: releaser release
        env:
          GITHUB_TOKEN: ${{"{{"}} secrets.GITHUB_TOKEN {{"}}"}}
{{ if .DockerEnabled }}
          DOCKER_USERNAME: ${{"{{"}} secrets.DOCKERHUB_USERNAME {{"}}"}}
          DOCKER_PASSWORD: ${{"{{"}} secrets.DOCKERHUB_TOKEN {{"}}"}}
{{ end }}
      - name: Upload artifacts
        uses: actions/upload-artifact@v4
        with:
          name: dist
          path: dist/
          retention-days: 5
`

	return g.writeTemplate(filepath.Join(workflowDir, "release.yml"), tmpl)
}

// generateGitLabCI creates GitLab CI configuration
func (g *Generator) generateGitLabCI(outputDir string) error {
	tmpl := `stages:
  - test
  - build
  - release

variables:
  GO_VERSION: "{{ .GoVersion }}"

.go-cache:
  variables:
    GOPATH: $CI_PROJECT_DIR/.go
  cache:
    paths:
      - .go/pkg/mod/

test:
  stage: test
  image: golang:${GO_VERSION}
  extends: .go-cache
  script:
    - {{ .TestCommand }}
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
    - if: $CI_COMMIT_TAG

build:snapshot:
  stage: build
  image: golang:${GO_VERSION}
  extends: .go-cache
  script:
    - {{ .BuildCommand }}
  artifacts:
    paths:
      - dist/
    expire_in: 1 day
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"

release:
  stage: release
  image: golang:${GO_VERSION}
  extends: .go-cache
  script:
    - releaser release
  artifacts:
    paths:
      - dist/
    expire_in: 30 days
  rules:
    - if: $CI_COMMIT_TAG
{{ if .DockerEnabled }}
release:docker:
  stage: release
  image: docker:latest
  services:
    - docker:dind
  script:
    - docker login -u $CI_REGISTRY_USER -p $CI_REGISTRY_PASSWORD $CI_REGISTRY
    - docker build -t $CI_REGISTRY_IMAGE:$CI_COMMIT_TAG .
    - docker push $CI_REGISTRY_IMAGE:$CI_COMMIT_TAG
  rules:
    - if: $CI_COMMIT_TAG
{{ end }}
`

	return g.writeTemplate(filepath.Join(outputDir, ".gitlab-ci.yml"), tmpl)
}

// generateCircleCI creates CircleCI configuration
func (g *Generator) generateCircleCI(outputDir string) error {
	circleDir := filepath.Join(outputDir, ".circleci")
	if err := os.MkdirAll(circleDir, 0755); err != nil {
		return err
	}

	tmpl := `version: 2.1

orbs:
  go: circleci/go@1.10

executors:
  default:
    docker:
      - image: cimg/go:{{ .GoVersion }}

jobs:
  test:
    executor: default
    steps:
      - checkout
      - go/load-cache
      - run:
          name: Run tests
          command: {{ .TestCommand }}
      - go/save-cache

  build:
    executor: default
    steps:
      - checkout
      - go/load-cache
      - run:
          name: Build snapshot
          command: {{ .BuildCommand }}
      - persist_to_workspace:
          root: .
          paths:
            - dist

  release:
    executor: default
    steps:
      - checkout
      - go/load-cache
      - run:
          name: Release
          command: releaser release
      - store_artifacts:
          path: dist

workflows:
  version: 2
  build-and-release:
    jobs:
      - test:
          filters:
            tags:
              only: /^v.*/
      - build:
          requires:
            - test
          filters:
            branches:
              only: {{ .PublishBranch }}
      - release:
          requires:
            - test
          filters:
            tags:
              only: /^v.*/
            branches:
              ignore: /.*/
`

	return g.writeTemplate(filepath.Join(circleDir, "config.yml"), tmpl)
}

// generateTravisCI creates Travis CI configuration
func (g *Generator) generateTravisCI(outputDir string) error {
	tmpl := `language: go

go:
  - {{ .GoVersion }}

cache:
  directories:
    - $GOPATH/pkg/mod

stages:
  - test
  - name: release
    if: tag IS present

jobs:
  include:
    - stage: test
      script: {{ .TestCommand }}

    - stage: release
      script:
        - releaser release
      deploy:
        provider: releases
        api_key: $GITHUB_TOKEN
        file_glob: true
        file: dist/*
        skip_cleanup: true
        on:
          tags: true
`

	return g.writeTemplate(filepath.Join(outputDir, ".travis.yml"), tmpl)
}

// generateJenkinsfile creates Jenkinsfile
func (g *Generator) generateJenkinsfile(outputDir string) error {
	tmpl := `pipeline {
    agent any

    tools {
        go '{{ .GoVersion }}'
    }

    environment {
        GOPATH = "${WORKSPACE}/go"
        PATH = "${GOPATH}/bin:${PATH}"
    }

    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        stage('Test') {
            steps {
                sh '{{ .TestCommand }}'
            }
        }

        stage('Build') {
            when {
                not { buildingTag() }
            }
            steps {
                sh '{{ .BuildCommand }}'
            }
        }

        stage('Release') {
            when {
                buildingTag()
            }
            environment {
                GITHUB_TOKEN = credentials('github-token')
            }
            steps {
                sh 'releaser release'
            }
        }
    }

    post {
        always {
            archiveArtifacts artifacts: 'dist/**/*', fingerprint: true, allowEmptyArchive: true
        }
    }
}
`

	return g.writeTemplate(filepath.Join(outputDir, "Jenkinsfile"), tmpl)
}

// generateAzurePipelines creates Azure Pipelines configuration
func (g *Generator) generateAzurePipelines(outputDir string) error {
	tmpl := `trigger:
  branches:
    include:
      - {{ .PublishBranch }}
  tags:
    include:
      - v*

pool:
  vmImage: 'ubuntu-latest'

variables:
  GOVERSION: '{{ .GoVersion }}'

stages:
  - stage: Test
    jobs:
      - job: Test
        steps:
          - task: GoTool@0
            inputs:
              version: '$(GOVERSION)'
          - script: {{ .TestCommand }}
            displayName: 'Run tests'

  - stage: Build
    condition: and(succeeded(), not(startsWith(variables['Build.SourceBranch'], 'refs/tags/')))
    jobs:
      - job: Build
        steps:
          - task: GoTool@0
            inputs:
              version: '$(GOVERSION)'
          - script: {{ .BuildCommand }}
            displayName: 'Build snapshot'
          - publish: dist
            artifact: dist

  - stage: Release
    condition: and(succeeded(), startsWith(variables['Build.SourceBranch'], 'refs/tags/'))
    jobs:
      - job: Release
        steps:
          - task: GoTool@0
            inputs:
              version: '$(GOVERSION)'
          - script: releaser release
            displayName: 'Run releaser'
            env:
              GITHUB_TOKEN: $(GITHUB_TOKEN)
          - publish: dist
            artifact: dist
`

	return g.writeTemplate(filepath.Join(outputDir, "azure-pipelines.yml"), tmpl)
}

// generateBitbucket creates Bitbucket Pipelines configuration
func (g *Generator) generateBitbucket(outputDir string) error {
	tmpl := `image: golang:{{ .GoVersion }}

pipelines:
  default:
    - step:
        name: Test
        caches:
          - go
        script:
          - {{ .TestCommand }}

  branches:
    {{ .PublishBranch }}:
      - step:
          name: Build
          caches:
            - go
          script:
            - {{ .BuildCommand }}
          artifacts:
            - dist/**

  tags:
    'v*':
      - step:
          name: Release
          caches:
            - go
          script:
            - releaser release
          artifacts:
            - dist/**

definitions:
  caches:
    go: /go/pkg/mod
`

	return g.writeTemplate(filepath.Join(outputDir, "bitbucket-pipelines.yml"), tmpl)
}

// generateDrone creates Drone CI configuration
func (g *Generator) generateDrone(outputDir string) error {
	tmpl := `kind: pipeline
type: docker
name: default

steps:
  - name: test
    image: golang:{{ .GoVersion }}
    commands:
      - {{ .TestCommand }}

  - name: build
    image: golang:{{ .GoVersion }}
    commands:
      - {{ .BuildCommand }}
    when:
      event:
        - push
        - pull_request

  - name: release
    image: golang:{{ .GoVersion }}
    environment:
      GITHUB_TOKEN:
        from_secret: github_token
    commands:
      - releaser release
    when:
      event:
        - tag
`

	return g.writeTemplate(filepath.Join(outputDir, ".drone.yml"), tmpl)
}

// writeTemplate renders and writes a template
func (g *Generator) writeTemplate(path string, tmplStr string) error {
	tmpl, err := template.New("cicd").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, g.opts); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// GetSupportedPlatforms returns all supported CI/CD platforms
func GetSupportedPlatforms() []Platform {
	return []Platform{
		PlatformGitHubActions,
		PlatformGitLabCI,
		PlatformCircleCI,
		PlatformTravisCI,
		PlatformJenkinsfile,
		PlatformAzurePipelines,
		PlatformBitbucket,
		PlatformDrone,
	}
}
