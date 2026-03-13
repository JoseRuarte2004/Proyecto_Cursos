# proyecto-cursos

## Configuracion local

Copiar el ejemplo antes de levantar servicios:

```powershell
Copy-Item .env.example .env
```

```sh
cp .env.example .env
```

`.env` queda ignorado por git. Edita ese archivo localmente y no lo subas.

## Como usar desde Front

Para el frontend usa:
- `API_BASE_URL=http://localhost:8080/api`

Ejemplo de login:
```sh
curl -X POST http://localhost:8080/api/users/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"admin1234"}'
```

Luego envia el token en cada request:
```sh
curl http://localhost:8080/api/users/me \
  -H "Authorization: Bearer $TOKEN"
```

Nota de CORS:
- El gateway permite por defecto el origen `http://localhost:5173`
- Se puede cambiar con `FRONT_ORIGIN` en `.env`

## Hardening web

El reverse proxy `nginx` aplica headers defensivos sin tocar la logica del negocio:
- `X-Content-Type-Options`
- `X-Frame-Options`
- `Referrer-Policy`
- `Permissions-Policy`
- `security.txt` en `/.well-known/security.txt`

Comportamiento por entorno:
- `APP_ENV=dev`: CSP en `Report-Only` y sin HSTS para no romper desarrollo local
- `APP_ENV=staging` o `APP_ENV=prod` con `PUBLIC_BASE_URL=https://...`: CSP en enforcement y HSTS activo

Variables nuevas de `security.txt`:
- `SECURITY_TXT_CONTACT`
- `SECURITY_TXT_EXPIRES`
- `SECURITY_TXT_LANGUAGES`
- `SECURITY_TXT_CANONICAL`

Contrato recomendado para frontend:
- Ver `docs/api.md`
- Flujos auth/email: `docs/auth.md`
- Chat realtime/permisos: `docs/chat.md`

## Webhooks de Mercado Pago en local

Para desarrollo local con Docker Desktop, el entrypoint publico correcto no es `payments-api:8085`, sino `nginx` en `http://localhost:8080`, porque ese gateway ya publica `/api/payments/*` hacia `payments-api`.

Usa estas variables en `.env`:
- `NGINX_PORT=8080`
- `PUBLIC_BASE_URL=http://localhost:8080`
- `WEBHOOK_SECRET_MERCADOPAGO=<tu_secreto_local>`

Docker ya publica los puertos relevantes:
- gateway publico: `8080 -> nginx:80`
- payments-api directo para debug: `8085 -> payments-api:8085`

Verificacion local:

```powershell
Invoke-WebRequest http://localhost:8080/health
Invoke-WebRequest http://localhost:8085/ready
```

Prueba del endpoint webhook por el gateway:

```powershell
Invoke-RestMethod `
  -Method Post `
  -Uri 'http://localhost:8080/api/payments/webhooks/mercadopago' `
  -Headers @{ 'X-Webhook-Secret' = 'tu_secreto_local' } `
  -ContentType 'application/json' `
  -Body '{"orderId":"test-order","status":"paid"}'
```

Ese request puede devolver error de dominio como `order not found`, pero si responde desde `payments-api` significa que el routing esta bien.

URL publica correcta del webhook:

```text
{PUBLIC_BASE_URL}/api/payments/webhooks/mercadopago
```

Ejemplos:
- local: `http://localhost:8080/api/payments/webhooks/mercadopago`
- ngrok: `https://abc123.ngrok.app/api/payments/webhooks/mercadopago`
- Cloudflare Tunnel: `https://demo.trycloudflare.com/api/payments/webhooks/mercadopago`

Comandos de tunel recomendados:

```powershell
ngrok http 8080
cloudflared tunnel --url http://localhost:8080
```

Cuando obtengas la URL publica, actualiza `.env`:

```dotenv
PUBLIC_BASE_URL=https://abc123.ngrok.app
```

Luego recrea `payments-api`:

```powershell
docker compose --env-file .env -f infra/docker-compose.yml up --build -d payments-api nginx
```

El codigo ya quedo preparado para evitar hardcodeos:
- `infra/docker-compose.yml`: inyecta `PUBLIC_BASE_URL` en `payments-api`
- `services/payments-api/internal/config/config.go`: lee `PUBLIC_BASE_URL`
- `services/payments-api/internal/app/public_urls.go`: construye la webhook URL publica

La ruta publica debe entrar siempre por `nginx`:
- publico: `/api/payments/webhooks/mercadopago`
- interno directo de servicio: `/webhooks/mercadopago`

Si usas Mercado Pago real, configura su webhook con la URL publica y no con `localhost`.
