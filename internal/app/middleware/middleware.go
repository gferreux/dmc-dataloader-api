// Package middleware holds HTTP middleware for the console API.
package middleware

import (
	"net/http"
	"strings"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/app/httpx"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/port"
)

const iapHeader = "X-Goog-IAP-JWT-Assertion"

// Authenticate rejects callers the configured authenticator does not accept.
func Authenticate(authn port.Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := authn.Authenticate(r.Context(), port.Credentials{
				IAPJWT: r.Header.Get(iapHeader),
			})
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication required", nil)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORS reflects configured origins. An empty list leaves browser cross-origin calls blocked.
func CORS(origins []string) func(http.Handler) http.Handler {
	allowed := map[string]struct{}{}
	allowAny := false

	for _, origin := range origins {
		if origin == "*" {
			allowAny = true
		}

		allowed[origin] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (allowAny || contains(allowed, origin)) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Add("Vary", "Access-Control-Request-Headers")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set(
					"Access-Control-Allow-Headers",
					"Content-Type, Authorization, "+iapHeader,
				)
				w.Header().Set("Access-Control-Expose-Headers", "Location")
				w.Header().Set("Access-Control-Max-Age", "600")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func contains(allowed map[string]struct{}, origin string) bool {
	_, ok := allowed[origin]

	return ok
}

// SplitOrigins parses a comma-separated CORS origin list.
func SplitOrigins(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")

	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	return origins
}
