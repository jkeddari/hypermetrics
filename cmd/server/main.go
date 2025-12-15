package main

import (
	"log/slog"
	"net/http"

	"github.com/jkeddari/hypermetrics/internal/app"
	"github.com/jkeddari/hypermetrics/internal/config"
	"github.com/jkeddari/hypermetrics/internal/logger"
	"github.com/jkeddari/hypermetrics/internal/routes"
)

func main() {
	cfg := config.Load()

	logger.Init(cfg.IsDevelopment(), cfg.SentryDSN)

	app, err := app.New(cfg)
	if err != nil {
		slog.Error("failed to initialize app", "error", err)
		panic(err)
	}
	defer func() {
		closeErr := app.Close()
		if closeErr != nil {
			slog.Error("failed to close app", "error", closeErr)
		}
	}()

	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	handler := routes.SetupRoutes(app)
	slog.Info("server starting", "port", cfg.Port, "env", cfg.AppEnv, "url", "http://localhost:"+cfg.Port)

	err = http.ListenAndServe(":"+cfg.Port, handler)
	if err != nil {
		slog.Error("server failed", "error", err)
		panic(err)
	}
}
