package main

import (
	"context"
	"os"
	"time"

	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/internal/platform/server"
	platformstore "proyecto-cursos/internal/platform/store"
	"proyecto-cursos/services/users-api/internal/app"
	serviceconfig "proyecto-cursos/services/users-api/internal/config"
	"proyecto-cursos/services/users-api/internal/repository"
	"proyecto-cursos/services/users-api/internal/service"
)

func main() {
	cfg, err := serviceconfig.Parse()
	if err != nil {
		logger.New("users-api").Error(context.Background(), "config parse failed", map[string]any{"error": err.Error()})
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

	userRepo := repository.NewPostgresUserRepository(db)
	auditRepo := repository.NewPostgresAuditLogRepository(db)
	verificationCodeStore := repository.NewRedisVerificationCodeStore(redisClient)
	pendingRegistrationStore := repository.NewRedisPendingRegistrationStore(redisClient)
	passwordResetCodeStore := repository.NewRedisVerificationCodeStoreWithPrefix(redisClient, "password_reset_code:")
	jwtManager := app.NewJWTManager(cfg.JWTSecret, cfg.JWTTTL)
	passwordManager := service.BcryptPasswordManager{}
	mailer, err := app.NewMailer(cfg, log)
	if err != nil {
		log.Error(context.Background(), "mailer setup failed", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	if err := app.EnsureBootstrapAdmin(ctx, cfg, userRepo, passwordManager, log); err != nil {
		log.Error(context.Background(), "bootstrap admin failed", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	userService := service.NewUserService(
		userRepo,
		auditRepo,
		passwordManager,
		jwtManager,
		cfg.EmailVerifyTTL,
		cfg.PasswordResetTTL,
		cfg.RequireEmailVerificationForLogin,
		service.WithVerificationCodeStore(verificationCodeStore, 15*time.Minute),
		service.WithPendingRegistrationStore(pendingRegistrationStore, 15*time.Minute),
		service.WithPasswordResetCodeStore(passwordResetCodeStore, 15*time.Minute),
	)
	handler := app.NewHTTPHandler(log, userService, jwtManager, mailer, cfg.AppBaseURL, db, redisClient, cfg.InternalToken)

	if err := server.Run(cfg.Addr, handler, log); err != nil {
		log.Error(context.Background(), "server stopped", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}
