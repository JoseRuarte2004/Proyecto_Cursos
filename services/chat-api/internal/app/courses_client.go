package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"proyecto-cursos/internal/platform/httpclient"
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

func (c *CoursesAPIClient) IsTeacherAssigned(ctx context.Context, courseID, teacherID string) (bool, error) {
	var payload struct {
		Assigned bool `json:"assigned"`
	}

	status, err := c.client.GetJSON(ctx, "/internal/courses/"+courseID+"/teachers/"+teacherID+"/assigned", nil, &payload)
	if err != nil {
		return false, err
	}

	if status == http.StatusNotFound {
		return false, nil
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("courses-api returned status %d", status)
	}

	return payload.Assigned, nil
}

func (c *CoursesAPIClient) ListTeacherIDs(ctx context.Context, courseID string) ([]string, error) {
	var payload struct {
		TeacherIDs []string `json:"teacherIds"`
	}

	status, err := c.client.GetJSON(ctx, "/internal/courses/"+courseID+"/teachers", nil, &payload)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return []string{}, nil
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("courses-api returned status %d", status)
	}

	return payload.TeacherIDs, nil
}
