package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"proyecto-cursos/internal/platform/mq"
	"proyecto-cursos/internal/platform/requestid"
	"proyecto-cursos/services/payments-api/internal/service"
)

type RabbitPublisher struct {
	manager *mq.ConnectionManager
}

func NewRabbitPublisher(manager *mq.ConnectionManager) *RabbitPublisher {
	return &RabbitPublisher{manager: manager}
}

func (p *RabbitPublisher) Ready(ctx context.Context) error {
	return p.manager.Ping(ctx)
}

func (p *RabbitPublisher) PublishPaymentPaid(ctx context.Context, event service.PaymentPaidEvent) error {
	channel, err := p.manager.Channel(ctx)
	if err != nil {
		return err
	}
	defer channel.Close()

	if err := channel.ExchangeDeclare("payments", "topic", true, false, false, false, nil); err != nil {
		return err
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	headers := amqp.Table{}
	if requestID := requestid.FromContext(ctx); requestID != "" {
		headers["requestId"] = requestID
	}

	return channel.PublishWithContext(ctx, "payments", "payment.paid", false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    uuid.NewString(),
		Timestamp:    time.Now().UTC(),
		Headers:      headers,
		Body:         payload,
	})
}
