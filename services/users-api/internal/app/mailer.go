package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"proyecto-cursos/internal/platform/logger"
	serviceconfig "proyecto-cursos/services/users-api/internal/config"
)

type MailMessage struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
	Link     string
}

type Mailer interface {
	Send(ctx context.Context, message MailMessage) error
}

func NewMailer(cfg serviceconfig.Config, log *logger.Logger) (Mailer, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.EmailProvider))
	if provider == "" {
		provider = "log"
	}

	switch provider {
	case "log":
		if cfg.AppEnv == "prod" {
			return nil, errors.New("EMAIL_PROVIDER=log is not allowed when APP_ENV=prod")
		}
		return logMailer{log: log, includeLink: true}, nil
	case "smtp":
		if strings.TrimSpace(cfg.SMTPHost) == "" {
			return nil, errors.New("SMTP_HOST is required when EMAIL_PROVIDER=smtp")
		}

		headerFrom := firstNonEmpty(cfg.EmailFrom, cfg.SMTPFrom)
		if strings.TrimSpace(headerFrom) == "" {
			return nil, errors.New("EMAIL_FROM or SMTP_FROM is required when EMAIL_PROVIDER=smtp")
		}
		envelopeFrom := firstNonEmpty(cfg.SMTPFrom, cfg.EmailFrom)

		return smtpMailer{
			addr:         fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort),
			host:         cfg.SMTPHost,
			port:         cfg.SMTPPort,
			username:     cfg.SMTPUser,
			password:     cfg.SMTPPass,
			headerFrom:   headerFrom,
			envelopeFrom: envelopeFrom,
		}, nil
	case "resend":
		if strings.TrimSpace(cfg.ResendAPIKey) == "" {
			return nil, errors.New("RESEND_API_KEY is required when EMAIL_PROVIDER=resend")
		}
		if strings.TrimSpace(cfg.EmailFrom) == "" {
			return nil, errors.New("EMAIL_FROM is required when EMAIL_PROVIDER=resend")
		}

		return apiMailer{
			endpoint: "https://api.resend.com/emails",
			headers: map[string]string{
				"Authorization": "Bearer " + cfg.ResendAPIKey,
			},
			timeout: 5 * time.Second,
			buildBody: func(message MailMessage) any {
				return map[string]any{
					"from":    cfg.EmailFrom,
					"to":      []string{message.To},
					"subject": message.Subject,
					"text":    message.TextBody,
					"html":    message.HTMLBody,
				}
			},
			success: func(status int) bool { return status >= 200 && status < 300 },
		}, nil
	case "sendgrid":
		if strings.TrimSpace(cfg.SendGridAPIKey) == "" {
			return nil, errors.New("SENDGRID_API_KEY is required when EMAIL_PROVIDER=sendgrid")
		}
		if strings.TrimSpace(cfg.EmailFrom) == "" {
			return nil, errors.New("EMAIL_FROM is required when EMAIL_PROVIDER=sendgrid")
		}

		return apiMailer{
			endpoint: "https://api.sendgrid.com/v3/mail/send",
			headers: map[string]string{
				"Authorization": "Bearer " + cfg.SendGridAPIKey,
			},
			timeout: 5 * time.Second,
			buildBody: func(message MailMessage) any {
				return map[string]any{
					"personalizations": []map[string]any{
						{
							"to": []map[string]string{
								{"email": message.To},
							},
						},
					},
					"from": map[string]string{
						"email": cfg.EmailFrom,
					},
					"subject": message.Subject,
					"content": []map[string]string{
						{
							"type":  "text/plain",
							"value": message.TextBody,
						},
						{
							"type":  "text/html",
							"value": message.HTMLBody,
						},
					},
				}
			},
			success: func(status int) bool { return status >= 200 && status < 300 },
		}, nil
	default:
		return nil, fmt.Errorf("unsupported EMAIL_PROVIDER %q", provider)
	}
}

type logMailer struct {
	log         *logger.Logger
	includeLink bool
}

func (m logMailer) Send(ctx context.Context, message MailMessage) error {
	fields := map[string]any{
		"provider": "log",
		"to":       message.To,
		"subject":  message.Subject,
	}
	if m.includeLink {
		fields["link"] = message.Link
	}

	m.log.Info(ctx, "email queued", fields)
	return nil
}

type smtpMailer struct {
	addr         string
	host         string
	port         int
	username     string
	password     string
	headerFrom   string
	envelopeFrom string
}

func (m smtpMailer) Send(_ context.Context, message MailMessage) error {
	auth := smtp.Auth(nil)
	if strings.TrimSpace(m.username) != "" || strings.TrimSpace(m.password) != "" {
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}

	fromHeader, _, err := normalizeAddress(m.headerFrom)
	if err != nil {
		return err
	}
	_, fromEnvelope, err := normalizeAddress(firstNonEmpty(m.envelopeFrom, m.headerFrom))
	if err != nil {
		return err
	}

	toHeader, toEnvelope, err := normalizeAddress(message.To)
	if err != nil {
		return err
	}

	var payload strings.Builder
	payload.WriteString("From: " + fromHeader + "\r\n")
	payload.WriteString("To: " + toHeader + "\r\n")
	payload.WriteString("Subject: " + strings.TrimSpace(message.Subject) + "\r\n")
	payload.WriteString("MIME-Version: 1.0\r\n")
	payload.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	payload.WriteString("\r\n")
	payload.WriteString(message.TextBody)

	client, err := smtp.Dial(m.addr)
	if err != nil {
		return err
	}
	defer client.Close()

	startTLSSupported, _ := client.Extension("STARTTLS")
	if startTLSSupported {
		if err := client.StartTLS(&tls.Config{
			ServerName: m.host,
			MinVersion: tls.VersionTLS12,
		}); err != nil {
			return err
		}
	} else if auth != nil {
		return errors.New("smtp server does not support STARTTLS; cannot authenticate securely")
	}

	if auth != nil {
		authSupported, _ := client.Extension("AUTH")
		if !authSupported {
			return errors.New("smtp server does not support AUTH")
		}
		if err := client.Auth(auth); err != nil {
			return err
		}
	}

	if err := client.Mail(fromEnvelope); err != nil {
		return err
	}
	if err := client.Rcpt(toEnvelope); err != nil {
		return err
	}

	dataWriter, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := dataWriter.Write([]byte(payload.String())); err != nil {
		_ = dataWriter.Close()
		return err
	}
	if err := dataWriter.Close(); err != nil {
		return err
	}

	return client.Quit()
}

type apiMailer struct {
	endpoint  string
	headers   map[string]string
	timeout   time.Duration
	buildBody func(message MailMessage) any
	success   func(status int) bool
}

func (m apiMailer) Send(ctx context.Context, message MailMessage) error {
	payload, err := json.Marshal(m.buildBody(message))
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range m.headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: m.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if m.success(resp.StatusCode) {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("email provider returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func normalizeAddress(value string) (string, string, error) {
	parsed, err := mail.ParseAddress(strings.TrimSpace(value))
	if err != nil {
		return "", "", err
	}

	return parsed.String(), parsed.Address, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}
