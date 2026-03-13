package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"proyecto-cursos/internal/platform/httpclient"
	"proyecto-cursos/services/courses-api/internal/service"
)

type EnrollmentsAPIClient struct {
	client *httpclient.Client
}

func NewEnrollmentsAPIClient(baseURL, serviceName, internalToken string) *EnrollmentsAPIClient {
	return &EnrollmentsAPIClient{
		client: httpclient.New(
			strings.TrimRight(baseURL, "/"),
			serviceName,
			"enrollments-api",
			httpclient.WithInternalToken(internalToken),
		),
	}
}

func (c *EnrollmentsAPIClient) DeleteCourseEnrollments(ctx context.Context, courseID string) error {
	resp, err := c.client.DoJSON(ctx, http.MethodDelete, "/internal/courses/"+courseID+"/enrollments", nil, nil, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("enrollments-api returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *EnrollmentsAPIClient) GetAvailability(ctx context.Context, courseID string) (*service.RecommendationAvailability, error) {
	var payload struct {
		CourseID    string `json:"courseId"`
		Capacity    int    `json:"capacity"`
		ActiveCount int    `json:"activeCount"`
		Available   int    `json:"available"`
	}

	status, err := c.client.GetJSON(ctx, "/courses/"+courseID+"/availability", nil, &payload)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("enrollments-api returned status %d", status)
	}

	return &service.RecommendationAvailability{
		CourseID:    payload.CourseID,
		Capacity:    payload.Capacity,
		ActiveCount: payload.ActiveCount,
		Available:   payload.Available,
	}, nil
}
