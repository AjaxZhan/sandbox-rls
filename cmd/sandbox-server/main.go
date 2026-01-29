// Package main provides the entry point for the sandbox server.
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ajaxzhan/sandbox-rls/internal/codebase"
	"github.com/ajaxzhan/sandbox-rls/internal/runtime/mock"
	"github.com/ajaxzhan/sandbox-rls/internal/server"
)

func main() {
	// Parse flags
	grpcAddr := flag.String("grpc-addr", ":9000", "gRPC server address")
	codebasePath := flag.String("codebase-path", "/tmp/sandbox/codebases", "Base path for codebase storage")
	flag.Parse()

	log.Printf("Starting sandbox server...")

	// Create codebase directory if it doesn't exist
	if err := os.MkdirAll(*codebasePath, 0755); err != nil {
		log.Fatalf("Failed to create codebase directory: %v", err)
	}

	// Create codebase manager
	cbManager, err := codebase.NewManager(*codebasePath)
	if err != nil {
		log.Fatalf("Failed to create codebase manager: %v", err)
	}
	log.Printf("Codebase manager initialized at %s", *codebasePath)

	// Create runtime
	// For now, use mock runtime. In production, use bwrap runtime on Linux.
	// TODO: Add bwrap runtime support with runtime selection flag
	rt := mock.New()
	log.Printf("Using %s runtime", rt.Name())

	// Create server config
	cfg := &server.Config{
		GRPCAddr: *grpcAddr,
	}

	// Create gRPC server
	srv, err := server.New(cfg, rt, cbManager)
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

	// Start server
	log.Printf("gRPC server listening on %s", *grpcAddr)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
