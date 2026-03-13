package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Parser struct {
	service string
	issues  []string
}

func NewParser(service string) *Parser {
	return &Parser{service: strings.TrimSpace(service)}
}

func (p *Parser) String(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func (p *Parser) RequiredString(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		p.issues = append(p.issues, fmt.Sprintf("%s is required", key))
	}
	return value
}

func (p *Parser) Int(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		p.issues = append(p.issues, fmt.Sprintf("%s must be an integer", key))
		return fallback
	}

	return parsed
}

func (p *Parser) Duration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		p.issues = append(p.issues, fmt.Sprintf("%s must be a duration", key))
		return fallback
	}

	return parsed
}

func (p *Parser) Err() error {
	if len(p.issues) == 0 {
		return nil
	}

	prefix := "config"
	if p.service != "" {
		prefix = p.service + " config"
	}

	return errors.New(prefix + ": " + strings.Join(p.issues, "; "))
}
