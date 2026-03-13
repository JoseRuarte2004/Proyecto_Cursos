package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

type randomTokenGenerator struct{}

func (randomTokenGenerator) Generate() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}

	token := base64.RawURLEncoding.EncodeToString(buf)
	tokenHash, err := hashOpaqueToken(token)
	if err != nil {
		return "", "", err
	}

	return token, tokenHash, nil
}

func hashOpaqueToken(token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", ErrTokenRequired
	}

	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:]), nil
}
