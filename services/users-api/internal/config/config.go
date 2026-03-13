package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	platformconfig "proyecto-cursos/internal/platform/config"
)

type Config struct {
	ServiceName                      string
	Addr                             string
	DatabaseURL                      string
	RedisURL                         string
	JWTSecret                        string
	JWTTTL                           time.Duration
	InternalToken                    string
	AppEnv                           string
	AppBaseURL                       string
	EmailProvider                    string
	EmailFrom                        string
	SMTPHost                         string
	SMTPPort                         int
	SMTPUser                         string
	SMTPPass                         string
	SMTPFrom                         string
	ResendAPIKey                     string
	SendGridAPIKey                   string
	EmailVerifyTTL                   time.Duration
	PasswordResetTTL                 time.Duration
	RequireEmailVerificationForLogin bool
	BootstrapAdminName               string
	BootstrapAdminEmail              string
	BootstrapAdminPassword           string
}

func Parse() (Config, error) {
	parser := platformconfig.NewParser("users-api")
	cfg := Config{
		ServiceName:            "users-api",
		Addr:                   parser.String("USERS_API_ADDR", ":8081"),
		DatabaseURL:            parser.RequiredString("USERS_API_DATABASE_URL"),
		RedisURL:               parser.RequiredString("USERS_API_REDIS_URL"),
		JWTSecret:              parser.String("USERS_API_JWT_SECRET", parser.String("JWT_SECRET", "change-me")),
		JWTTTL:                 parser.Duration("USERS_API_JWT_TTL", 24*time.Hour),
		InternalToken:          parser.String("USERS_INTERNAL_TOKEN", "internal-token"),
		AppEnv:                 strings.ToLower(strings.TrimSpace(parser.String("APP_ENV", "dev"))),
		AppBaseURL:             parser.String("APP_BASE_URL", "http://localhost:8080"),
		EmailProvider:          parser.String("EMAIL_PROVIDER", "log"),
		EmailFrom:              parser.String("EMAIL_FROM", "no-reply@example.com"),
		SMTPHost:               parser.String("SMTP_HOST", ""),
		SMTPPort:               parser.Int("SMTP_PORT", 587),
		SMTPUser:               parser.String("SMTP_USER", ""),
		SMTPPass:               parser.String("SMTP_PASS", ""),
		SMTPFrom:               parser.String("SMTP_FROM", ""),
		ResendAPIKey:           parser.String("RESEND_API_KEY", ""),
		SendGridAPIKey:         parser.String("SENDGRID_API_KEY", ""),
		EmailVerifyTTL:         time.Duration(parser.Int("TOKEN_EMAIL_VERIFY_TTL_HOURS", 24)) * time.Hour,
		PasswordResetTTL:       time.Duration(parser.Int("TOKEN_PASSWORD_RESET_TTL_MINUTES", 60)) * time.Minute,
		BootstrapAdminName:     parser.String("USERS_API_BOOTSTRAP_ADMIN_NAME", "Platform Admin"),
		BootstrapAdminEmail:    parser.String("USERS_API_BOOTSTRAP_ADMIN_EMAIL", "admin@example.com"),
		BootstrapAdminPassword: parser.String("USERS_API_BOOTSTRAP_ADMIN_PASSWORD", "admin1234"),
	}

	var issues []string
	if cfg.AppEnv != "dev" && cfg.AppEnv != "prod" {
		issues = append(issues, "APP_ENV must be dev or prod")
	}

	requireVerify, err := parseBoolEnv("REQUIRE_EMAIL_VERIFICATION_FOR_LOGIN", false)
	if err != nil {
		issues = append(issues, err.Error())
	} else {
		cfg.RequireEmailVerificationForLogin = requireVerify
	}

	return cfg, combineConfigErr("users-api", parser.Err(), issues)
}

func parseBoolEnv(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback, fmt.Errorf("%s must be a boolean", key)
	}

	return parsed, nil
}

func combineConfigErr(service string, baseErr error, issues []string) error {
	if len(issues) == 0 {
		return baseErr
	}

	prefix := service + " config: " + strings.Join(issues, "; ")
	if baseErr == nil {
		return errors.New(prefix)
	}

	return fmt.Errorf("%w; %s", baseErr, prefix)
}
