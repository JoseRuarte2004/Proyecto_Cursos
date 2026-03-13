package config

import platformconfig "proyecto-cursos/internal/platform/config"

type Config struct {
	ServiceName              string
	Addr                     string
	DatabaseURL              string
	JWTSecret                string
	CoursesAPIBaseURL        string
	CoursesInternalToken     string
	EnrollmentsAPIBaseURL    string
	EnrollmentsInternalToken string
	UsersAPIBaseURL          string
	UsersInternalToken       string
}

func Parse() (Config, error) {
	parser := platformconfig.NewParser("chat-api")
	cfg := Config{
		ServiceName:              parser.String("CHAT_SERVICE_NAME", parser.String("SERVICE_NAME", "chat-api")),
		Addr:                     parser.String("CHAT_API_ADDR", ":8082"),
		DatabaseURL:              parser.RequiredString("CHAT_DB_DSN"),
		JWTSecret:                parser.String("JWT_SECRET", "change-me"),
		CoursesAPIBaseURL:        parser.String("COURSES_API_BASE_URL", "http://courses-api:8082"),
		CoursesInternalToken:     parser.String("COURSES_INTERNAL_TOKEN", "internal-courses"),
		EnrollmentsAPIBaseURL:    parser.String("ENROLLMENTS_API_BASE_URL", "http://enrollments-api:8084"),
		EnrollmentsInternalToken: parser.String("ENROLL_INTERNAL_TOKEN", parser.String("ENROLLMENTS_INTERNAL_TOKEN", "internal-enrollments")),
		UsersAPIBaseURL:          parser.String("USERS_API_BASE_URL", "http://users-api:8081"),
		UsersInternalToken:       parser.String("USERS_INTERNAL_TOKEN", "internal-users"),
	}

	return cfg, parser.Err()
}
