package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"proyecto-cursos/internal/platform/auth"
	"proyecto-cursos/internal/platform/httpclient"
	"proyecto-cursos/services/courses-api/internal/service"
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

func (c *UsersAPIClient) GetTeacher(ctx context.Context, teacherID string) (*service.TeacherInfo, error) {
	var payload struct {
		ID    string    `json:"id"`
		Name  string    `json:"name"`
		Email string    `json:"email"`
		Role  auth.Role `json:"role"`
	}

	status, err := c.client.GetJSON(ctx, "/internal/users/"+teacherID, nil, &payload)
	if err != nil {
		return nil, err
	}

	switch status {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, service.ErrTeacherNotFound
	default:
		return nil, fmt.Errorf("users-api returned status %d", status)
	}
	if payload.ID == "" {
		return nil, errors.New("users-api returned empty id")
	}

	return &service.TeacherInfo{
		ID:    payload.ID,
		Name:  payload.Name,
		Email: payload.Email,
		Role:  payload.Role,
	}, nil
}
