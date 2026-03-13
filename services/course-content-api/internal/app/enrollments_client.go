package app

import (
	"context"
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
		return false, nil
	}

	if status == http.StatusNotFound || status == http.StatusNotImplemented || status == http.StatusForbidden {
		return false, nil
	}
	if status != http.StatusOK {
		return false, nil
	}

	return payload.Enrolled, nil
}
