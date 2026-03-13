package config

import platformconfig "proyecto-cursos/internal/platform/config"
import "time"

type Config struct {
	ServiceName              string
	Addr                     string
	AppEnv                   string
	PublicBaseURL            string
	FrontendBaseURL          string
	DatabaseURL              string
	JWTSecret                string
	RabbitMQURL              string
	EnrollmentsBaseURL       string
	EnrollInternalToken      string
	CoursesBaseURL           string
	CoursesInternalToken     string
	MercadoPagoAccessToken   string
	MercadoPagoWebhookSecret string
	MercadoPagoCheckoutEnv   string
	OrderCreatedTTL          time.Duration
	OrderPendingTTL          time.Duration
	OutboxPollInterval       time.Duration
	ReconcilePollInterval    time.Duration
	ReconcileStaleAfter      time.Duration
	CleanupPollInterval      time.Duration
	WorkerBatchSize          int
	WebhookSecretStripe      string
}

func Parse() (Config, error) {
	parser := platformconfig.NewParser("payments-api")
	appEnv := parser.String("APP_ENV", "dev")
	mercadoPagoCheckoutEnv := parser.String("MERCADOPAGO_CHECKOUT_ENV", "production")
	cfg := Config{
		ServiceName:              parser.String("PAYMENTS_SERVICE_NAME", parser.String("SERVICE_NAME", "payments-api")),
		Addr:                     parser.String("PAYMENTS_API_ADDR", ":8085"),
		AppEnv:                   appEnv,
		PublicBaseURL:            parser.String("PUBLIC_BASE_URL", parser.String("APP_BASE_URL", "http://localhost:8080")),
		FrontendBaseURL:          parser.String("FRONTEND_BASE_URL", parser.String("PUBLIC_BASE_URL", parser.String("APP_BASE_URL", "http://localhost:8080"))),
		DatabaseURL:              parser.RequiredString("PAYMENTS_DB_DSN"),
		JWTSecret:                parser.String("JWT_SECRET", "change-me"),
		RabbitMQURL:              parser.RequiredString("RABBITMQ_URL"),
		EnrollmentsBaseURL:       parser.RequiredString("ENROLLMENTS_API_BASE_URL"),
		EnrollInternalToken:      parser.String("ENROLL_INTERNAL_TOKEN", parser.String("ENROLLMENTS_INTERNAL_TOKEN", "internal-token")),
		CoursesBaseURL:           parser.RequiredString("COURSES_API_BASE_URL"),
		CoursesInternalToken:     parser.String("COURSES_INTERNAL_TOKEN", "internal-token"),
		MercadoPagoAccessToken:   parser.String("MERCADOPAGO_ACCESS_TOKEN", ""),
		MercadoPagoWebhookSecret: parser.String("MERCADOPAGO_WEBHOOK_SECRET", parser.String("WEBHOOK_SECRET_MERCADOPAGO", "")),
		MercadoPagoCheckoutEnv:   mercadoPagoCheckoutEnv,
		OrderCreatedTTL:          parser.Duration("PAYMENTS_ORDER_CREATED_TTL", 30*time.Minute),
		OrderPendingTTL:          parser.Duration("PAYMENTS_ORDER_PENDING_TTL", 72*time.Hour),
		OutboxPollInterval:       parser.Duration("PAYMENTS_OUTBOX_POLL_INTERVAL", 2*time.Second),
		ReconcilePollInterval:    parser.Duration("PAYMENTS_RECONCILE_POLL_INTERVAL", 30*time.Second),
		ReconcileStaleAfter:      parser.Duration("PAYMENTS_RECONCILE_STALE_AFTER", 15*time.Second),
		CleanupPollInterval:      parser.Duration("PAYMENTS_CLEANUP_POLL_INTERVAL", 2*time.Minute),
		WorkerBatchSize:          parser.Int("PAYMENTS_WORKER_BATCH_SIZE", 20),
		WebhookSecretStripe:      parser.String("WEBHOOK_SECRET_STRIPE", ""),
	}

	return cfg, parser.Err()
}
