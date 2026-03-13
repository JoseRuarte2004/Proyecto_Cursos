package config

import platformconfig "proyecto-cursos/internal/platform/config"

type Config struct {
	ServiceName           string
	Addr                  string
	DatabaseURL           string
	CoursesAPIBaseURL     string
	EnrollmentsAPIBaseURL string
	CoursesInternalToken  string
	EnrollInternalToken   string
	JWTSecret             string
}

func Parse() (Config, error) {
	parser := platformconfig.NewParser("course-content-api")
	cfg := Config{
		ServiceName:           "course-content-api",
		Addr:                  parser.String("COURSE_CONTENT_API_ADDR", ":8083"),
		DatabaseURL:           parser.RequiredString("CONTENT_DB_DSN"),
		CoursesAPIBaseURL:     parser.RequiredString("COURSES_API_BASE_URL"),
		EnrollmentsAPIBaseURL: parser.RequiredString("ENROLLMENTS_API_BASE_URL"),
		CoursesInternalToken:  parser.String("COURSES_INTERNAL_TOKEN", "internal-token"),
		EnrollInternalToken:   parser.String("ENROLL_INTERNAL_TOKEN", parser.String("ENROLLMENTS_INTERNAL_TOKEN", "internal-token")),
		JWTSecret:             parser.String("JWT_SECRET", "change-me"),
	}

	return cfg, parser.Err()
}
