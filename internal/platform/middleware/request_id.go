package middleware

import (
	"net/http"

	"proyecto-cursos/internal/platform/requestid"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentID := r.Header.Get(requestid.Header)
		if currentID == "" {
			currentID = requestid.New()
		}

		ctx := requestid.WithContext(r.Context(), currentID)
		w.Header().Set(requestid.Header, currentID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
