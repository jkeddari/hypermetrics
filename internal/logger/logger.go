package logger

import (
	"log/slog"
	"os"

	"github.com/getsentry/sentry-go"
	slogmulti "github.com/samber/slog-multi"
	slogsentry "github.com/samber/slog-sentry/v2"
)

// Log is the global logger instance
var Log *slog.Logger

// Init configures structured logs and optional Sentry error reporting.
func Init(sentryDSN string) {
	handlers := []slog.Handler{slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})}

	// Optional Sentry handler (sends errors only)
	if sentryDSN != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:              sentryDSN,
			TracesSampleRate: 1.0,
		})
		if err == nil {
			handlers = append(handlers, slogsentry.Option{
				Level: slog.LevelError,
			}.NewSentryHandler())

			// Test Sentry connection
			sentry.CaptureMessage("Sentry initialized successfully")
		}
	}

	// Use multi-handler if we have multiple, otherwise use single
	var handler slog.Handler
	if len(handlers) > 1 {
		handler = slogmulti.Fanout(handlers...)
	} else {
		handler = handlers[0]
	}

	Log = slog.New(handler)
	slog.SetDefault(Log)
}
