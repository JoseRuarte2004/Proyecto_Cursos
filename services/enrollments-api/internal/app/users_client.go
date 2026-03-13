package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"proyecto-cursos/internal/platform/httpclient"
	"proyecto-cursos/services/enrollments-api/internal/service"
)

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

func (c *UsersAPIClient) IsEmailVerified(ctx context.Context, userID string) (bool, error) {
	var payload struct {
		UserID        string `json:"userId"`
		EmailVerified bool   `json:"emailVerified"`
	}

	status, err := c.client.GetJSON(ctx, "/internal/users/"+userID+"/email-verified", nil, &payload)
	if err != nil {
		return false, err
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("users-api returned status %d", status)
	}

	return payload.EmailVerified, nil
}

func (c *UsersAPIClient) GetUser(ctx context.Context, userID string) (*service.UserInfo, error) {
	var payload struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}

	status, err := c.client.GetJSON(ctx, "/internal/users/"+userID, nil, &payload)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("users-api returned status %d", status)
	}

	return &service.UserInfo{
		ID:    payload.ID,
		Name:  payload.Name,
		Email: payload.Email,
		Role:  payload.Role,
	}, nil
}
