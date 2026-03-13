package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"proyecto-cursos/internal/platform/httpclient"
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

func (c *EnrollmentsAPIClient) IsStudentEnrolled(ctx context.Context, courseID, studentID string) (bool, error) {
	var payload struct {
		Enrolled bool `json:"enrolled"`
	}

	status, err := c.client.GetJSON(ctx, "/internal/courses/"+courseID+"/students/"+studentID+"/enrolled", nil, &payload)
	if err != nil {
		return false, err
	}

	if status == http.StatusNotFound {
		return false, nil
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("enrollments-api returned status %d", status)
	}

	return payload.Enrolled, nil
}

func (c *EnrollmentsAPIClient) ListActiveStudentIDs(ctx context.Context, courseID string) ([]string, error) {
	var payload struct {
		StudentIDs []string `json:"studentIds"`
	}

	status, err := c.client.GetJSON(ctx, "/internal/courses/"+courseID+"/students", nil, &payload)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return []string{}, nil
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("enrollments-api returned status %d", status)
	}

	return payload.StudentIDs, nil
}
