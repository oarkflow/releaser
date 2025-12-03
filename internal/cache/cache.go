// Package cache provides build caching for incremental builds.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

// Cache manages build artifact caching
type Cache struct {
	dir      string
	metadata map[string]*CacheEntry
	metaFile string
	mu       sync.RWMutex
}

// CacheEntry represents a cached build entry
type CacheEntry struct {
	Key       string            `json:"key"`
	Hash      string            `json:"hash"`
	Path      string            `json:"path"`
	CreatedAt time.Time         `json:"created_at"`
	ExpiresAt time.Time         `json:"expires_at"`
	Size      int64             `json:"size"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// CacheOptions configures cache behavior
type CacheOptions struct {
	Dir     string
	MaxSize int64         // Maximum cache size in bytes
	MaxAge  time.Duration // Maximum age of cached items
	Enabled bool
}

// DefaultOptions returns default cache options
func DefaultOptions() CacheOptions {
	homeDir, _ := os.UserHomeDir()
	return CacheOptions{
		Dir:     filepath.Join(homeDir, ".cache", "releaser"),
		MaxSize: 1 << 30,            // 1GB
		MaxAge:  7 * 24 * time.Hour, // 7 days
		Enabled: true,
	}
}

// New creates a new cache
func New(opts CacheOptions) (*Cache, error) {
	if !opts.Enabled {
		return nil, nil
	}

	if err := os.MkdirAll(opts.Dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache dir: %w", err)
	}

	c := &Cache{
		dir:      opts.Dir,
		metaFile: filepath.Join(opts.Dir, "metadata.json"),
		metadata: make(map[string]*CacheEntry),
	}

	// Load existing metadata
	c.loadMetadata()

	return c, nil
}

// loadMetadata loads cache metadata from disk
func (c *Cache) loadMetadata() {
	data, err := os.ReadFile(c.metaFile)
	if err != nil {
		return
	}
	meta := make(map[string]*CacheEntry)
	if err := json.Unmarshal(data, &meta); err != nil {
		return
	}
	c.mu.Lock()
	c.metadata = meta
	c.mu.Unlock()
}

// saveMetadata saves cache metadata to disk
func (c *Cache) saveMetadataLocked() error {
	data, err := json.MarshalIndent(c.metadata, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.metaFile, data, 0644)
}

// Key generates a cache key from inputs
func Key(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// HashFile computes the SHA256 hash of a file
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashDir computes a hash of a directory's contents
func HashDir(dir string, patterns ...string) (string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Check patterns
		if len(patterns) > 0 {
			matched := false
			for _, p := range patterns {
				if ok, _ := filepath.Match(p, filepath.Base(path)); ok {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		files = append(files, path)
		return nil
	})
	if err != nil {
		return "", err
	}

	// Sort for deterministic hashing
	sort.Strings(files)

	h := sha256.New()
	for _, f := range files {
		// Include file path
		h.Write([]byte(f))

		// Include file content
		data, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		h.Write(data)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// Get retrieves a cached item
func (c *Cache) Get(key string) (*CacheEntry, bool) {
	if c == nil {
		return nil, false
	}

	c.mu.RLock()
	entry, ok := c.metadata[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}

	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		c.Delete(key)
		return nil, false
	}

	// Check file exists
	if _, err := os.Stat(entry.Path); os.IsNotExist(err) {
		c.mu.Lock()
		delete(c.metadata, key)
		err := c.saveMetadataLocked()
		c.mu.Unlock()
		if err != nil {
			log.Warn("failed to save cache metadata", "error", err)
		}
		return nil, false
	}

	log.Debug("Cache hit", "key", key)
	return entry, true
}

// Put stores an item in the cache
func (c *Cache) Put(key string, sourcePath string, ttl time.Duration, metadata map[string]string) (*CacheEntry, error) {
	if c == nil {
		return nil, nil
	}

	// Compute hash
	hash, err := HashFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to hash file: %w", err)
	}

	// Copy to cache
	destPath := filepath.Join(c.dir, key+"_"+filepath.Base(sourcePath))
	if err := copyFile(sourcePath, destPath); err != nil {
		return nil, fmt.Errorf("failed to copy to cache: %w", err)
	}

	info, _ := os.Stat(destPath)
	entry := &CacheEntry{
		Key:       key,
		Hash:      hash,
		Path:      destPath,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
		Size:      info.Size(),
		Metadata:  metadata,
	}

	c.mu.Lock()
	c.metadata[key] = entry
	if err := c.saveMetadataLocked(); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	c.mu.Unlock()

	log.Debug("Cached", "key", key, "path", destPath)
	return entry, nil
}

// PutBytes stores byte data in the cache
func (c *Cache) PutBytes(key string, name string, data []byte, ttl time.Duration) (*CacheEntry, error) {
	if c == nil {
		return nil, nil
	}

	destPath := filepath.Join(c.dir, key+"_"+name)
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return nil, err
	}

	h := sha256.Sum256(data)
	entry := &CacheEntry{
		Key:       key,
		Hash:      hex.EncodeToString(h[:]),
		Path:      destPath,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
		Size:      int64(len(data)),
	}

	c.mu.Lock()
	c.metadata[key] = entry
	if err := c.saveMetadataLocked(); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	c.mu.Unlock()

	return entry, nil
}

// GetPath returns the cached file path if valid
func (c *Cache) GetPath(key string) (string, bool) {
	entry, ok := c.Get(key)
	if !ok {
		return "", false
	}
	return entry.Path, true
}

// Delete removes an item from the cache
func (c *Cache) Delete(key string) error {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	entry, ok := c.metadata[key]
	if !ok {
		c.mu.Unlock()
		return nil
	}

	os.Remove(entry.Path)
	delete(c.metadata, key)
	err := c.saveMetadataLocked()
	c.mu.Unlock()
	return err
}

// Clear removes all items from the cache
func (c *Cache) Clear() error {
	if c == nil {
		return nil
	}

	c.mu.RLock()
	keys := make([]string, 0, len(c.metadata))
	for key := range c.metadata {
		keys = append(keys, key)
	}
	c.mu.RUnlock()

	for _, key := range keys {
		c.Delete(key)
	}

	// Also remove any orphaned files
	entries, _ := os.ReadDir(c.dir)
	for _, e := range entries {
		if e.Name() != "metadata.json" {
			os.Remove(filepath.Join(c.dir, e.Name()))
		}
	}

	return nil
}

// Prune removes expired items
func (c *Cache) Prune() error {
	if c == nil {
		return nil
	}

	now := time.Now()
	c.mu.RLock()
	var keys []string
	for key, entry := range c.metadata {
		if now.After(entry.ExpiresAt) {
			keys = append(keys, key)
		}
	}
	c.mu.RUnlock()

	for _, key := range keys {
		c.Delete(key)
	}

	return nil
}

// Size returns the total cache size in bytes
func (c *Cache) Size() int64 {
	if c == nil {
		return 0
	}

	var total int64
	c.mu.RLock()
	for _, entry := range c.metadata {
		total += entry.Size
	}
	c.mu.RUnlock()
	return total
}

// Stats returns cache statistics
func (c *Cache) Stats() map[string]interface{} {
	if c == nil {
		return nil
	}

	var expired int
	now := time.Now()
	c.mu.RLock()
	for _, entry := range c.metadata {
		if now.After(entry.ExpiresAt) {
			expired++
		}
	}
	totalEntries := len(c.metadata)
	c.mu.RUnlock()

	return map[string]interface{}{
		"entries":    totalEntries,
		"expired":    expired,
		"size_bytes": c.Size(),
		"size_human": formatBytes(c.Size()),
		"cache_dir":  c.dir,
	}
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	dest, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dest.Close()

	_, err = io.Copy(dest, source)
	return err
}

// formatBytes formats bytes as human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// BuildCache provides build-specific caching
type BuildCache struct {
	cache *Cache
}

// NewBuildCache creates a new build cache
func NewBuildCache(opts CacheOptions) (*BuildCache, error) {
	c, err := New(opts)
	if err != nil {
		return nil, err
	}
	return &BuildCache{cache: c}, nil
}

// BuildKey generates a cache key for a build
func (bc *BuildCache) BuildKey(goos, goarch, binary string, sources []string) string {
	parts := []string{goos, goarch, binary}

	// Hash source files
	for _, src := range sources {
		if hash, err := HashFile(src); err == nil {
			parts = append(parts, hash[:8])
		}
	}

	return Key(parts...)
}

// GetBinary retrieves a cached binary
func (bc *BuildCache) GetBinary(key string) (string, bool) {
	if bc == nil || bc.cache == nil {
		return "", false
	}
	return bc.cache.GetPath(key)
}

// PutBinary stores a binary in the cache
func (bc *BuildCache) PutBinary(key string, binaryPath string, goos, goarch string) error {
	if bc == nil || bc.cache == nil {
		return nil
	}

	_, err := bc.cache.Put(key, binaryPath, 24*time.Hour, map[string]string{
		"goos":   goos,
		"goarch": goarch,
	})
	return err
}

// SourceHash computes a hash of source files
func (bc *BuildCache) SourceHash(patterns ...string) (string, error) {
	dir, _ := os.Getwd()
	return HashDir(dir, patterns...)
}

// ContentHasher provides content-based hashing
type ContentHasher struct {
	patterns []string
	exclude  []string
}

// NewContentHasher creates a new content hasher
func NewContentHasher(patterns, exclude []string) *ContentHasher {
	return &ContentHasher{
		patterns: patterns,
		exclude:  exclude,
	}
}

// Hash computes the hash of matching files
func (h *ContentHasher) Hash(dir string) (string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Check exclude
			for _, ex := range h.exclude {
				if strings.Contains(path, ex) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check patterns
		matched := len(h.patterns) == 0
		for _, p := range h.patterns {
			if ok, _ := filepath.Match(p, filepath.Base(path)); ok {
				matched = true
				break
			}
		}

		// Check exclude
		for _, ex := range h.exclude {
			if strings.Contains(path, ex) {
				return nil
			}
		}

		if matched {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(files)

	hash := sha256.New()
	for _, f := range files {
		hash.Write([]byte(f))
		if data, err := os.ReadFile(f); err == nil {
			hash.Write(data)
		}
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
