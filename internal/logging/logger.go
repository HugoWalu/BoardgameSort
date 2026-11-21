package logging

import (
	"log/slog"
	"os"
)

// New returns the application logger.
//
// Cloud Run sets K_SERVICE, and Cloud Logging parses JSON lines from stdout, so
// production gets JSON. Locally, text is easier to read in a terminal. Logs
// always go to stdout: the container filesystem is ephemeral and is not where
// anything should be collected from.
func New() *slog.Logger {
	if os.Getenv("K_SERVICE") != "" {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}
