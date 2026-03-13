package main

import (
	"context"
	"os"
	"time"

	"proyecto-cursos/internal/platform/auth"
	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/internal/platform/server"
	platformstore "proyecto-cursos/internal/platform/store"
	"proyecto-cursos/services/course-content-api/internal/app"
	serviceconfig "proyecto-cursos/services/course-content-api/internal/config"
	"proyecto-cursos/services/course-content-api/internal/repository"
	"proyecto-cursos/services/course-content-api/internal/service"
)

func main() {
	cfg, err := serviceconfig.Parse()
	if err != nil {
		logger.New("course-content-api").Error(context.Background(), "config parse failed", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
	log := logger.New(cfg.ServiceName)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := platformstore.ConnectPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error(context.Background(), "postgres connection failed", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
	defer db.Close()

	if err := platformstore.WaitForPostgres(ctx, db, 30, 2*time.Second); err != nil {
		log.Error(context.Background(), "postgres not ready", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	if err := app.RunMigrations(ctx, db); err != nil {
		log.Error(context.Background(), "migrations failed", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	repo := repository.NewPostgresLessonRepository(db)
	assignments := app.NewCoursesAPIClient(cfg.CoursesAPIBaseURL, cfg.ServiceName, cfg.CoursesInternalToken)
	enrollments := app.NewEnrollmentsAPIClient(cfg.EnrollmentsAPIBaseURL, cfg.ServiceName, cfg.EnrollInternalToken)
	lessonService := service.NewLessonService(repo, assignments, enrollments)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, 24*time.Hour)
	router := app.NewHTTPHandler(log, lessonService, jwtManager, db)

	if err := server.Run(cfg.Addr, router, log); err != nil {
		log.Error(context.Background(), "server stopped", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}
