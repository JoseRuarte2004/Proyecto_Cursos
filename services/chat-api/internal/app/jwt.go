package app

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenValidator interface {
	Validate(token string) (Principal, error)
}

type Principal struct {
	UserID string
	Role   string
}

type JWTValidator struct {
	secret []byte
}

func NewJWTValidator(secret string) *JWTValidator {
	return &JWTValidator{
		secret: []byte(secret),
	}
}

func (v *JWTValidator) Validate(tokenString string) (Principal, error) {
	tokenString = strings.TrimSpace(tokenString)
	if tokenString == "" {
		return Principal{}, errors.New("token is required")
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing method")
		}
		return v.secret, nil
	})
	if err != nil || !token.Valid {
		return Principal{}, errors.New("invalid token")
	}

	role := normalizeRole(claims["role"])

	// Basic validator: toma `userId` o `sub`.
	// Si usas otro proveedor de identidad, enchufa tu validador fuerte aqui.
	if userID, ok := claims["userId"].(string); ok && strings.TrimSpace(userID) != "" {
		return Principal{UserID: strings.TrimSpace(userID), Role: role}, nil
	}
	if sub, ok := claims["sub"].(string); ok && strings.TrimSpace(sub) != "" {
		return Principal{UserID: strings.TrimSpace(sub), Role: role}, nil
	}

	// jwt.MapClaims.Valid() ya verifica exp/nbf/iat si estan presentes, pero reforzamos `exp` si vino numerico.
	if expRaw, ok := claims["exp"].(float64); ok {
		if time.Unix(int64(expRaw), 0).Before(time.Now()) {
			return Principal{}, errors.New("token expired")
		}
	}

	return Principal{}, errors.New("token missing user identifier")
}

func normalizeRole(raw any) string {
	role, ok := raw.(string)
	if !ok {
		return "student"
	}

	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin":
		return "admin"
	case "teacher":
		return "teacher"
	default:
		return "student"
	}
}
