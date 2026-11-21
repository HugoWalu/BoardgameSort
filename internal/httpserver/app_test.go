package httpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubPinger struct {
	err error
}

func (p stubPinger) Ping(context.Context) error {
	return p.err
}

func newTestApplication(t *testing.T, pinger Pinger) *Application {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := New(pinger, logger, Options{
		AppOrigin:         "http://localhost:8080",
		TemplateDirectory: "../../web/templates",
		PublicDirectory:   "../../web/public",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return app
}

func TestHealthz(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		wantStatus int
	}{
		{
			name:       "database reachable",
			wantStatus: http.StatusOK,
		},
		{
			name:       "database unreachable",
			pingErr:    errors.New("database is down"),
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newTestApplication(t, stubPinger{err: tt.pingErr})
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/healthz", nil)

			app.Handler.ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
		})
	}
}

func TestHome(t *testing.T) {
	app := newTestApplication(t, stubPinger{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	app.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if body := response.Body.String(); !strings.Contains(body, "BoardgameSort") {
		t.Fatalf("body does not contain BoardgameSort: %q", body)
	}
}

func TestNotFound(t *testing.T) {
	app := newTestApplication(t, stubPinger{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)

	app.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
