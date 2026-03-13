package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"proyecto-cursos/internal/platform/httpclient"
	"proyecto-cursos/services/payments-api/internal/service"
)

type CoursesAPIClient struct {
	client *httpclient.Client
}

func NewCoursesAPIClient(baseURL, serviceName, internalToken string) *CoursesAPIClient {
	return &CoursesAPIClient{
		client: httpclient.New(
			strings.TrimRight(baseURL, "/"),
			serviceName,
			"courses-api",
			httpclient.WithInternalToken(internalToken),
		),
	}
}

func (c *CoursesAPIClient) GetCourse(ctx context.Context, courseID string) (*service.CourseInfo, error) {
	var payload struct {
		ID       string      `json:"id"`
		Title    string      `json:"title"`
		Category string      `json:"category"`
		ImageURL *string     `json:"imageUrl"`
		Price    json.Number `json:"price"`
		Currency string      `json:"currency"`
		Status   string      `json:"status"`
	}
	status, err := c.client.GetJSON(ctx, "/internal/courses/"+courseID, nil, &payload)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("courses-api returned status %d", status)
	}

	priceCents, err := service.NumericTextToCents(payload.Price.String())
	if err != nil {
		return nil, err
	}

	return &service.CourseInfo{
		ID:         payload.ID,
		Title:      payload.Title,
		Category:   payload.Category,
		ImageURL:   payload.ImageURL,
		PriceCents: priceCents,
		Currency:   payload.Currency,
		Status:     payload.Status,
	}, nil
}
