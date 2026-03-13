package main

import (
	"context"
	"os"
	"time"

	"proyecto-cursos/internal/platform/auth"
	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/internal/platform/mq"
	"proyecto-cursos/internal/platform/server"
	platformstore "proyecto-cursos/internal/platform/store"
	"proyecto-cursos/services/enrollments-api/internal/app"
	serviceconfig "proyecto-cursos/services/enrollments-api/internal/config"
	"proyecto-cursos/services/enrollments-api/internal/repository"
	"proyecto-cursos/services/enrollments-api/internal/service"
)

func main() {
	cfg, err := serviceconfig.Parse()
	if err != nil {
		logger.New("enrollments-api").Error(context.Background(), "config parse failed", map[string]any{"error": err.Error()})
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

	rabbitManager := mq.NewConnectionManager(cfg.RabbitMQURL)
	if err := rabbitManager.Connect(ctx, 30, 2*time.Second); err != nil {
		log.Error(context.Background(), "rabbitmq connection failed", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
	defer rabbitManager.Close()

	repo := repository.NewPostgresEnrollmentRepository(db)
	coursesClient := app.NewCoursesAPIClient(cfg.CoursesBaseURL, cfg.ServiceName, cfg.CoursesInternalToken)
	usersClient := app.NewUsersAPIClient(cfg.UsersBaseURL, cfg.ServiceName, cfg.UsersInternalToken)
	enrollmentService := service.NewEnrollmentService(repo, coursesClient, usersClient)
	runtimeCtx := context.Background()
	app.NewPaymentPaidConsumer(log, rabbitManager, enrollmentService, repo).Start(runtimeCtx)
	app.NewPaymentActivationIssueWorker(log, enrollmentService, repo, 30*time.Second, 20).Start(runtimeCtx)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, 24*time.Hour)
	router := app.NewHTTPHandler(log, enrollmentService, jwtManager, db, rabbitManager, cfg.InternalToken)

	if err := server.Run(cfg.Addr, router, log); err != nil {
		log.Error(context.Background(), "server stopped", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}
