// Package config provides configuration management for the sandbox server.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete server configuration.
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Storage StorageConfig `yaml:"storage"`
	Runtime RuntimeConfig `yaml:"runtime"`
	Sandbox SandboxConfig `yaml:"sandbox"`
	Logging LoggingConfig `yaml:"logging"`
}

// ServerConfig holds server address configuration.
type ServerConfig struct {
	GRPCAddr string `yaml:"grpc_addr"`
	HTTPAddr string `yaml:"http_addr"`
}

// StorageConfig holds storage path configuration.
type StorageConfig struct {
	CodebasePath string `yaml:"codebase_path"`
	MountPath    string `yaml:"mount_path"`
	DBPath       string `yaml:"db_path"`
}

// RuntimeConfig holds runtime configuration.
type RuntimeConfig struct {
	Type           string `yaml:"type"`
	BwrapPath      string `yaml:"bwrap_path"`
	DefaultTimeout string `yaml:"default_timeout"`
	MaxTimeout     string `yaml:"max_timeout"`
}

// SandboxConfig holds sandbox defaults configuration.
type SandboxConfig struct {
	DefaultExpiration string `yaml:"default_expiration"`
	MaxPerUser        int    `yaml:"max_per_user"`
	CleanupInterval   string `yaml:"cleanup_interval"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			GRPCAddr: ":9000",
			HTTPAddr: ":8080",
		},
		Storage: StorageConfig{
			CodebasePath: "/tmp/sandbox/codebases",
			MountPath:    "/tmp/sandbox/mounts",
			DBPath:       "/tmp/sandbox/metadata.db",
		},
		Runtime: RuntimeConfig{
			Type:           "mock",
			BwrapPath:      "bwrap",
			DefaultTimeout: "30s",
			MaxTimeout:     "10m",
		},
		Sandbox: SandboxConfig{
			DefaultExpiration: "24h",
			MaxPerUser:        10,
			CleanupInterval:   "5m",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// Load loads configuration from a YAML file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// LoadOrDefault loads configuration from a file, or returns default if file doesn't exist.
func LoadOrDefault(path string) (*Config, error) {
	if path == "" {
		return DefaultConfig(), nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	return Load(path)
}

// GetDefaultTimeout returns the default timeout as a time.Duration.
func (c *RuntimeConfig) GetDefaultTimeout() time.Duration {
	d, err := time.ParseDuration(c.DefaultTimeout)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

// GetMaxTimeout returns the max timeout as a time.Duration.
func (c *RuntimeConfig) GetMaxTimeout() time.Duration {
	d, err := time.ParseDuration(c.MaxTimeout)
	if err != nil {
		return 10 * time.Minute
	}
	return d
}

// GetDefaultExpiration returns the default expiration as a time.Duration.
func (c *SandboxConfig) GetDefaultExpiration() time.Duration {
	d, err := time.ParseDuration(c.DefaultExpiration)
	if err != nil {
		return 24 * time.Hour
	}
	return d
}

// GetCleanupInterval returns the cleanup interval as a time.Duration.
func (c *SandboxConfig) GetCleanupInterval() time.Duration {
	d, err := time.ParseDuration(c.CleanupInterval)
	if err != nil {
		return 5 * time.Minute
	}
	return d
}
