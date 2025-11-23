package main

import (
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jkeddari/hypermetrics/internal/server"
)

func main() {
	// Parse command-line flags
	addr := flag.String("addr", ":8080", "Server listen address")
	flag.Parse()

	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Create server configuration
	config := server.DefaultConfig()
	config.Address = *addr
	config.Logger = logger

	// Create and start server
	srv, err := server.NewServer(config)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		logger.Info("server starting", "address", config.Address)
		if err := srv.Start(); err != nil {
			logger.Error("server error", "error", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info("shutdown signal received")

	// Stop server gracefully
	if err := srv.Stop(); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	logger.Info("server stopped")
}
