package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jkeddari/hypermetrics/internal/api"
	"github.com/jkeddari/hypermetrics/internal/app"
	"github.com/jkeddari/hypermetrics/internal/config"
	"github.com/jkeddari/hypermetrics/internal/logger"
)

func main() {
	cfg := config.Load(".env.api")

	logger.Init(cfg.SentryDSN)

	app, err := app.NewAPI(cfg)
	if err != nil {
		slog.Error("failed to initialize api app", "error", err)
		panic(err)
	}
	defer func() {
		closeErr := app.Close()
		if closeErr != nil {
			slog.Error("failed to close api app", "error", closeErr)
		}
	}()

	app.RunHypercoreCollectors(context.Background())

	handler := api.SetupRoutes(app)
	slog.Info("api server starting", "port", cfg.Port, "url", "http://localhost:"+cfg.Port)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	err = server.ListenAndServe()
	if err != nil {
		slog.Error("api server failed", "error", err)
		panic(err)
	}
}
