package httpserver

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"

	"boardgamesort/internal/httpx"
	"boardgamesort/internal/templates"
)

type middleware func(http.Handler) http.Handler

func ApplyMiddleware(handler http.Handler, middlewareChain ...middleware) http.Handler {
	for _, currentMiddleware := range slices.Backward(middlewareChain) {
		handler = currentMiddleware(handler)
	}
	return handler
}

func SecurityHeaders(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		nonce := httpx.CSPNonce(request.Context())
		responseWriter.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; img-src 'self' data:; frame-src 'self'; object-src 'none'; frame-ancestors 'self'; base-uri 'self'; form-action 'self'; script-src 'self' 'nonce-"+nonce+"';")
		responseWriter.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		responseWriter.Header().Set("X-Content-Type-Options", "nosniff")
		responseWriter.Header().Set("X-Frame-Options", "SAMEORIGIN")
		responseWriter.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		responseWriter.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		responseWriter.Header().Set("Origin-Agent-Cluster", "?1")
		responseWriter.Header().Set("X-DNS-Prefetch-Control", "off")
		responseWriter.Header().Set("X-Download-Options", "noopen")
		responseWriter.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
		responseWriter.Header().Set("X-XSS-Protection", "0")
		handler.ServeHTTP(responseWriter, request)
	})
}

func GlobalSourceValidation(appOrigin string, renderer *templates.Renderer) middleware {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			if request.Method != "POST" {
				handler.ServeHTTP(responseWriter, request)
				return
			}
			origin := request.Header.Get("Origin")
			ori := request.Header.Get("Referer")
			if ori != "" && origin == "" {
				url, err := url.Parse(ori)
				if err != nil {
					if err := httpx.RespondWithErrorPage(responseWriter, renderer, http.StatusForbidden, "Not the Origin", "Origin missing malformed or untrusted"); err != nil {
						http.Error(responseWriter, http.StatusText(http.StatusForbidden), http.StatusForbidden)

					}
					return
				}
				origin = url.Scheme + "://" + url.Host
			}
			if origin != "" && appOrigin == origin {
				handler.ServeHTTP(responseWriter, request)
				return
			} else {
				if err := httpx.RespondWithErrorPage(responseWriter, renderer, http.StatusForbidden, "Not the Origin", "Origin missing malformed or untrusted"); err != nil {
					http.Error(responseWriter, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				}
			}

		})
	}
}
func CspNonce(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		nonceBytes := make([]byte, 16)
		if _, err := rand.Read(nonceBytes); err != nil {
			http.Error(responseWriter, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		nonce := base64.StdEncoding.EncodeToString(nonceBytes)
		request = request.WithContext(httpx.WithCSPNonce(request.Context(), nonce))
		next.ServeHTTP(responseWriter, request)
	})
}

func RecoverPanics(logger *slog.Logger, renderer *templates.Renderer) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("unhandled_error",
						"method", request.Method,
						"path", request.URL.Path,
						"message", fmt.Sprint(recovered))
					if err := httpx.RespondWithErrorPage(responseWriter, renderer, http.StatusInternalServerError, "Unhandled Error", fmt.Sprint(recovered)); err != nil {
						http.Error(responseWriter, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					}
				}
			}()
			next.ServeHTTP(responseWriter, request)
		})
	}
}
