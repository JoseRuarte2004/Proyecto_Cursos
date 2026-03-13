package app

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"proyecto-cursos/internal/platform/auth"
	"proyecto-cursos/internal/platform/httpx"
	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/internal/platform/mq"
	"proyecto-cursos/internal/platform/server"
	"proyecto-cursos/services/payments-api/internal/domain"
	"proyecto-cursos/services/payments-api/internal/service"
)

type WebhookParser interface {
	ParseWebhook(ctx context.Context, request *http.Request, rawPayload []byte) (*service.QueueWebhookInput, error)
}

type Handler struct {
	service        *service.PaymentsService
	logger         *logger.Logger
	webhookParsers map[domain.Provider]WebhookParser
}

func NewHTTPHandler(log *logger.Logger, paymentsService *service.PaymentsService, jwtManager *auth.JWTManager, db *sql.DB, rabbit *mq.ConnectionManager, webhookParsers map[domain.Provider]WebhookParser) http.Handler {
	readyFn := func(ctx context.Context) error {
		if err := db.PingContext(ctx); err != nil {
			return err
		}

		return rabbit.Ping(ctx)
	}

	router := server.NewRouter("payments-api", log, readyFn)
	handler := &Handler{
		service:        paymentsService,
		logger:         log,
		webhookParsers: webhookParsers,
	}

	router.Post("/webhooks/{provider}", handler.handleWebhook)

	router.Group(func(r chi.Router) {
		r.Use(auth.AuthRequired(jwtManager))
		r.Use(auth.RequireRoles(auth.RoleStudent))
		r.Post("/orders", handler.handleCreateOrder)
		r.Get("/orders/{orderID}", handler.handleGetOrder)
	})

	return router
}

type createOrderRequest struct {
	CourseID string          `json:"courseId"`
	Provider domain.Provider `json:"provider"`
}

func (h *Handler) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing auth context")
		return
	}

	var req createOrderRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.CreateOrder(r.Context(), session.Claims.UserID, service.CreateOrderInput{
		CourseID: req.CourseID,
		Provider: req.Provider,
	}, r.Header.Get("Idempotency-Key"))
	if err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	w.Header().Set("Idempotency-Key", result.IdempotencyKey)
	status := http.StatusCreated
	if !result.Created {
		status = http.StatusOK
	}

	httpx.WriteJSON(w, status, orderResponse(result.Order, result.CheckoutURL))
}

func (h *Handler) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing auth context")
		return
	}

	orderID := strings.TrimSpace(chi.URLParam(r, "orderID"))
	order, err := h.service.GetOrder(r.Context(), session.Claims.UserID, orderID)
	if err != nil {
		h.writeRequestError(w, r, err)
		return
	}

	paymentID := strings.TrimSpace(firstNonEmptyValue(
		r.URL.Query().Get("paymentId"),
		r.URL.Query().Get("payment_id"),
		r.URL.Query().Get("collection_id"),
	))
	if paymentID != "" {
		refreshed, refreshErr := h.service.RefreshOrder(r.Context(), session.Claims.UserID, orderID, service.RefreshOrderInput{
			PaymentID: paymentID,
		})
		if refreshErr != nil {
			log.Printf("[payments-api] order refresh failed order_id=%s payment_id=%s request_id=%s err=%v", orderID, paymentID, strings.TrimSpace(r.Header.Get("X-Request-Id")), refreshErr)
			h.writeRequestError(w, r, refreshErr)
			return
		} else {
			order = refreshed
		}
	}

	httpx.WriteJSON(w, http.StatusOK, orderResponse(order, dereference(order.CheckoutURL)))
}

func (h *Handler) handleWebhook(w http.ResponseWriter, r *http.Request) {
	provider := domain.Provider(strings.ToLower(strings.TrimSpace(chi.URLParam(r, "provider"))))
	if !provider.IsValid() {
		httpx.WriteError(w, http.StatusBadRequest, service.ErrInvalidProvider.Error())
		return
	}

	parser, ok := h.webhookParsers[provider]
	if !ok || parser == nil {
		h.writeRequestError(w, r, service.ErrProviderUnsupported)
		return
	}

	rawPayload, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "request body could not be read")
		return
	}
	_ = r.Body.Close()

	log.Printf(
		"[payments-api] webhook received provider=%s method=%s path=%s query=%s request_id=%s content_length=%d payload=%q",
		provider,
		r.Method,
		r.URL.Path,
		r.URL.RawQuery,
		strings.TrimSpace(r.Header.Get("X-Request-Id")),
		len(rawPayload),
		truncateForLog(string(rawPayload), 400),
	)

	input, err := parser.ParseWebhook(r.Context(), r, rawPayload)
	if err != nil {
		log.Printf(
			"[payments-api] webhook parse failed provider=%s request_id=%s err=%v",
			provider,
			strings.TrimSpace(r.Header.Get("X-Request-Id")),
			err,
		)
		h.writeRequestError(w, r, err)
		return
	}

	job, queued, err := h.service.QueueWebhook(r.Context(), *input)
	if err != nil {
		log.Printf(
			"[payments-api] webhook enqueue failed provider=%s resource_id=%s request_id=%s err=%v",
			provider,
			input.ResourceID,
			strings.TrimSpace(r.Header.Get("X-Request-Id")),
			err,
		)
		h.writeRequestError(w, r, err)
		return
	}

	log.Printf(
		"[payments-api] webhook accepted provider=%s resource_id=%s dedupe_key=%s queued=%t job_id=%s",
		provider,
		input.ResourceID,
		input.DedupeKey,
		queued,
		job.ID,
	)

	if queued {
		workerID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if workerID == "" {
			workerID = strings.TrimSpace(job.ID)
		}

		bgCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		jobProcessed, processErr := h.service.ProcessWebhookJobByID(bgCtx, workerID, job.ID)
		cancel()
		if processErr != nil {
			log.Printf(
				"[payments-api] webhook immediate processing failed job_id=%s worker_id=%s err=%v",
				job.ID,
				workerID,
				processErr,
			)
		} else {
			log.Printf(
				"[payments-api] webhook immediate processing completed job_id=%s worker_id=%s processed=%t",
				job.ID,
				workerID,
				jobProcessed,
			)
		}

		if !jobProcessed {
			go func(queuedJob domain.PaymentWebhookJob, fallbackWorkerID string) {
				time.Sleep(2 * time.Second)

				retryCtx, retryCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer retryCancel()

				retryProcessed, retryErr := h.service.ProcessWebhookJobDirect(retryCtx, queuedJob)
				if retryErr != nil {
					log.Printf(
						"[payments-api] webhook delayed processing failed job_id=%s worker_id=%s err=%v",
						queuedJob.ID,
						fallbackWorkerID,
						retryErr,
					)
					return
				}

				log.Printf(
					"[payments-api] webhook delayed processing completed job_id=%s worker_id=%s processed=%t",
					queuedJob.ID,
					fallbackWorkerID,
					retryProcessed,
				)
			}(*job, workerID)
		}
	}

	status := http.StatusAccepted
	if !queued && job.ProcessedAt != nil {
		status = http.StatusOK
	}
	httpx.WriteJSON(w, status, map[string]any{
		"accepted":   true,
		"queued":     queued,
		"resourceId": input.ResourceID,
		"jobId":      job.ID,
	})
}

func (h *Handler) writeRequestError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrCourseIDRequired),
		errors.Is(err, service.ErrInvalidProvider),
		errors.Is(err, service.ErrWebhookStatusInvalid),
		errors.Is(err, service.ErrProviderMismatch),
		errors.Is(err, service.ErrProviderUnsupported),
		errors.Is(err, service.ErrWebhookPayloadInvalid),
		errors.Is(err, service.ErrWebhookEventKeyRequired):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrPendingEnrollmentRequired):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrPaymentAmountMismatch),
		errors.Is(err, service.ErrPaymentCurrencyMismatch),
		errors.Is(err, service.ErrPaymentConflict):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrWebhookSignatureInvalid):
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, service.ErrOrderNotFound):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrProviderMisconfigured):
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
	case errors.Is(err, service.ErrProviderRequestFailed):
		httpx.WriteError(w, http.StatusBadGateway, err.Error())
	default:
		h.logger.Error(r.Context(), "request failed", map[string]any{
			"error": err.Error(),
		})
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}

func orderResponse(order *domain.Order, checkoutURL string) map[string]any {
	return map[string]any{
		"orderId":              order.ID,
		"status":               order.Status,
		"provider":             order.Provider,
		"checkoutUrl":          checkoutURL,
		"idempotencyKey":       order.IdempotencyKey,
		"providerStatus":       dereference(order.ProviderStatus),
		"providerPaymentId":    dereference(order.ProviderPaymentID),
		"providerPreferenceId": dereference(order.ProviderPreferenceID),
		"externalReference":    dereference(order.ExternalReference),
		"paidAt":               formatTime(order.PaidAt),
		"failedAt":             formatTime(order.FailedAt),
		"lastWebhookAt":        formatTime(order.LastWebhookAt),
	}
}

func dereference(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func formatTime(value *time.Time) string {
	if value == nil {
		return ""
	}

	return value.UTC().Format(time.RFC3339Nano)
}

func truncateForLog(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= limit {
		return trimmed
	}

	return trimmed[:limit] + "...(truncated)"
}

func firstNonEmptyValue(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}
