package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMercadoPagoProviderResolveWebhookAcceptsNumericIDs(t *testing.T) {
	t.Parallel()

	provider := NewMercadoPagoProvider(
		"APP_USR-test",
		"webhook-secret",
		"https://example.ngrok-free.dev/api/payments/webhooks/mercadopago",
		"https://example.ngrok-free.dev",
		"sandbox",
		true,
	)
	provider.httpClient = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			require.Equal(t, "https://api.mercadopago.com/v1/payments/149318594659", request.URL.String())

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"id": 149318594659,
					"status": "approved",
					"external_reference": "order-123",
					"currency_id": "ARS",
					"transaction_amount": 25000
				}`)),
			}, nil
		}),
	}

	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"https://example.ngrok-free.dev/api/payments/webhooks/mercadopago?type=payment",
		strings.NewReader(`{"action":"payment.updated","data":{"id":149318594659}}`),
	)
	require.NoError(t, err)

	requestID := "request-123"
	resourceID := "149318594659"
	timestamp := "1710261825000"
	signature := signMercadoPagoWebhook(t, "webhook-secret", resourceID, requestID, timestamp)
	request.Header.Set("X-Request-Id", requestID)
	request.Header.Set("X-Signature", "ts="+timestamp+",v1="+signature)

	input, err := provider.ResolveWebhook(context.Background(), request, []byte(`{"action":"payment.updated","data":{"id":149318594659}}`))
	require.NoError(t, err)
	require.Equal(t, "order-123", input.OrderID)
	require.NotNil(t, input.ProviderPaymentID)
	require.Equal(t, "149318594659", *input.ProviderPaymentID)
	require.NotNil(t, input.ProviderStatus)
	require.Equal(t, "approved", *input.ProviderStatus)
	require.NotNil(t, input.AmountCents)
	require.EqualValues(t, 2500000, *input.AmountCents)
	require.NotNil(t, input.Currency)
	require.Equal(t, "ARS", *input.Currency)
	require.NotNil(t, input.Topic)
	require.Equal(t, "payment", *input.Topic)
}

func TestMercadoPagoProviderParseWebhookAcceptsAccessTokenFallback(t *testing.T) {
	t.Parallel()

	provider := NewMercadoPagoProvider(
		"APP_USR-test-fallback",
		"",
		"https://example.ngrok-free.dev/api/payments/webhooks/mercadopago",
		"https://example.ngrok-free.dev",
		"production",
		true,
	)

	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"https://example.ngrok-free.dev/api/payments/webhooks/mercadopago?data.id=149318594659&type=payment",
		strings.NewReader(`{"action":"payment.updated"}`),
	)
	require.NoError(t, err)

	requestID := "request-fallback"
	timestamp := "1710261825000"
	signature := signMercadoPagoWebhook(t, "APP_USR-test-fallback", "149318594659", requestID, timestamp)
	request.Header.Set("X-Request-Id", requestID)
	request.Header.Set("X-Signature", "ts="+timestamp+",v1="+signature)

	queued, err := provider.ParseWebhook(context.Background(), request, []byte(`{"action":"payment.updated"}`))
	require.NoError(t, err)
	require.Equal(t, "149318594659", queued.ResourceID)
	require.Equal(t, "149318594659:request-fallback", queued.DedupeKey)
	require.NotNil(t, queued.RequestID)
	require.Equal(t, requestID, *queued.RequestID)
}

func TestMercadoPagoProviderParseWebhookAcceptsLegacySpaceManifest(t *testing.T) {
	t.Parallel()

	provider := NewMercadoPagoProvider(
		"APP_USR-test",
		"webhook-secret",
		"https://example.ngrok-free.dev/api/payments/webhooks/mercadopago",
		"https://example.ngrok-free.dev",
		"production",
		false,
	)

	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"https://example.ngrok-free.dev/api/payments/webhooks/mercadopago?data.id=149318594659&type=payment",
		strings.NewReader(`{"action":"payment.updated"}`),
	)
	require.NoError(t, err)

	requestID := "request-legacy-space"
	timestamp := "1710261825000"
	signature := signMercadoPagoWebhookSpaceManifest(t, "webhook-secret", "149318594659", requestID, timestamp)
	request.Header.Set("X-Request-Id", requestID)
	request.Header.Set("X-Signature", "ts="+timestamp+",v1="+signature)

	queued, err := provider.ParseWebhook(context.Background(), request, []byte(`{"action":"payment.updated"}`))
	require.NoError(t, err)
	require.Equal(t, "149318594659", queued.ResourceID)
	require.Equal(t, "149318594659:request-legacy-space", queued.DedupeKey)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func signMercadoPagoWebhook(t *testing.T, secret, resourceID, requestID, timestamp string) string {
	t.Helper()

	manifest := "id:" + resourceID + ";request-id:" + requestID + ";ts:" + timestamp + ";"
	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write([]byte(manifest))
	require.NoError(t, err)

	return hex.EncodeToString(mac.Sum(nil))
}

func signMercadoPagoWebhookSpaceManifest(t *testing.T, secret, resourceID, requestID, timestamp string) string {
	t.Helper()

	manifest := "id:" + resourceID + " request-id:" + requestID + " ts:" + timestamp
	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write([]byte(manifest))
	require.NoError(t, err)

	return hex.EncodeToString(mac.Sum(nil))
}
