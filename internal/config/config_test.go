package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.GRPCAddr != ":9000" {
		t.Errorf("expected gRPC addr :9000, got %s", cfg.Server.GRPCAddr)
	}
	if cfg.Server.HTTPAddr != ":8080" {
		t.Errorf("expected HTTP addr :8080, got %s", cfg.Server.HTTPAddr)
	}
	if cfg.Runtime.Type != "mock" {
		t.Errorf("expected runtime type mock, got %s", cfg.Runtime.Type)
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
server:
  grpc_addr: ":9001"
  http_addr: ":8081"
storage:
  codebase_path: "/custom/codebases"
  mount_path: "/custom/mounts"
runtime:
  type: "bwrap"
  bwrap_path: "/usr/local/bin/bwrap"
  default_timeout: "60s"
sandbox:
  max_per_user: 20
logging:
  level: "debug"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Server.GRPCAddr != ":9001" {
		t.Errorf("expected gRPC addr :9001, got %s", cfg.Server.GRPCAddr)
	}
	if cfg.Server.HTTPAddr != ":8081" {
		t.Errorf("expected HTTP addr :8081, got %s", cfg.Server.HTTPAddr)
	}
	if cfg.Storage.CodebasePath != "/custom/codebases" {
		t.Errorf("expected codebase path /custom/codebases, got %s", cfg.Storage.CodebasePath)
	}
	if cfg.Runtime.Type != "bwrap" {
		t.Errorf("expected runtime type bwrap, got %s", cfg.Runtime.Type)
	}
	if cfg.Runtime.BwrapPath != "/usr/local/bin/bwrap" {
		t.Errorf("expected bwrap path /usr/local/bin/bwrap, got %s", cfg.Runtime.BwrapPath)
	}
	if cfg.Sandbox.MaxPerUser != 20 {
		t.Errorf("expected max_per_user 20, got %d", cfg.Sandbox.MaxPerUser)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.Logging.Level)
	}
}

func TestLoadOrDefault(t *testing.T) {
	// Test with non-existent file
	cfg, err := LoadOrDefault("/nonexistent/path.yaml")
	if err != nil {
		t.Fatalf("LoadOrDefault should not error for non-existent file: %v", err)
	}
	if cfg.Runtime.Type != "mock" {
		t.Errorf("expected default runtime type mock, got %s", cfg.Runtime.Type)
	}

	// Test with empty path
	cfg, err = LoadOrDefault("")
	if err != nil {
		t.Fatalf("LoadOrDefault should not error for empty path: %v", err)
	}
	if cfg.Server.GRPCAddr != ":9000" {
		t.Errorf("expected default gRPC addr :9000, got %s", cfg.Server.GRPCAddr)
	}
}

func TestRuntimeConfigDurations(t *testing.T) {
	cfg := &RuntimeConfig{
		DefaultTimeout: "45s",
		MaxTimeout:     "15m",
	}

	if cfg.GetDefaultTimeout() != 45*time.Second {
		t.Errorf("expected 45s, got %v", cfg.GetDefaultTimeout())
	}
	if cfg.GetMaxTimeout() != 15*time.Minute {
		t.Errorf("expected 15m, got %v", cfg.GetMaxTimeout())
	}

	// Test invalid duration fallback
	cfg.DefaultTimeout = "invalid"
	if cfg.GetDefaultTimeout() != 30*time.Second {
		t.Errorf("expected fallback 30s, got %v", cfg.GetDefaultTimeout())
	}
}

func TestSandboxConfigDurations(t *testing.T) {
	cfg := &SandboxConfig{
		DefaultExpiration: "48h",
		CleanupInterval:   "10m",
	}

	if cfg.GetDefaultExpiration() != 48*time.Hour {
		t.Errorf("expected 48h, got %v", cfg.GetDefaultExpiration())
	}
	if cfg.GetCleanupInterval() != 10*time.Minute {
		t.Errorf("expected 10m, got %v", cfg.GetCleanupInterval())
	}
}
