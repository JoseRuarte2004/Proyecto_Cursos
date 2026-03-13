package main

import (
	"context"
	"os"
	"time"

	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/internal/platform/server"
	platformstore "proyecto-cursos/internal/platform/store"
	"proyecto-cursos/services/chat-api/internal/app"
	serviceconfig "proyecto-cursos/services/chat-api/internal/config"
	"proyecto-cursos/services/chat-api/internal/repository"
	"proyecto-cursos/services/chat-api/internal/service"
	"proyecto-cursos/services/chat-api/internal/ws"
)

func main() {
	cfg, err := serviceconfig.Parse()
	if err != nil {
		logger.New("chat-api").Error(context.Background(), "config parse failed", map[string]any{"error": err.Error()})
		os.Exit(1)
	}

	log := logger.New(cfg.ServiceName)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := platformstore.ConnectPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error(context.Background(), "postgres connection failed", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
	defer db.Close()

	if err := platformstore.WaitForPostgres(ctx, db, 30, 2*time.Second); err != nil {
		log.Error(context.Background(), "postgres not ready", map[string]any{"error": err.Error()})
		os.Exit(1)
	}

	if err := app.RunMigrations(ctx, db); err != nil {
		log.Error(context.Background(), "migrations failed", map[string]any{"error": err.Error()})
		os.Exit(1)
	}

	repo := repository.NewPostgresMessageRepository(db)
	chatService := service.NewChatService(repo)
	coursesClient := app.NewCoursesAPIClient(cfg.CoursesAPIBaseURL, cfg.ServiceName, cfg.CoursesInternalToken)
	enrollmentsClient := app.NewEnrollmentsAPIClient(cfg.EnrollmentsAPIBaseURL, cfg.ServiceName, cfg.EnrollmentsInternalToken)
	usersClient := app.NewUsersAPIClient(cfg.UsersAPIBaseURL, cfg.ServiceName, cfg.UsersInternalToken)
	accessService := app.NewCourseAccessService(coursesClient, enrollmentsClient, usersClient)

	hub := ws.NewHub(log)
	hubCtx, stopHub := context.WithCancel(context.Background())
	defer stopHub()
	go hub.Run(hubCtx)

	tokenValidator := app.NewJWTValidator(cfg.JWTSecret)
	router := app.NewHTTPHandler(log, db, chatService, accessService, usersClient, hub, tokenValidator)

	if err := server.Run(cfg.Addr, router, log); err != nil {
		log.Error(context.Background(), "server stopped", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
}
