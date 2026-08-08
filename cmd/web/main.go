package main

import (
	"log/slog"
	"net/http"

	"github.com/jkeddari/hypermetrics/internal/app"
	"github.com/jkeddari/hypermetrics/internal/config"
	"github.com/jkeddari/hypermetrics/internal/logger"
	webhttp "github.com/jkeddari/hypermetrics/internal/web"
)

func main() {
	cfg := config.Load(".env.web")
	logger.Init(cfg.SentryDSN)

	webApp, err := app.NewWeb(cfg)
	if err != nil {
		slog.Error("failed to initialize web app", "error", err)
		panic(err)
	}
	defer webApp.Close()

	slog.Info("web server starting", "port", cfg.Port, "url", cfg.AppURL)
	if err := http.ListenAndServe(":"+cfg.Port, webhttp.SetupRoutes(webApp)); err != nil {
		slog.Error("web server failed", "error", err)
		panic(err)
	}
}
