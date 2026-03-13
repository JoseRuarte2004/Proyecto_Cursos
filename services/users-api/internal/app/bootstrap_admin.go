package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"proyecto-cursos/internal/platform/logger"
	serviceconfig "proyecto-cursos/services/users-api/internal/config"
	"proyecto-cursos/services/users-api/internal/domain"
	"proyecto-cursos/services/users-api/internal/service"
)

func EnsureBootstrapAdmin(ctx context.Context, cfg serviceconfig.Config, users service.UserRepository, passwordManager service.PasswordManager, log *logger.Logger) error {
	email := strings.ToLower(strings.TrimSpace(cfg.BootstrapAdminEmail))
	password := strings.TrimSpace(cfg.BootstrapAdminPassword)
	name := strings.TrimSpace(cfg.BootstrapAdminName)

	if email == "" || password == "" || name == "" {
		return nil
	}

	existingUser, err := users.GetByEmail(ctx, email)
	if err == nil {
		if existingUser.Role == domain.RoleAdmin {
			log.Info(context.Background(), "bootstrap admin already present", map[string]any{
				"email": email,
			})
			return nil
		}

		if _, err := users.UpdateRole(ctx, existingUser.ID, domain.RoleAdmin); err != nil {
			return err
		}

		log.Info(context.Background(), "bootstrap admin promoted", map[string]any{
			"email": email,
		})
		return nil
	}

	if !errors.Is(err, service.ErrUserNotFound) {
		return err
	}

	hash, err := passwordManager.Hash(password)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	_, err = users.Create(ctx, domain.User{
		ID:              uuid.NewString(),
		Name:            name,
		Email:           email,
		PasswordHash:    hash,
		IsVerified:      true,
		EmailVerified:   true,
		EmailVerifiedAt: &now,
		Role:            domain.RoleAdmin,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err != nil {
		return err
	}

	log.Info(context.Background(), "bootstrap admin created", map[string]any{
		"email": email,
	})
	return nil
}
