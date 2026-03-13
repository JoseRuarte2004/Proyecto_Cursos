package middleware

import (
	"fmt"
	"net/http"

	"proyecto-cursos/internal/platform/api"
	"proyecto-cursos/internal/platform/logger"
)

func Recovery(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Error(r.Context(), "panic recovered", map[string]any{
						"panic": fmt.Sprint(recovered),
					})
					api.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "internal server error")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
