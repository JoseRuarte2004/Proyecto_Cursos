package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"proyecto-cursos/internal/platform/httpclient"
)

type UserProfile struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type UsersAPIClient struct {
	client *httpclient.Client
}

func NewUsersAPIClient(baseURL, serviceName, internalToken string) *UsersAPIClient {
	return &UsersAPIClient{
		client: httpclient.New(
			strings.TrimRight(baseURL, "/"),
			serviceName,
			"users-api",
			httpclient.WithInternalToken(internalToken),
		),
	}
}

func (c *UsersAPIClient) GetUser(ctx context.Context, userID string) (*UserProfile, error) {
	var payload UserProfile

	status, err := c.client.GetJSON(ctx, "/internal/users/"+userID, nil, &payload)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return nil, ErrUserNotFound
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("users-api returned status %d", status)
	}

	return &payload, nil
}
