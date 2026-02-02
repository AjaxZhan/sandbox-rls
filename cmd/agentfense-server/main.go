// Package main provides the entry point for the sandbox server.
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/AjaxZhan/AgentFense/internal/codebase"
	"github.com/AjaxZhan/AgentFense/internal/config"
	"github.com/AjaxZhan/AgentFense/internal/logging"
	"github.com/AjaxZhan/AgentFense/internal/runtime"
	"github.com/AjaxZhan/AgentFense/internal/runtime/bwrap"
	"github.com/AjaxZhan/AgentFense/internal/runtime/docker"
	"github.com/AjaxZhan/AgentFense/internal/runtime/mock"
	"github.com/AjaxZhan/AgentFense/internal/server"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "", "Path to configuration file (YAML)")
	grpcAddr := flag.String("grpc-addr", "", "gRPC server address (overrides config)")
	httpAddr := flag.String("http-addr", "", "HTTP server address (overrides config)")
	codebasePath := flag.String("codebase-path", "", "Base path for codebase storage (overrides config)")
	runtimeType := flag.String("runtime", "", "Runtime type: bwrap, docker, mock (overrides config)")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadOrDefault(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logging system
	if err := logging.Init(&logging.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	}); err != nil {
		log.Fatalf("Failed to initialize logging: %v", err)
	}
	defer logging.Sync()

	// Apply command-line overrides
	if *grpcAddr != "" {
		cfg.Server.GRPCAddr = *grpcAddr
	}
	if *httpAddr != "" {
		cfg.Server.HTTPAddr = *httpAddr
	}
	if *codebasePath != "" {
		cfg.Storage.CodebasePath = *codebasePath
	}
	if *runtimeType != "" {
		cfg.Runtime.Type = *runtimeType
	}

	// Convert storage paths to absolute paths (required for Docker bind mounts)
	if err := normalizeStoragePaths(cfg); err != nil {
		logging.Fatal("Failed to normalize storage paths", logging.Err(err))
	}

	logging.Info("Starting sandbox server...",
		logging.String("grpc_addr", cfg.Server.GRPCAddr),
		logging.String("http_addr", cfg.Server.HTTPAddr),
		logging.String("runtime", cfg.Runtime.Type),
	)

	// Create codebase directory if it doesn't exist
	if err := os.MkdirAll(cfg.Storage.CodebasePath, 0755); err != nil {
		logging.Fatal("Failed to create codebase directory", logging.Err(err))
	}

	// Create codebase manager
	cbManager, err := codebase.NewManager(cfg.Storage.CodebasePath)
	if err != nil {
		logging.Fatal("Failed to create codebase manager", logging.Err(err))
	}
	logging.Info("Codebase manager initialized", logging.String("path", cfg.Storage.CodebasePath))

	// Create runtime based on configuration
	rt := createRuntime(cfg)
	logging.Info("Runtime initialized", logging.String("runtime", rt.Name()))

	// Create server config
	serverCfg := &server.Config{
		GRPCAddr: cfg.Server.GRPCAddr,
		RESTAddr: cfg.Server.HTTPAddr,
	}

	// Create gRPC server
	srv, err := server.New(serverCfg, rt, cbManager)
	if err != nil {
		logging.Fatal("Failed to create server", logging.Err(err))
	}

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logging.Info("Shutting down server...")
		srv.Stop()
	}()

	// Start server with REST gateway if HTTP address is configured
	if cfg.Server.HTTPAddr != "" {
		logging.Info("gRPC server listening", logging.String("addr", cfg.Server.GRPCAddr))
		logging.Info("REST gateway listening", logging.String("addr", cfg.Server.HTTPAddr))
		if err := srv.StartWithGateway(); err != nil {
			logging.Fatal("Server failed", logging.Err(err))
		}
	} else {
		logging.Info("gRPC server listening", logging.String("addr", cfg.Server.GRPCAddr))
		if err := srv.Start(); err != nil {
			logging.Fatal("Server failed", logging.Err(err))
		}
	}
}

// normalizeStoragePaths converts all storage paths to absolute paths.
// This is required for Docker bind mounts which must use absolute paths.
func normalizeStoragePaths(cfg *config.Config) error {
	var err error
	
	// Convert CodebasePath to absolute
	if cfg.Storage.CodebasePath != "" && !filepath.IsAbs(cfg.Storage.CodebasePath) {
		cfg.Storage.CodebasePath, err = filepath.Abs(cfg.Storage.CodebasePath)
		if err != nil {
			return err
		}
	}
	
	// Convert MountPath to absolute
	if cfg.Storage.MountPath != "" && !filepath.IsAbs(cfg.Storage.MountPath) {
		cfg.Storage.MountPath, err = filepath.Abs(cfg.Storage.MountPath)
		if err != nil {
			return err
		}
	}
	
	// Convert DBPath to absolute
	if cfg.Storage.DBPath != "" && !filepath.IsAbs(cfg.Storage.DBPath) {
		cfg.Storage.DBPath, err = filepath.Abs(cfg.Storage.DBPath)
		if err != nil {
			return err
		}
	}
	
	return nil
}

// createRuntime creates a runtime instance based on configuration.
func createRuntime(cfg *config.Config) runtime.RuntimeWithExecutor {
	switch cfg.Runtime.Type {
	case "bwrap":
		bwrapCfg := &bwrap.Config{
			BwrapPath:      cfg.Runtime.BwrapPath,
			DefaultTimeout: cfg.Runtime.GetDefaultTimeout(),
			WorkDir:        cfg.Storage.MountPath,
			FUSEMountBase:  cfg.Storage.MountPath + "/fuse",
		}
		return bwrap.New(bwrapCfg)
	case "docker":
		dockerCfg := &docker.Config{
			DockerHost:       cfg.Runtime.Docker.Host,
			DefaultImage:     cfg.Runtime.Docker.DefaultImage,
			DefaultTimeout:   cfg.Runtime.GetDefaultTimeout(),
			FUSEMountBase:    cfg.Storage.MountPath + "/fuse",
			NetworkMode:      cfg.Runtime.Docker.NetworkMode,
			EnableNetworking: cfg.Runtime.Docker.EnableNetworking,
		}
		rt, err := docker.New(dockerCfg)
		if err != nil {
			logging.Fatal("Failed to create Docker runtime", logging.Err(err))
		}
		return rt
	case "mock":
		return mock.New()
	default:
		logging.Warn("Unknown runtime type, falling back to mock",
			logging.String("runtime_type", cfg.Runtime.Type),
		)
		return mock.New()
	}
}
