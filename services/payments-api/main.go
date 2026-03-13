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
	"proyecto-cursos/services/payments-api/internal/app"
	serviceconfig "proyecto-cursos/services/payments-api/internal/config"
	"proyecto-cursos/services/payments-api/internal/domain"
	"proyecto-cursos/services/payments-api/internal/repository"
	"proyecto-cursos/services/payments-api/internal/service"
)

func main() {
	cfg, err := serviceconfig.Parse()
	if err != nil {
		logger.New("payments-api").Error(context.Background(), "config parse failed", map[string]any{"error": err.Error()})
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

	mercadoPagoProvider := app.NewMercadoPagoProvider(
		cfg.MercadoPagoAccessToken,
		cfg.MercadoPagoWebhookSecret,
		app.BuildPublicWebhookURL(cfg.PublicBaseURL, domain.ProviderMercadoPago),
		cfg.FrontendBaseURL,
		cfg.MercadoPagoCheckoutEnv,
		cfg.AppEnv != "prod" && cfg.AppEnv != "production",
	)

	paymentsService := service.NewPaymentsService(
		repository.NewPostgresOrderRepository(db),
		app.NewEnrollmentsAPIClient(cfg.EnrollmentsBaseURL, cfg.ServiceName, cfg.EnrollInternalToken),
		app.NewCoursesAPIClient(cfg.CoursesBaseURL, cfg.ServiceName, cfg.CoursesInternalToken),
		app.NewRabbitPublisher(rabbitManager),
		map[domain.Provider]service.CheckoutProvider{
			domain.ProviderMercadoPago: mercadoPagoProvider,
		},
		cfg.OrderCreatedTTL,
		cfg.OrderPendingTTL,
	)
	log.Info(context.Background(), "public webhook urls configured", map[string]any{
		"mercadoPagoWebhookURL": app.BuildPublicWebhookURL(cfg.PublicBaseURL, domain.ProviderMercadoPago),
		"stripeWebhookURL":      app.BuildPublicWebhookURL(cfg.PublicBaseURL, domain.ProviderStripe),
	})
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, 24*time.Hour)
	router := app.NewHTTPHandler(log, paymentsService, jwtManager, db, rabbitManager, map[domain.Provider]app.WebhookParser{
		domain.ProviderMercadoPago: mercadoPagoProvider,
	})
	runtimeCtx, runtimeCancel := context.WithCancel(context.Background())
	defer runtimeCancel()
	app.NewMaintenanceWorker(
		log,
		paymentsService,
		cfg.OutboxPollInterval,
		cfg.ReconcilePollInterval,
		cfg.ReconcileStaleAfter,
		cfg.CleanupPollInterval,
		cfg.WorkerBatchSize,
	).Start(runtimeCtx)

	if err := server.Run(cfg.Addr, router, log); err != nil {
		log.Error(context.Background(), "server stopped", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}
