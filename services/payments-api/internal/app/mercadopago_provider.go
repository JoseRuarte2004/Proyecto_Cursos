package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"proyecto-cursos/services/payments-api/internal/domain"
	"proyecto-cursos/services/payments-api/internal/service"
)

const (
	mercadoPagoAPIBaseURL = "https://api.mercadopago.com"
)

type MercadoPagoProvider struct {
	accessToken                     string
	webhookSecret                   string
	notificationURL                 string
	frontendBaseURL                 string
	checkoutEnv                     string
	allowAccessTokenWebhookFallback bool
	httpClient                      *http.Client
}

type mercadoPagoPreferenceRequest struct {
	ExternalReference  string                        `json:"external_reference"`
	NotificationURL    string                        `json:"notification_url,omitempty"`
	AutoReturn         string                        `json:"auto_return,omitempty"`
	Expires            bool                          `json:"expires,omitempty"`
	ExpirationDateFrom string                        `json:"expiration_date_from,omitempty"`
	ExpirationDateTo   string                        `json:"expiration_date_to,omitempty"`
	BackURLs           mercadoPagoPreferenceBackURLs `json:"back_urls"`
	Items              []mercadoPagoPreferenceItem   `json:"items"`
}

type mercadoPagoPreferenceBackURLs struct {
	Success string `json:"success"`
	Pending string `json:"pending"`
	Failure string `json:"failure"`
}

type mercadoPagoPreferenceItem struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Quantity   int     `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	CurrencyID string  `json:"currency_id"`
}

type mercadoPagoPreferenceResponse struct {
	ID               string `json:"id"`
	InitPoint        string `json:"init_point"`
	SandboxInitPoint string `json:"sandbox_init_point"`
}

type mercadoPagoWebhookBody struct {
	Action string `json:"action"`
	Type   string `json:"type"`
	Topic  string `json:"topic"`
	Data   struct {
		ID any `json:"id"`
	} `json:"data"`
	ID any `json:"id"`
}

type mercadoPagoPaymentResponse struct {
	ID                any         `json:"id"`
	Status            string      `json:"status"`
	StatusDetail      string      `json:"status_detail"`
	ExternalReference string      `json:"external_reference"`
	CurrencyID        string      `json:"currency_id"`
	TransactionAmount json.Number `json:"transaction_amount"`
}

type mercadoPagoPaymentSearchResponse struct {
	Results []mercadoPagoPaymentResponse `json:"results"`
}

func NewMercadoPagoProvider(accessToken, webhookSecret, notificationURL, frontendBaseURL, checkoutEnv string, allowAccessTokenWebhookFallback bool) *MercadoPagoProvider {
	return &MercadoPagoProvider{
		accessToken:                     strings.TrimSpace(accessToken),
		webhookSecret:                   strings.TrimSpace(webhookSecret),
		notificationURL:                 strings.TrimSpace(notificationURL),
		frontendBaseURL:                 strings.TrimSpace(frontendBaseURL),
		checkoutEnv:                     strings.ToLower(strings.TrimSpace(checkoutEnv)),
		allowAccessTokenWebhookFallback: allowAccessTokenWebhookFallback,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (p *MercadoPagoProvider) CreateCheckout(ctx context.Context, input service.CreateCheckoutSessionInput) (*service.CheckoutSession, error) {
	if strings.TrimSpace(p.accessToken) == "" {
		return nil, service.ErrProviderMisconfigured
	}

	requestPayload := mercadoPagoPreferenceRequest{
		ExternalReference: strings.TrimSpace(input.OrderID),
		NotificationURL:   p.buildNotificationURL(),
		AutoReturn:        "approved",
		BackURLs: mercadoPagoPreferenceBackURLs{
			Success: BuildFrontendCheckoutResultURL(p.frontendBaseURL, "/checkout/success", input.OrderID, input.CourseID),
			Pending: BuildFrontendCheckoutResultURL(p.frontendBaseURL, "/checkout/success", input.OrderID, input.CourseID),
			Failure: BuildFrontendCheckoutResultURL(p.frontendBaseURL, "/checkout/failure", input.OrderID, input.CourseID),
		},
		Items: []mercadoPagoPreferenceItem{
			{
				ID:         strings.TrimSpace(input.CourseID),
				Title:      strings.TrimSpace(input.CourseTitle),
				Quantity:   1,
				UnitPrice:  service.CentsToAmount(input.AmountCents),
				CurrencyID: strings.TrimSpace(input.Currency),
			},
		},
	}
	if input.ExpiresAt != nil {
		requestPayload.Expires = true
		requestPayload.ExpirationDateFrom = time.Now().UTC().Format(time.RFC3339)
		requestPayload.ExpirationDateTo = input.ExpiresAt.UTC().Format(time.RFC3339)
	}

	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, mercadoPagoAPIBaseURL+"/checkout/preferences", bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+p.accessToken)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Idempotency-Key", strings.TrimSpace(input.OrderID))

	response, err := p.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", service.ErrProviderRequestFailed, err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: mercadopago preference creation returned %d: %s", service.ErrProviderRequestFailed, response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var payload mercadoPagoPreferenceResponse
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", service.ErrProviderRequestFailed, err)
	}

	checkoutURL := p.selectCheckoutURL(payload)
	if strings.TrimSpace(checkoutURL) == "" || strings.TrimSpace(payload.ID) == "" {
		return nil, fmt.Errorf("%w: mercadopago preference response is incomplete", service.ErrProviderRequestFailed)
	}
	if !isAllowedMercadoPagoCheckoutURL(checkoutURL) {
		return nil, fmt.Errorf("%w: unexpected checkout host", service.ErrProviderRequestFailed)
	}

	providerStatus := "created"
	return &service.CheckoutSession{
		CheckoutURL:         checkoutURL,
		ExternalReference:   strings.TrimSpace(input.OrderID),
		ProviderReferenceID: strings.TrimSpace(payload.ID),
		ProviderStatus:      &providerStatus,
	}, nil
}

func (p *MercadoPagoProvider) ParseWebhook(_ context.Context, request *http.Request, rawPayload []byte) (*service.QueueWebhookInput, error) {
	if strings.TrimSpace(p.accessToken) == "" || len(p.signatureCandidates()) == 0 {
		log.Printf(
			"[payments-api] mercadopago webhook misconfigured access_token_set=%t webhook_secret_set=%t fallback_to_access_token=%t",
			strings.TrimSpace(p.accessToken) != "",
			strings.TrimSpace(p.webhookSecret) != "",
			p.allowAccessTokenWebhookFallback,
		)
		return nil, service.ErrProviderMisconfigured
	}

	body := mercadoPagoWebhookBody{}
	if len(bytes.TrimSpace(rawPayload)) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(rawPayload))
		decoder.UseNumber()
		if err := decoder.Decode(&body); err != nil {
			log.Printf("[payments-api] mercadopago webhook payload decode failed err=%v payload=%q", err, truncateForLog(string(rawPayload), 400))
			return nil, fmt.Errorf("%w: %v", service.ErrWebhookPayloadInvalid, err)
		}
	}

	resourceID := strings.TrimSpace(firstNonEmpty(
		request.URL.Query().Get("data.id"),
		request.URL.Query().Get("id"),
		normalizeWebhookID(body.Data.ID),
		normalizeWebhookID(body.ID),
	))
	if resourceID == "" {
		log.Printf("[payments-api] mercadopago webhook missing resource id query=%s payload=%q", request.URL.RawQuery, truncateForLog(string(rawPayload), 400))
		return nil, service.ErrWebhookPayloadInvalid
	}

	requestID := strings.TrimSpace(request.Header.Get("X-Request-Id"))
	signature, err := p.validateWebhookSignature(request, resourceID, requestID)
	if err != nil {
		return nil, err
	}

	action := strings.TrimSpace(body.Action)
	topic := strings.TrimSpace(firstNonEmpty(body.Type, body.Topic, request.URL.Query().Get("type"), request.URL.Query().Get("topic")))
	receivedAt := time.Now().UTC()
	dedupeKey := buildWebhookJobDedupeKey(resourceID, requestID, signature.Timestamp)

	return &service.QueueWebhookInput{
		Provider:           domain.ProviderMercadoPago,
		DedupeKey:          dedupeKey,
		ResourceID:         resourceID,
		RequestID:          optionalString(requestID),
		SignatureTimestamp: optionalString(signature.Timestamp),
		Topic:              optionalString(topic),
		Action:             optionalString(action),
		ReceivedAt:         receivedAt,
		Payload:            strings.TrimSpace(string(rawPayload)),
	}, nil
}

func (p *MercadoPagoProvider) ResolveWebhook(ctx context.Context, request *http.Request, rawPayload []byte) (*service.ProcessWebhookInput, error) {
	queued, err := p.ParseWebhook(ctx, request, rawPayload)
	if err != nil {
		return nil, err
	}

	payment, err := p.fetchPayment(ctx, queued.ResourceID)
	if err != nil {
		log.Printf("[payments-api] mercadopago payment fetch failed payment_id=%s err=%v", queued.ResourceID, err)
		return nil, err
	}

	result, err := p.paymentToProcessWebhookInput(payment, derefString(queued.Action), derefString(queued.Topic), derefString(queued.RequestID), queued.ResourceID, queued.Payload)
	if err != nil {
		return nil, err
	}
	if result.ReceivedAt.IsZero() {
		result.ReceivedAt = queued.ReceivedAt
	}
	return result, nil
}

func (p *MercadoPagoProvider) ReconcilePayment(ctx context.Context, input service.ReconcilePaymentInput) (*service.ProcessWebhookInput, error) {
	if strings.TrimSpace(p.accessToken) == "" {
		return nil, service.ErrProviderMisconfigured
	}

	paymentID := strings.TrimSpace(input.PaymentID)
	if paymentID == "" {
		return nil, service.ErrWebhookPayloadInvalid
	}

	payment, err := p.fetchPayment(ctx, paymentID)
	if err != nil {
		log.Printf("[payments-api] mercadopago reconcile fetch failed payment_id=%s err=%v", paymentID, err)
		return nil, err
	}

	result, err := p.paymentToProcessWebhookInput(payment, "reconcile", "payment", "", paymentID, "")
	if err != nil {
		return nil, err
	}
	if input.Order != nil && strings.TrimSpace(result.OrderID) != strings.TrimSpace(input.Order.ID) {
		log.Printf(
			"[payments-api] mercadopago reconcile order mismatch expected_order_id=%s payment_id=%s external_reference=%s",
			input.Order.ID,
			paymentID,
			result.OrderID,
		)
		return nil, service.ErrWebhookPayloadInvalid
	}

	return result, nil
}

func (p *MercadoPagoProvider) ReconcileOpenOrder(ctx context.Context, order *domain.Order) (*service.ProcessWebhookInput, error) {
	if order == nil {
		return nil, nil
	}
	externalReference := ""
	if order.ExternalReference != nil {
		externalReference = strings.TrimSpace(*order.ExternalReference)
	}
	if externalReference == "" {
		return nil, nil
	}

	payment, err := p.searchPaymentByExternalReference(ctx, externalReference)
	if err != nil {
		return nil, err
	}
	if payment == nil {
		return nil, nil
	}

	return p.paymentToProcessWebhookInput(payment, "reconcile-open-order", "payment", "", normalizeWebhookID(payment.ID), "")
}

func (p *MercadoPagoProvider) fetchPayment(ctx context.Context, paymentID string) (*mercadoPagoPaymentResponse, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, mercadoPagoAPIBaseURL+"/v1/payments/"+url.PathEscape(strings.TrimSpace(paymentID)), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+p.accessToken)

	response, err := p.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", service.ErrProviderRequestFailed, err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: mercadopago payment lookup returned %d: %s", service.ErrProviderRequestFailed, response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var payload mercadoPagoPaymentResponse
	decoder := json.NewDecoder(bytes.NewReader(responseBody))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("%w: %v", service.ErrProviderRequestFailed, err)
	}

	return &payload, nil
}

func (p *MercadoPagoProvider) searchPaymentByExternalReference(ctx context.Context, externalReference string) (*mercadoPagoPaymentResponse, error) {
	if strings.TrimSpace(externalReference) == "" {
		return nil, nil
	}

	query := url.Values{}
	query.Set("external_reference", externalReference)
	query.Set("sort", "date_created")
	query.Set("criteria", "desc")
	query.Set("limit", "10")

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, mercadoPagoAPIBaseURL+"/v1/payments/search?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+p.accessToken)

	response, err := p.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", service.ErrProviderRequestFailed, err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: mercadopago payment search returned %d: %s", service.ErrProviderRequestFailed, response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var payload mercadoPagoPaymentSearchResponse
	decoder := json.NewDecoder(bytes.NewReader(responseBody))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("%w: %v", service.ErrProviderRequestFailed, err)
	}

	for idx := range payload.Results {
		result := payload.Results[idx]
		if strings.TrimSpace(result.ExternalReference) == strings.TrimSpace(externalReference) {
			return &result, nil
		}
	}

	return nil, nil
}

func (p *MercadoPagoProvider) paymentToProcessWebhookInput(payment *mercadoPagoPaymentResponse, action, topic, requestID, resourceID, rawPayload string) (*service.ProcessWebhookInput, error) {
	if payment == nil {
		return nil, service.ErrWebhookPayloadInvalid
	}
	externalReference := strings.TrimSpace(payment.ExternalReference)
	if externalReference == "" {
		log.Printf("[payments-api] mercadopago payment missing external_reference payment_id=%s status=%s", normalizeWebhookID(payment.ID), strings.TrimSpace(payment.Status))
		return nil, service.ErrWebhookPayloadInvalid
	}

	amountCents, err := service.NumericTextToCents(payment.TransactionAmount.String())
	if err != nil {
		log.Printf("[payments-api] mercadopago payment amount parse failed payment_id=%s amount=%q err=%v", normalizeWebhookID(payment.ID), payment.TransactionAmount.String(), err)
		return nil, service.ErrProviderRequestFailed
	}

	providerPaymentID := normalizeWebhookID(payment.ID)
	providerStatus := strings.TrimSpace(payment.Status)
	eventKey := buildPaymentEventKey(providerPaymentID, providerStatus, requestID)

	return &service.ProcessWebhookInput{
		EventKey:          eventKey,
		OrderID:           externalReference,
		Provider:          domain.ProviderMercadoPago,
		ProviderPaymentID: optionalString(providerPaymentID),
		Status:            providerStatus,
		ProviderStatus:    optionalString(providerStatus),
		AmountCents:       &amountCents,
		Currency:          optionalString(strings.TrimSpace(payment.CurrencyID)),
		RequestID:         optionalString(requestID),
		Topic:             optionalString(topic),
		Action:            optionalString(action),
		ResourceID:        optionalString(firstNonEmpty(resourceID, providerPaymentID)),
		ReceivedAt:        time.Now().UTC(),
		Payload:           strings.TrimSpace(rawPayload),
	}, nil
}

type signatureCandidate struct {
	label  string
	secret string
}

type webhookSignature struct {
	Timestamp     string
	Source        string
	SignatureHash string
}

func (p *MercadoPagoProvider) validateWebhookSignature(request *http.Request, resourceID, requestID string) (*webhookSignature, error) {
	signatureHeader := strings.TrimSpace(request.Header.Get("X-Signature"))
	if signatureHeader == "" {
		log.Printf("[payments-api] mercadopago signature validation failed reason=missing_signature_header query=%s", request.URL.RawQuery)
		return nil, service.ErrWebhookSignatureInvalid
	}

	signatureParts := parseSignatureHeader(signatureHeader)
	ts := strings.TrimSpace(signatureParts["ts"])
	v1 := strings.ToLower(strings.TrimSpace(signatureParts["v1"]))
	if ts == "" || v1 == "" {
		log.Printf("[payments-api] mercadopago signature validation failed reason=missing_signature_parts signature_header=%q", signatureHeader)
		return nil, service.ErrWebhookSignatureInvalid
	}

	manifests := buildSignatureManifests(resourceID, requestID, ts)
	if len(manifests) == 0 {
		log.Printf("[payments-api] mercadopago signature validation failed reason=empty_manifest resource_id=%q request_id=%q ts=%q", resourceID, requestID, ts)
		return nil, service.ErrWebhookSignatureInvalid
	}

	candidates := p.signatureCandidates()
	attemptedSignatures := make([]string, 0, len(manifests)*len(candidates))
	for _, candidate := range candidates {
		for _, manifest := range manifests {
			mac := hmac.New(sha256.New, []byte(candidate.secret))
			_, _ = mac.Write([]byte(manifest))
			expected := hex.EncodeToString(mac.Sum(nil))
			attemptedSignatures = append(attemptedSignatures, candidate.label+":"+manifest)
			if hmac.Equal([]byte(expected), []byte(v1)) {
				if candidate.label != "webhook_secret" {
					log.Printf(
						"[payments-api] mercadopago signature validation matched fallback source=%s resource_id=%s request_id=%s ts=%s",
						candidate.label,
						resourceID,
						requestID,
						ts,
					)
				}
				return &webhookSignature{
					Timestamp:     ts,
					Source:        candidate.label,
					SignatureHash: v1,
				}, nil
			}
		}
	}

	log.Printf(
		"[payments-api] mercadopago signature validation failed reason=signature_mismatch resource_id=%s request_id=%s ts=%s actual=%s attempted=%q candidate_count=%d",
		resourceID,
		requestID,
		ts,
		v1,
		attemptedSignatures,
		len(candidates),
	)
	return nil, service.ErrWebhookSignatureInvalid
}

func (p *MercadoPagoProvider) selectCheckoutURL(payload mercadoPagoPreferenceResponse) string {
	switch strings.ToLower(strings.TrimSpace(p.checkoutEnv)) {
	case "sandbox", "test", "testing":
		if strings.TrimSpace(payload.SandboxInitPoint) != "" {
			return strings.TrimSpace(payload.SandboxInitPoint)
		}
		return strings.TrimSpace(payload.InitPoint)
	default:
		if strings.TrimSpace(payload.InitPoint) != "" {
			return strings.TrimSpace(payload.InitPoint)
		}
		return strings.TrimSpace(payload.SandboxInitPoint)
	}
}

func (p *MercadoPagoProvider) buildNotificationURL() string {
	parsed, err := url.Parse(strings.TrimSpace(p.notificationURL))
	if err != nil {
		return strings.TrimSpace(p.notificationURL)
	}

	query := parsed.Query()
	query.Set("source_news", "webhooks")
	parsed.RawQuery = query.Encode()

	return parsed.String()
}

func buildPaymentEventKey(paymentID, status, requestID string) string {
	paymentID = strings.TrimSpace(paymentID)
	status = strings.ToLower(strings.TrimSpace(status))
	if paymentID != "" && status != "" {
		return paymentID + ":" + status
	}
	if paymentID != "" {
		return paymentID
	}
	if strings.TrimSpace(requestID) != "" {
		return strings.TrimSpace(requestID)
	}
	return "webhook:" + uuidLikeTimestamp()
}

func parseSignatureHeader(header string) map[string]string {
	parts := make(map[string]string)
	for _, section := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(section), "=")
		if !ok {
			continue
		}
		parts[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}

	return parts
}

func (p *MercadoPagoProvider) signatureCandidates() []signatureCandidate {
	seen := make(map[string]struct{})
	candidates := make([]signatureCandidate, 0, 3)

	addCandidate := func(label, secret string) {
		trimmed := strings.TrimSpace(secret)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		candidates = append(candidates, signatureCandidate{
			label:  label,
			secret: trimmed,
		})
	}

	for _, secret := range splitSecretCandidates(p.webhookSecret) {
		addCandidate("webhook_secret", secret)
	}
	if p.allowAccessTokenWebhookFallback {
		addCandidate("access_token_fallback", p.accessToken)
	}

	return candidates
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func normalizeWebhookID(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	case float64:
		return strings.TrimSpace(strconv.FormatFloat(typed, 'f', -1, 64))
	case float32:
		return strings.TrimSpace(strconv.FormatFloat(float64(typed), 'f', -1, 32))
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func buildSignatureManifests(resourceID, requestID, ts string) []string {
	requestID = strings.TrimSpace(requestID)
	ts = strings.TrimSpace(ts)

	resourceVariants := []string{strings.TrimSpace(resourceID)}
	if lower := strings.ToLower(strings.TrimSpace(resourceID)); lower != "" && lower != strings.TrimSpace(resourceID) {
		resourceVariants = append(resourceVariants, lower)
	}

	manifests := make([]string, 0, len(resourceVariants)*2)
	for _, variant := range uniqueNonEmptyStrings(resourceVariants) {
		fields := make([]string, 0, 3)
		if variant != "" {
			fields = append(fields, "id:"+variant)
		}
		if requestID != "" {
			fields = append(fields, "request-id:"+requestID)
		}
		if ts != "" {
			fields = append(fields, "ts:"+ts)
		}
		if len(fields) == 0 {
			continue
		}
		semicolonManifest := strings.Join(fields, ";")
		manifests = append(manifests, semicolonManifest+";")
		manifests = append(manifests, semicolonManifest)
		manifests = append(manifests, strings.Join(fields, " "))
	}

	return uniqueNonEmptyStrings(manifests)
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}

	return result
}

func splitSecretCandidates(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	replacer := strings.NewReplacer("\r", "\n", ";", ",")
	chunks := strings.Split(replacer.Replace(value), ",")
	secrets := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		for _, line := range strings.Split(chunk, "\n") {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				secrets = append(secrets, trimmed)
			}
		}
	}

	return uniqueNonEmptyStrings(secrets)
}

func buildWebhookJobDedupeKey(resourceID, requestID, timestamp string) string {
	resourceID = strings.TrimSpace(resourceID)
	requestID = strings.TrimSpace(requestID)
	timestamp = strings.TrimSpace(timestamp)
	switch {
	case resourceID != "" && requestID != "":
		return resourceID + ":" + requestID
	case resourceID != "" && timestamp != "":
		return resourceID + ":" + timestamp
	default:
		return resourceID
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}

	return strings.TrimSpace(*value)
}

func isAllowedMercadoPagoCheckoutURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return false
	}

	allowedSuffixes := []string{
		"mercadopago.com",
		"mercadopago.com.ar",
		"mercadopago.com.br",
		"mercadopago.cl",
		"mercadopago.com.mx",
		"mpago.la",
	}
	for _, suffix := range allowedSuffixes {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}

	return false
}

func (p *MercadoPagoProvider) IsCheckoutURLAllowed(rawURL string) bool {
	if !isAllowedMercadoPagoCheckoutURL(rawURL) {
		return false
	}

	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}

	host := strings.ToLower(parsed.Hostname())
	switch strings.ToLower(strings.TrimSpace(p.checkoutEnv)) {
	case "sandbox", "test", "testing":
		return strings.Contains(host, "sandbox.mercadopago.")
	default:
		return !strings.Contains(host, "sandbox.mercadopago.")
	}
}

func uuidLikeTimestamp() string {
	return strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
}
