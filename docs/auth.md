# Auth flows

Base recomendada con gateway:
- `APP_BASE_URL=http://localhost:8080`
- API users via gateway: `http://localhost:8080/api/users`

Notas:
- `POST /auth/register` mantiene el response actual, pero ahora crea el usuario con `email_verified=false` y dispara un mail de verificacion.
- En `APP_ENV=dev`, `EMAIL_PROVIDER=log` funciona como `mail sink` y el link se imprime en logs JSON.
- En `APP_ENV=prod`, `EMAIL_PROVIDER=log` no esta permitido y el servicio falla al arrancar.
- El link de verificacion apunta a `{APP_BASE_URL}/auth/verify-email?token=...`.
- El link de reset apunta a `{APP_BASE_URL}/reset-password?token=...`; normalmente esa URL la consume el frontend y luego hace `POST /api/users/auth/password/reset`.

## Variables de entorno

- `APP_ENV=dev|prod`
- `APP_BASE_URL`
- `EMAIL_PROVIDER=log|smtp|resend|sendgrid`
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

## Verificacion de email

Login:
- Si `REQUIRE_EMAIL_VERIFICATION_FOR_LOGIN=false` el login mantiene el comportamiento actual.
- Si `REQUIRE_EMAIL_VERIFICATION_FOR_LOGIN=true` y el usuario no verifico el email, `POST /api/users/auth/login` responde `403 email not verified`.

Flujo:
1. `POST /api/users/auth/register`
2. Backend crea `users.email_verified=false`
3. Backend guarda hash SHA-256 del token en `email_verification_tokens`
4. Backend envia mail con `{APP_BASE_URL}/auth/verify-email?token=<token>`
5. Usuario abre el link
6. `GET /auth/verify-email?token=...` marca `users.email_verified=true`, completa `email_verified_at` y marca `used_at`
7. `POST /api/enrollments/enrollments/reserve` queda habilitado

Registro:
```sh
curl -X POST http://localhost:8080/api/users/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Ana","email":"ana@example.com","password":"secret123","phone":"111","dni":"30111222","address":"Calle 123"}'
```

Verificar email:
```sh
curl "http://localhost:8080/auth/verify-email?token=$TOKEN"
```

Response:
```json
{
  "status": "verified"
}
```

Reenviar verificacion:
```sh
curl -X POST http://localhost:8080/api/users/auth/verify-email/request \
  -H "Content-Type: application/json" \
  -d '{"email":"ana@example.com"}'
```

Response:
```json
{
  "status": "sent"
}
```

Endpoint interno para otros servicios:
```sh
curl http://localhost:8081/internal/users/$USER_ID/email-verified \
  -H "X-Internal-Token: $USERS_INTERNAL_TOKEN"
```

Response:
```json
{
  "userId": "user-id",
  "emailVerified": true
}
```

## Password reset

Flujo:
1. `POST /api/users/auth/password/forgot`
2. Respuesta siempre `200`, exista o no el email
3. Si el usuario existe, backend guarda hash SHA-256 del token en `password_reset_tokens`
4. Backend envia mail con `{APP_BASE_URL}/reset-password?token=<token>`
5. Frontend toma ese token y llama `POST /api/users/auth/password/reset`
6. Backend actualiza `users.password_hash`, marca `used_at` y cierra el token

Forgot password:
```sh
curl -X POST http://localhost:8080/api/users/auth/password/forgot \
  -H "Content-Type: application/json" \
  -d '{"email":"ana@example.com"}'
```

Response:
```json
{
  "status": "sent"
}
```

Reset password:
```sh
curl -X POST http://localhost:8080/api/users/auth/password/reset \
  -H "Content-Type: application/json" \
  -d '{"token":"'$TOKEN'","newPassword":"new-secret123"}'
```

Response:
```json
{
  "status": "password_updated"
}
```

## Errores esperados

- Token de verificacion invalido o expirado: `400 EMAIL_VERIFICATION_TOKEN_IS_INVALID_OR_EXPIRED`
- Token de reset invalido o expirado: `400 PASSWORD_RESET_TOKEN_IS_INVALID_OR_EXPIRED`
- Password corta: `400 PASSWORD_MUST_BE_AT_LEAST_8_CHARACTERS`
- Reserva sin email verificado: `403 EMAIL_NOT_VERIFIED`
