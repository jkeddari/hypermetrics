package logger

import (
	"log/slog"
	"os"

	slogmulti "github.com/samber/slog-multi"
	slogsentry "github.com/samber/slog-sentry/v2"
	"github.com/getsentry/sentry-go"
)

// Log is the global logger instance
var Log *slog.Logger

// Init initializes the global logger based on environment
// Development: Text format with Debug level
// Production: JSON format with Info level
// Optionally sends errors to Sentry for error tracking
func Init(isDev bool, sentryDSN string) {
	var level slog.Level
	var handlers []slog.Handler

	// Base handler for stdout (always enabled)
	if isDev {
		level = slog.LevelDebug
		handlers = append(handlers, slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		}))
	} else {
		level = slog.LevelInfo
		handlers = append(handlers, slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		}))
	}

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
