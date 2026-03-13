package config

import (
	"time"

	platformconfig "proyecto-cursos/internal/platform/config"
)

type Config struct {
	ServiceName              string
	Addr                     string
	DatabaseURL              string
	RedisURL                 string
	UsersAPIBaseURL          string
	UsersInternalToken       string
	EnrollmentsAPIBaseURL    string
	EnrollmentsInternalToken string
	OpenAIAPIKey             string
	OpenAIModel              string
	JWTSecret                string
	InternalToken            string
	CacheTTL                 time.Duration
}

func Parse() (Config, error) {
	parser := platformconfig.NewParser("courses-api")
	cfg := Config{
		ServiceName:              "courses-api",
		Addr:                     parser.String("COURSES_API_ADDR", ":8082"),
		DatabaseURL:              parser.RequiredString("COURSES_DB_DSN"),
		RedisURL:                 parser.RequiredString("COURSES_REDIS_ADDR"),
		UsersAPIBaseURL:          parser.RequiredString("USERS_API_BASE_URL"),
		UsersInternalToken:       parser.String("USERS_INTERNAL_TOKEN", "internal-token"),
		EnrollmentsAPIBaseURL:    parser.RequiredString("ENROLLMENTS_API_BASE_URL"),
		EnrollmentsInternalToken: parser.String("ENROLL_INTERNAL_TOKEN", parser.String("ENROLLMENTS_INTERNAL_TOKEN", "internal-enrollments")),
		OpenAIAPIKey:             parser.String("OPENAI_API_KEY", ""),
		OpenAIModel:              parser.String("OPENAI_MODEL", "gpt-4.1"),
		JWTSecret:                parser.String("JWT_SECRET", "change-me"),
		InternalToken:            parser.String("COURSES_INTERNAL_TOKEN", "internal-token"),
		CacheTTL:                 parser.Duration("COURSES_CACHE_TTL", 5*time.Minute),
	}

	return cfg, parser.Err()
}
