package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"proyecto-cursos/internal/platform/requestid"
	"proyecto-cursos/services/users-api/internal/domain"
)

func TestRequireRolesBlocksForbiddenRole(t *testing.T) {
	t.Parallel()

	handler := RequireRoles(domain.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req = req.WithContext(requestid.WithContext(context.Background(), "req-123"))
	req = req.WithContext(context.WithValue(req.Context(), authContextKey{}, AuthClaims{
		UserID: "student-1",
		Role:   domain.RoleStudent,
	}))

	rec := httptest.NewRecorder()
	rec.Header().Set(requestid.Header, "req-123")

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)

	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		RequestID string `json:"requestId"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, "FORBIDDEN", payload.Error.Code)
	require.Equal(t, "forbidden", payload.Error.Message)
	require.Equal(t, "req-123", payload.RequestID)
}

func TestRequireRolesAllowsExpectedRole(t *testing.T) {
	t.Parallel()

	called := false
	handler := RequireRoles(domain.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req = req.WithContext(context.WithValue(req.Context(), authContextKey{}, AuthClaims{
		UserID: "admin-1",
		Role:   domain.RoleAdmin,
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}
