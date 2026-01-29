// Package main provides the entry point for the sandbox server.
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ajaxzhan/sandbox-rls/internal/codebase"
	"github.com/ajaxzhan/sandbox-rls/internal/config"
	"github.com/ajaxzhan/sandbox-rls/internal/runtime"
	"github.com/ajaxzhan/sandbox-rls/internal/runtime/bwrap"
	"github.com/ajaxzhan/sandbox-rls/internal/runtime/mock"
	"github.com/ajaxzhan/sandbox-rls/internal/server"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "", "Path to configuration file (YAML)")
	grpcAddr := flag.String("grpc-addr", "", "gRPC server address (overrides config)")
	httpAddr := flag.String("http-addr", "", "HTTP server address (overrides config)")
	codebasePath := flag.String("codebase-path", "", "Base path for codebase storage (overrides config)")
	runtimeType := flag.String("runtime", "", "Runtime type: bwrap, mock (overrides config)")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadOrDefault(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

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

	log.Printf("Starting sandbox server...")
	log.Printf("Configuration: gRPC=%s, HTTP=%s, Runtime=%s",
		cfg.Server.GRPCAddr, cfg.Server.HTTPAddr, cfg.Runtime.Type)

	// Create codebase directory if it doesn't exist
	if err := os.MkdirAll(cfg.Storage.CodebasePath, 0755); err != nil {
		log.Fatalf("Failed to create codebase directory: %v", err)
	}

	// Create codebase manager
	cbManager, err := codebase.NewManager(cfg.Storage.CodebasePath)
	if err != nil {
		log.Fatalf("Failed to create codebase manager: %v", err)
	}
	log.Printf("Codebase manager initialized at %s", cfg.Storage.CodebasePath)

	// Create runtime based on configuration
	rt := createRuntime(cfg)
	log.Printf("Using %s runtime", rt.Name())

	// Create server config
	serverCfg := &server.Config{
		GRPCAddr: cfg.Server.GRPCAddr,
		RESTAddr: cfg.Server.HTTPAddr,
	}

	// Create gRPC server
	srv, err := server.New(serverCfg, rt, cbManager)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Printf("Shutting down server...")
		srv.Stop()
	}()

	// Start server with REST gateway if HTTP address is configured
	if cfg.Server.HTTPAddr != "" {
		log.Printf("gRPC server listening on %s", cfg.Server.GRPCAddr)
		log.Printf("REST gateway listening on %s", cfg.Server.HTTPAddr)
		if err := srv.StartWithGateway(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	} else {
		log.Printf("gRPC server listening on %s", cfg.Server.GRPCAddr)
		if err := srv.Start(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}
}

// createRuntime creates a runtime instance based on configuration.
func createRuntime(cfg *config.Config) runtime.RuntimeWithExecutor {
	switch cfg.Runtime.Type {
	case "bwrap":
		bwrapCfg := &bwrap.Config{
			BwrapPath:      cfg.Runtime.BwrapPath,
			DefaultTimeout: cfg.Runtime.GetDefaultTimeout(),
			WorkDir:        cfg.Storage.MountPath,
		}
		return bwrap.New(bwrapCfg)
	case "mock":
		return mock.New()
	default:
		log.Printf("Unknown runtime type '%s', falling back to mock", cfg.Runtime.Type)
		return mock.New()
	}
}
