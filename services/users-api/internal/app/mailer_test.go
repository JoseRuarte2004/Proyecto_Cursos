package app

import (
	"testing"

	"github.com/stretchr/testify/require"

	"proyecto-cursos/internal/platform/logger"
	serviceconfig "proyecto-cursos/services/users-api/internal/config"
)

func TestNewMailerRejectsLogProviderInProd(t *testing.T) {
	t.Parallel()

	_, err := NewMailer(serviceconfig.Config{
		AppEnv:        "prod",
		EmailProvider: "log",
	}, logger.New("users-api"))

	require.Error(t, err)
	require.ErrorContains(t, err, "EMAIL_PROVIDER=log is not allowed when APP_ENV=prod")
}

func TestNewMailerAllowsLogProviderInDev(t *testing.T) {
	t.Parallel()

	mailer, err := NewMailer(serviceconfig.Config{
		AppEnv:        "dev",
		EmailProvider: "log",
	}, logger.New("users-api"))

	require.NoError(t, err)
	require.NotNil(t, mailer)
}

func TestNewMailerSMTPUsesDisplayFromAndEnvelopeFrom(t *testing.T) {
	t.Parallel()

	mailer, err := NewMailer(serviceconfig.Config{
		AppEnv:        "dev",
		EmailProvider: "smtp",
		SMTPHost:      "smtp.gmail.com",
		SMTPPort:      587,
		SMTPUser:      "viralnow433@gmail.com",
		SMTPPass:      "secret",
		SMTPFrom:      "viralnow433@gmail.com",
		EmailFrom:     "Plataforma de Cursos <viralnow433@gmail.com>",
	}, logger.New("users-api"))

	require.NoError(t, err)

	smtpMailer, ok := mailer.(smtpMailer)
	require.True(t, ok)
	require.Equal(t, "smtp.gmail.com", smtpMailer.host)
	require.Equal(t, 587, smtpMailer.port)
	require.Equal(t, "Plataforma de Cursos <viralnow433@gmail.com>", smtpMailer.headerFrom)
	require.Equal(t, "viralnow433@gmail.com", smtpMailer.envelopeFrom)
}
