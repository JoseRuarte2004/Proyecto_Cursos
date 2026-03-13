package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"proyecto-cursos/internal/platform/httpx"
	"proyecto-cursos/services/users-api/internal/domain"
)

type AuthClaims struct {
	UserID string      `json:"userId"`
	Role   domain.Role `json:"role"`
	jwt.RegisteredClaims
}

type authContextKey struct{}

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

func (m *JWTManager) Issue(userID string, role domain.Role) (string, error) {
	claims := AuthClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) Parse(tokenString string) (*AuthClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AuthClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}

		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*AuthClaims)
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
				httpx.WriteError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}

			tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			claims, err := jwtManager.Parse(tokenString)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			ctx := context.WithValue(r.Context(), authContextKey{}, *claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRoles(roles ...domain.Role) func(http.Handler) http.Handler {
	allowedRoles := make(map[domain.Role]struct{}, len(roles))
	for _, role := range roles {
		allowedRoles[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, "missing auth context")
				return
			}

			if _, ok := allowedRoles[claims.Role]; !ok {
				httpx.WriteError(w, http.StatusForbidden, "forbidden")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func ClaimsFromContext(ctx context.Context) (AuthClaims, bool) {
	claims, ok := ctx.Value(authContextKey{}).(AuthClaims)
	return claims, ok
}
