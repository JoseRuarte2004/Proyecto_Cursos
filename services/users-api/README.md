# users-api

Variables principales:
- `USERS_API_ADDR`
- `USERS_API_DATABASE_URL`
- `USERS_API_REDIS_URL`
- `USERS_API_JWT_SECRET`
- `USERS_API_JWT_TTL`
- `APP_ENV`
- `APP_BASE_URL`
- `EMAIL_PROVIDER`
- `EMAIL_FROM`
- `SMTP_HOST`
- `SMTP_PORT`
- `SMTP_USER`
- `SMTP_PASS`
- `SMTP_FROM`
- `RESEND_API_KEY`
- `SENDGRID_API_KEY`
- `USERS_INTERNAL_TOKEN`
- `TOKEN_EMAIL_VERIFY_TTL_HOURS`
- `TOKEN_PASSWORD_RESET_TTL_MINUTES`
- `REQUIRE_EMAIL_VERIFICATION_FOR_LOGIN`
- `USERS_API_BOOTSTRAP_ADMIN_EMAIL`
- `USERS_API_BOOTSTRAP_ADMIN_PASSWORD`

Endpoints:
- `GET /health`
- `GET /ready`
- `GET /metrics`
- `POST /register` (email + password, genera codigo de verificacion en Redis)
- `POST /verify` (email + code)
- `POST /auth/register`
- `POST /auth/login`
- `GET /auth/verify-email`
- `POST /auth/verify-email/request`
- `POST /auth/password/forgot`
- `POST /auth/password/reset`
- `POST /auth/password/forgot/code`
- `POST /auth/password/reset/code`
- `GET /me`
- `GET /admin/users`
- `GET /admin/users/:id`
- `PATCH /admin/users/:id/role`
- `GET /internal/users/:id` (`X-Internal-Token`)
- `GET /internal/users/:id/email-verified` (`X-Internal-Token`)

Auth:
- Usuario: `Authorization: Bearer <jwt>`
- Interna: `X-Internal-Token: <USERS_INTERNAL_TOKEN>`

Notas:
- En desarrollo se recomienda `EMAIL_PROVIDER=smtp` con `SMTP_HOST=mailhog` y `SMTP_PORT=1025`.
- Bandeja local MailHog: `http://localhost:8025`
- Si `REQUIRE_EMAIL_VERIFICATION_FOR_LOGIN=true`, el login devuelve `403 email not verified` para usuarios no verificados.
- Flujo de 2 pasos:
  - `POST /register` crea el usuario con `isVerified=false`.
  - Guarda el codigo en Redis con key `verify_code:{email}` y TTL de 15 minutos.
  - `POST /verify` valida codigo y marca el usuario como verificado.
- Recuperacion de contrasena por codigo:
  - `POST /auth/password/forgot/code` envia codigo de 6 digitos por email.
  - Guarda codigo en Redis con key `password_reset_code:{email}` y TTL de 15 minutos.
  - `POST /auth/password/reset/code` valida codigo y actualiza password.

Curl minimo:
```sh
curl -X POST http://localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"admin1234"}'
```

Ver docs adicionales:
- `docs/auth.md`
