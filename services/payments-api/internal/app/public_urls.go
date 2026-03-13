package app

import (
	"net/url"
	"strings"

	"proyecto-cursos/services/payments-api/internal/domain"
)

const defaultPublicBaseURL = "http://localhost:8080"

func NormalizePublicBaseURL(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		trimmed = defaultPublicBaseURL
	}

	return strings.TrimRight(trimmed, "/")
}

func BuildPublicWebhookURL(baseURL string, provider domain.Provider) string {
	return NormalizePublicBaseURL(baseURL) + "/api/payments/webhooks/" + string(provider)
}

func BuildFrontendCheckoutResultURL(baseURL, resultPath, orderID, courseID string) string {
	parsed, err := url.Parse(NormalizePublicBaseURL(baseURL))
	if err != nil {
		return NormalizePublicBaseURL(baseURL) + resultPath
	}

	basePath := strings.TrimRight(parsed.Path, "/")
	parsed.Path = basePath + resultPath

	query := parsed.Query()
	if trimmed := strings.TrimSpace(orderID); trimmed != "" {
		query.Set("orderId", trimmed)
	}
	if trimmed := strings.TrimSpace(courseID); trimmed != "" {
		query.Set("courseId", trimmed)
	}
	parsed.RawQuery = query.Encode()

	return parsed.String()
}
