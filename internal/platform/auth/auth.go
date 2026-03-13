package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"proyecto-cursos/internal/platform/api"
)

type Role string

const (
	RoleStudent Role = "student"
	RoleTeacher Role = "teacher"
	RoleAdmin   Role = "admin"
)

type Claims struct {
	UserID string `json:"userId"`
	Role   Role   `json:"role"`
	jwt.RegisteredClaims
}

type Session struct {
	Token  string
	Claims Claims
}

type contextKey struct{}

type JWTManager struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTManager(secret string, ttl time.Duration) *JWTManager {
	return &JWTManager{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (m *JWTManager) Issue(userID string, role Role) (string, error) {
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) Parse(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}

		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

func AuthRequired(jwtManager *JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				api.WriteError(w, http.StatusUnauthorized, "MISSING_BEARER_TOKEN", "missing bearer token")
				return
			}

			tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			claims, err := jwtManager.Parse(tokenString)
			if err != nil {
				api.WriteError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid token")
				return
			}

			ctx := context.WithValue(r.Context(), contextKey{}, Session{
				Token:  tokenString,
				Claims: *claims,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRoles(roles ...Role) func(http.Handler) http.Handler {
	allowedRoles := make(map[Role]struct{}, len(roles))
	for _, role := range roles {
		allowedRoles[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, ok := FromContext(r.Context())
			if !ok {
				api.WriteError(w, http.StatusUnauthorized, "MISSING_AUTH_CONTEXT", "missing auth context")
				return
			}

			if _, ok := allowedRoles[session.Claims.Role]; !ok {
				api.WriteError(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func FromContext(ctx context.Context) (Session, bool) {
	session, ok := ctx.Value(contextKey{}).(Session)
	return session, ok
}
