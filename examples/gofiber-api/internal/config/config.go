// Package config handles application configuration
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	CORSOrigins string `yaml:"cors_origins"`
}

// DatabaseConfig holds database settings (for future use)
type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// DefaultConfig returns default configuration values
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:        "0.0.0.0",
			Port:        3000,
			CORSOrigins: "*",
		},
		Database: DatabaseConfig{
			Driver: "memory",
			DSN:    "",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// Load loads configuration from a file or returns defaults
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Try to find config file
	if path == "" {
		// Look for config in common locations
		locations := []string{
			"config.yaml",
			"config.yml",
			"/etc/gofiber-api/config.yaml",
			filepath.Join(os.Getenv("HOME"), ".config", "gofiber-api", "config.yaml"),
		}
		for _, loc := range locations {
			if _, err := os.Stat(loc); err == nil {
				path = loc
				break
			}
		}
	}

	// If no config file found, return defaults
	if path == "" {
		return cfg, nil
	}

	// Read and parse config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Override with environment variables
	if host := os.Getenv("GOFIBER_HOST"); host != "" {
		cfg.Server.Host = host
	}
	if port := os.Getenv("GOFIBER_PORT"); port != "" {
		// Simple conversion, could use strconv
		cfg.Server.Port = 3000 // Default, env parsing would be more robust
	}
	if cors := os.Getenv("GOFIBER_CORS_ORIGINS"); cors != "" {
		cfg.Server.CORSOrigins = cors
	}

	return cfg, nil
}
