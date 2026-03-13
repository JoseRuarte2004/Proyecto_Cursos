package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/services/payments-api/internal/domain"
	"proyecto-cursos/services/payments-api/internal/service"
)

type MaintenanceWorker struct {
	log                   *logger.Logger
	service               *service.PaymentsService
	outboxPollInterval    time.Duration
	reconcilePollInterval time.Duration
	reconcileStaleAfter   time.Duration
	cleanupPollInterval   time.Duration
	batchSize             int
}

func NewMaintenanceWorker(
	log *logger.Logger,
	paymentsService *service.PaymentsService,
	outboxPollInterval, reconcilePollInterval, reconcileStaleAfter, cleanupPollInterval time.Duration,
	batchSize int,
) *MaintenanceWorker {
	if batchSize <= 0 {
		batchSize = 20
	}
	if outboxPollInterval <= 0 {
		outboxPollInterval = 2 * time.Second
	}
	if reconcilePollInterval <= 0 {
		reconcilePollInterval = 30 * time.Second
	}
	if reconcileStaleAfter <= 0 {
		reconcileStaleAfter = 15 * time.Second
	}
	if cleanupPollInterval <= 0 {
		cleanupPollInterval = 2 * time.Minute
	}

	return &MaintenanceWorker{
		log:                   log,
		service:               paymentsService,
		outboxPollInterval:    outboxPollInterval,
		reconcilePollInterval: reconcilePollInterval,
		reconcileStaleAfter:   reconcileStaleAfter,
		cleanupPollInterval:   cleanupPollInterval,
		batchSize:             batchSize,
	}
}

func (w *MaintenanceWorker) Start(ctx context.Context) {
	workerID := uuid.NewString()

	go w.loop(ctx, "payments-outbox", w.outboxPollInterval, func(loopCtx context.Context) error {
		_, err := w.service.DrainOutbox(loopCtx, workerID, w.batchSize)
		return err
	})

	go w.loop(ctx, "payments-webhook-jobs", w.outboxPollInterval, func(loopCtx context.Context) error {
		_, err := w.service.ProcessWebhookJobs(loopCtx, workerID, w.batchSize)
		return err
	})

	go w.loop(ctx, "payments-reconcile-mercadopago", w.reconcilePollInterval, func(loopCtx context.Context) error {
		olderThan := time.Now().UTC().Add(-w.reconcileStaleAfter)
		_, err := w.service.ReconcileOpenOrders(loopCtx, domain.ProviderMercadoPago, olderThan, w.batchSize)
		return err
	})

	go w.loop(ctx, "payments-cleanup", w.cleanupPollInterval, func(loopCtx context.Context) error {
		_, err := w.service.ExpireStaleOpenOrders(loopCtx, w.batchSize)
		return err
	})
}

func (w *MaintenanceWorker) loop(ctx context.Context, name string, every time.Duration, run func(context.Context) error) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		if err := run(ctx); err != nil && ctx.Err() == nil {
			w.log.Error(context.Background(), "payments maintenance task failed", map[string]any{
				"task":  name,
				"error": err.Error(),
			})
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
