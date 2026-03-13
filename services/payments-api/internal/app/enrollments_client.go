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

func (c *EnrollmentsAPIClient) HasPendingEnrollment(ctx context.Context, userID, courseID string) (bool, error) {
	var payload struct {
		Pending bool `json:"pending"`
	}
	status, err := c.client.GetJSON(ctx, "/internal/users/"+userID+"/courses/"+courseID+"/pending", nil, &payload)
	if err != nil {
		return false, err
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("enrollments-api returned status %d", status)
	}

	return payload.Pending, nil
}

func (c *EnrollmentsAPIClient) CancelPendingEnrollment(ctx context.Context, userID, courseID string) error {
	response, err := c.client.Do(ctx, http.MethodDelete, "/internal/users/"+userID+"/courses/"+courseID+"/pending", nil, nil)
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusNoContent || response.StatusCode == http.StatusOK || response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusConflict {
		return nil
	}

	return fmt.Errorf("enrollments-api returned status %d", response.StatusCode)
}
