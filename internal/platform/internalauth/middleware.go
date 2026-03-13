package internalauth

import (
	"net/http"
	"strings"

	"proyecto-cursos/internal/platform/api"
)

const Header = "X-Internal-Token"

func RequireToken(expected string) func(http.Handler) http.Handler {
	expected = strings.TrimSpace(expected)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if expected == "" || r.Header.Get(Header) != expected {
				api.WriteError(w, http.StatusUnauthorized, "INVALID_INTERNAL_TOKEN", "invalid internal token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
