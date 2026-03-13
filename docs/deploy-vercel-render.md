# Deploy Vercel + Render

## Frontend (Vercel)

- Root directory: `frontend`
- Install command: `npm install`
- Build command: `npm run build`
- Output directory: `dist`

Variable obligatoria:

```env
VITE_API_BASE_URL=https://api.example.com/api
```

## Backend (Render)

Arquitectura recomendada:

- `backend-gateway`: servicio publico (`nginx`)
- `users-api`: privado
- `courses-api`: privado
- `course-content-api`: privado
- `enrollments-api`: privado
- `payments-api`: privado
- `chat-api`: privado
- `platform-redis`: privado
- `platform-rabbitmq`: privado
- `platform-postgres`: base de datos administrada por Render

El frontend no se sirve desde `nginx` en cloud. `nginx` queda solo como gateway/API publico.

### URLs finales esperadas

```env
FRONTEND_BASE_URL=https://app.example.com
PUBLIC_BASE_URL=https://api.example.com
MERCADOPAGO_WEBHOOK_URL=https://api.example.com/api/payments/webhooks/mercadopago
```

### Variables compartidas del backend

Estas variables deben tener el mismo valor en todos los servicios backend que las usen:

```env
APP_ENV=production
JWT_SECRET=...
USERS_API_JWT_SECRET=...
USERS_INTERNAL_TOKEN=...
COURSES_INTERNAL_TOKEN=...
CONTENT_INTERNAL_TOKEN=...
ENROLL_INTERNAL_TOKEN=...
ENROLLMENTS_INTERNAL_TOKEN=...
PAYMENTS_INTERNAL_TOKEN=...
CHAT_INTERNAL_TOKEN=...
PUBLIC_BASE_URL=https://api.example.com
FRONTEND_BASE_URL=https://app.example.com
FRONT_ORIGIN=https://app.example.com
APP_BASE_URL=https://api.example.com
RABBITMQ_URL=amqp://<user>:<password>@platform-rabbitmq:5672/
```

### Notas operativas

- `APP_BASE_URL` en `users-api` debe apuntar al backend publico porque genera links como `/auth/verify-email`.
- `FRONTEND_BASE_URL` en `payments-api` debe apuntar al frontend publico porque Mercado Pago vuelve a `/checkout/success` y `/checkout/failure`.
- El webhook de Mercado Pago debe configurarse con:

```text
https://api.example.com/api/payments/webhooks/mercadopago
```
