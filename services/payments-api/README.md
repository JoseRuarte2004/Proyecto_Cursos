# payments-api

Variables principales:
- `PAYMENTS_API_ADDR`
- `PAYMENTS_DB_DSN`
- `JWT_SECRET`
- `RABBITMQ_URL`
- `COURSES_API_BASE_URL`
- `COURSES_INTERNAL_TOKEN`
- `ENROLLMENTS_API_BASE_URL`
- `ENROLL_INTERNAL_TOKEN`
- `PUBLIC_BASE_URL`
- `FRONTEND_BASE_URL`
- `MERCADOPAGO_ACCESS_TOKEN`
- `MERCADOPAGO_WEBHOOK_SECRET`
- `WEBHOOK_SECRET_STRIPE`

Endpoints:
- `GET /health`
- `GET /ready`
- `GET /metrics`
- `POST /orders`
- `GET /orders/:orderID`
- `POST /webhooks/:provider`

Auth:
- Usuario: `Authorization: Bearer <jwt>`
- Llamadas internas salientes: `X-Internal-Token` hacia `courses-api` y `enrollments-api`

Curl minimo:
```sh
curl -X POST http://localhost:8085/orders \
  -H "Authorization: Bearer $TOKEN_STUDENT" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: order-1" \
  -d "{\"courseId\":\"$COURSE_ID\",\"provider\":\"mercadopago\"}"
```
