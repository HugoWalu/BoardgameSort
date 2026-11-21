package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"boardgamesort/internal/httpx"
	"boardgamesort/internal/templates"
)

type Pinger interface {
	Ping(context.Context) error
}

type Options struct {
	AppOrigin         string
	TemplateDirectory string
	PublicDirectory   string
}

type Application struct {
	Handler    http.Handler
	publicRoot *os.Root
}

func New(db Pinger, logger *slog.Logger, options Options) (*Application, error) {
	publicRoot, err := os.OpenRoot(options.PublicDirectory)
	if err != nil {
		return nil, fmt.Errorf("open public directory: %w", err)
	}

	renderer, err := templates.Load(options.TemplateDirectory)
	if err != nil {
		_ = publicRoot.Close()
		return nil, fmt.Errorf("load templates: %w", err)
	}

	mux := http.NewServeMux()
	dynamicMux := http.NewServeMux()
	dynamichandler := ApplyMiddleware(dynamicMux, GlobalSourceValidation(options.AppOrigin, renderer))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			logger.Error("healthz: database ping failed", "error", err)
			httpx.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]any{
				"ok": false, "app": "boardgamesort",
			})
			return
		}
		httpx.RespondWithJSON(w, http.StatusOK, map[string]any{"ok": true, "app": "boardgamesort"})
	})

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if err := renderer.Render(w, http.StatusOK, "home", templates.Page{Title: "BoardgameSort"}); err != nil {
			logger.Error("render home", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	})
	mux.Handle("/", dynamichandler)
	handler := ApplyMiddleware(
		mux,
		CspNonce,
		SecurityHeaders,
		RecoverPanics(logger, renderer))
	return &Application{Handler: handler, publicRoot: publicRoot}, nil
}

func (a *Application) Close() error {
	return a.publicRoot.Close()
}
