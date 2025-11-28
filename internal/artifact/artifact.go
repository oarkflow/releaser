/*
Package artifact provides artifact management for Releaser.
*/
package artifact

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Type represents the type of artifact
type Type string

const (
	TypeBinary          Type = "Binary"
	TypeArchive         Type = "Archive"
	TypePackage         Type = "Package"
	TypeChecksum        Type = "Checksum"
	TypeSignature       Type = "Signature"
	TypeLinuxPackage    Type = "Linux Package"
	TypeDockerImage     Type = "Docker Image"
	TypeDockerManifest  Type = "Docker Manifest"
	TypeSourceArchive   Type = "Source Archive"
	TypeSBOM            Type = "SBOM"
	TypeUploadable      Type = "Uploadable"
	TypePublishable     Type = "Publishable"
	TypeAnnounce        Type = "Announce"
	TypeMetadata        Type = "Metadata"
	TypeHeader          Type = "Header"
	TypeBrewTap         Type = "Homebrew Tap"
	TypeScoopManifest   Type = "Scoop Manifest"
	TypeNPMPackage      Type = "NPM Package"
	TypeDMG             Type = "DMG"
	TypeMSI             Type = "MSI"
	TypeNSIS            Type = "NSIS"
	TypeAppBundle       Type = "App Bundle"
	TypeUniversalBinary Type = "Universal Binary"
)

// Artifact represents a build artifact
type Artifact struct {
	// Name of the artifact
	Name string `json:"name"`

	// Path to the artifact file
	Path string `json:"path"`

	// Type of artifact
	Type Type `json:"type"`

	// Goos is the target OS
	Goos string `json:"goos,omitempty"`

	// Goarch is the target architecture
	Goarch string `json:"goarch,omitempty"`

	// Goarm is the ARM version
	Goarm string `json:"goarm,omitempty"`

	// Goamd64 is the AMD64 version
	Goamd64 string `json:"goamd64,omitempty"`

	// BuildID is the ID of the build that created this artifact
	BuildID string `json:"build_id,omitempty"`

	// Extra holds additional metadata
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// Manager manages artifacts
type Manager struct {
	artifacts []Artifact
	mu        sync.RWMutex
}

// NewManager creates a new artifact manager
func NewManager() *Manager {
	return &Manager{
		artifacts: make([]Artifact, 0),
	}
}

// Add adds an artifact
func (m *Manager) Add(a Artifact) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.artifacts = append(m.artifacts, a)
}

// All returns all artifacts
func (m *Manager) All() []Artifact {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Artifact, len(m.artifacts))
	copy(result, m.artifacts)
	return result
}

// List is an alias for All
func (m *Manager) List() []Artifact {
	return m.All()
}

// Count returns the number of artifacts
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.artifacts)
}

// Filter returns artifacts matching the given filters
func (m *Manager) Filter(filters ...FilterFunc) []Artifact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Artifact, 0)
	for _, a := range m.artifacts {
		match := true
		for _, f := range filters {
			if !f(a) {
				match = false
				break
			}
		}
		if match {
			result = append(result, a)
		}
	}
	return result
}

// FilterFunc is a function that filters artifacts
type FilterFunc func(Artifact) bool

// Filter is a struct-based filter specification
type Filter struct {
	Types  []Type
	Goos   string
	Goarch string
}

// ByType returns a filter for artifact type
func ByType(t Type) FilterFunc {
	return func(a Artifact) bool {
		return a.Type == t
	}
}

// ByGoos returns a filter for target OS
func ByGoos(goos string) FilterFunc {
	return func(a Artifact) bool {
		return a.Goos == goos
	}
}

// ByGoarch returns a filter for target architecture
func ByGoarch(goarch string) FilterFunc {
	return func(a Artifact) bool {
		return a.Goarch == goarch
	}
}

// ByBuildID returns a filter for build ID
func ByBuildID(id string) FilterFunc {
	return func(a Artifact) bool {
		return a.BuildID == id
	}
}

// ByIf evaluates an if statement for artifact filtering
func ByIf(expr string, ctx map[string]interface{}) FilterFunc {
	return func(a Artifact) bool {
		// TODO: Implement expression evaluation
		return true
	}
}

// Save saves artifacts to a JSON file
func (m *Manager) Save(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.artifacts, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Load loads artifacts from a JSON file
func (m *Manager) Load(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &m.artifacts)
}

// Clear removes all artifacts
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.artifacts = make([]Artifact, 0)
}

// GroupByPlatform groups artifacts by platform (goos/goarch)
func (m *Manager) GroupByPlatform() map[string][]Artifact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]Artifact)
	for _, a := range m.artifacts {
		key := a.Goos + "_" + a.Goarch
		if a.Goarm != "" {
			key += "_" + a.Goarm
		}
		result[key] = append(result[key], a)
	}
	return result
}

// GroupByType groups artifacts by type
func (m *Manager) GroupByType() map[Type][]Artifact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[Type][]Artifact)
	for _, a := range m.artifacts {
		result[a.Type] = append(result[a.Type], a)
	}
	return result
}

// GroupByBuild groups artifacts by build ID
func (m *Manager) GroupByBuild() map[string][]Artifact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]Artifact)
	for _, a := range m.artifacts {
		result[a.BuildID] = append(result[a.BuildID], a)
	}
	return result
}
