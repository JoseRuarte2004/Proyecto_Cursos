package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"proyecto-cursos/internal/platform/api"
	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/internal/platform/metrics"
	"proyecto-cursos/internal/platform/middleware"
)

func NewRouter(serviceName string, log *logger.Logger, readyFn func(context.Context) error) chi.Router {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logging(log))
	router.Use(metrics.Middleware(serviceName))
	router.Use(middleware.Recovery(log))

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	router.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if readyFn != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()

			if err := readyFn(ctx); err != nil {
				log.Error(r.Context(), "service not ready", map[string]any{
					"error": err.Error(),
				})
				api.WriteError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "service unavailable")
				return
			}
		}

		api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Handle("/metrics", metrics.Handler())

	return router
}

func Run(addr string, handler http.Handler, log *logger.Logger) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Info(context.Background(), "server starting", map[string]any{
		"addr": addr,
	})

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
