package app

import (
	"testing"

	"github.com/stretchr/testify/require"

	"proyecto-cursos/services/payments-api/internal/domain"
)

func TestNormalizePublicBaseURL(t *testing.T) {
	t.Parallel()

	require.Equal(t, "http://localhost:8080", NormalizePublicBaseURL(""))
	require.Equal(t, "https://example.ngrok-free.app", NormalizePublicBaseURL("https://example.ngrok-free.app/"))
}

func TestBuildPublicWebhookURL(t *testing.T) {
	t.Parallel()

	require.Equal(
		t,
		"https://demo.trycloudflare.com/api/payments/webhooks/mercadopago",
		BuildPublicWebhookURL("https://demo.trycloudflare.com/", domain.ProviderMercadoPago),
	)
}

func TestBuildFrontendCheckoutResultURL(t *testing.T) {
	t.Parallel()

	require.Equal(
		t,
		"https://demo.trycloudflare.com/checkout/success?courseId=course-1&orderId=order-1",
		BuildFrontendCheckoutResultURL("https://demo.trycloudflare.com/", "/checkout/success", "order-1", "course-1"),
	)
}
