package config

import platformconfig "proyecto-cursos/internal/platform/config"

type Config struct {
	ServiceName          string
	Addr                 string
	DatabaseURL          string
	CoursesBaseURL       string
	CoursesInternalToken string
	UsersBaseURL         string
	UsersInternalToken   string
	JWTSecret            string
	RabbitMQURL          string
	InternalToken        string
}

func Parse() (Config, error) {
	parser := platformconfig.NewParser("enrollments-api")
	cfg := Config{
		ServiceName:          parser.String("ENROLLMENTS_SERVICE_NAME", parser.String("SERVICE_NAME", "enrollments-api")),
		Addr:                 parser.String("ENROLLMENTS_API_ADDR", ":8084"),
		DatabaseURL:          parser.RequiredString("ENROLLMENTS_DB_DSN"),
		CoursesBaseURL:       parser.RequiredString("COURSES_API_BASE_URL"),
		CoursesInternalToken: parser.String("COURSES_INTERNAL_TOKEN", "internal-token"),
		UsersBaseURL:         parser.RequiredString("USERS_API_BASE_URL"),
		UsersInternalToken:   parser.String("USERS_INTERNAL_TOKEN", "internal-token"),
		JWTSecret:            parser.String("JWT_SECRET", "change-me"),
		RabbitMQURL:          parser.RequiredString("RABBITMQ_URL"),
		InternalToken:        parser.String("ENROLL_INTERNAL_TOKEN", parser.String("ENROLLMENTS_INTERNAL_TOKEN", "internal-token")),
	}

	return cfg, parser.Err()
}
