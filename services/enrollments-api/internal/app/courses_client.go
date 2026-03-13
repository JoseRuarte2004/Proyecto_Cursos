package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"proyecto-cursos/internal/platform/httpclient"
	"proyecto-cursos/services/enrollments-api/internal/service"
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
		ID       string  `json:"id"`
		Title    string  `json:"title"`
		Category string  `json:"category"`
		ImageURL *string `json:"imageUrl"`
		Price    float64 `json:"price"`
		Currency string  `json:"currency"`
		Status   string  `json:"status"`
		Capacity int     `json:"capacity"`
	}

	status, err := c.client.GetJSON(ctx, "/internal/courses/"+courseID, nil, &payload)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return nil, service.ErrCourseNotFound
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("courses-api returned status %d", status)
	}

	return &service.CourseInfo{
		ID:       payload.ID,
		Title:    payload.Title,
		Category: payload.Category,
		ImageURL: payload.ImageURL,
		Price:    payload.Price,
		Currency: payload.Currency,
		Status:   payload.Status,
		Capacity: payload.Capacity,
	}, nil
}

func (c *CoursesAPIClient) IsTeacherAssigned(ctx context.Context, courseID, teacherID string) (bool, error) {
	var payload struct {
		Assigned bool `json:"assigned"`
	}

	status, err := c.client.GetJSON(ctx, "/internal/courses/"+courseID+"/teachers/"+teacherID+"/assigned", nil, &payload)
	if err != nil {
		return false, err
	}

	if status == http.StatusNotFound || status == http.StatusForbidden {
		return false, nil
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("courses-api returned status %d", status)
	}

	return payload.Assigned, nil
}
