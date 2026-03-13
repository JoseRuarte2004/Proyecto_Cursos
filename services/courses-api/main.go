package main

import (
	"context"
	"os"
	"time"

	"proyecto-cursos/internal/platform/auth"
	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/internal/platform/server"
	platformstore "proyecto-cursos/internal/platform/store"
	"proyecto-cursos/services/courses-api/internal/app"
	serviceconfig "proyecto-cursos/services/courses-api/internal/config"
	"proyecto-cursos/services/courses-api/internal/repository"
	"proyecto-cursos/services/courses-api/internal/service"
)

func main() {
	cfg, err := serviceconfig.Parse()
	if err != nil {
		logger.New("courses-api").Error(context.Background(), "config parse failed", map[string]any{"error": err.Error()})
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

	redisClient, err := platformstore.ConnectRedis(cfg.RedisURL)
	if err != nil {
		log.Error(context.Background(), "redis connection failed", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
	defer redisClient.Close()

	repo := repository.NewPostgresCourseRepository(db)
	if err := app.EnsureBootstrapCatalog(ctx, repo, log); err != nil {
		log.Error(context.Background(), "bootstrap catalog failed", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
	usersClient := app.NewUsersAPIClient(cfg.UsersAPIBaseURL, cfg.ServiceName, cfg.UsersInternalToken)
	enrollmentsClient := app.NewEnrollmentsAPIClient(cfg.EnrollmentsAPIBaseURL, cfg.ServiceName, cfg.EnrollmentsInternalToken)
	courseService := service.NewCourseService(repo, usersClient, enrollmentsClient)
	recommendationAIClient := app.NewOpenAIRecommendationClient(cfg.OpenAIAPIKey, cfg.OpenAIModel)
	recommendationService := service.NewRecommendationService(repo, repo, usersClient, enrollmentsClient, recommendationAIClient)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, 24*time.Hour)
	cache := app.NewCourseCache(redisClient, cfg.CacheTTL)
	router := app.NewHTTPHandler(log, courseService, recommendationService, jwtManager, db, redisClient, cache, cfg.InternalToken)

	if err := server.Run(cfg.Addr, router, log); err != nil {
		log.Error(context.Background(), "server stopped", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}
