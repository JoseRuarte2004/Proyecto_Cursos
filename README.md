# proyecto-cursos

## Entorno local

Copiar el ejemplo antes de levantar servicios:

```powershell
Copy-Item .env.example .env
```

```sh
cp .env.example .env
```

`.env` queda ignorado por git. Edita ese archivo localmente y no lo subas.

### Frontend local

- carpeta: `frontend`
- dev server: `http://localhost:5173`
- API local esperada: `http://localhost:8080/api`

### Backend local

El gateway publico local sigue siendo `nginx` en:

- `http://localhost:8080`

Los microservicios quedan detras del gateway y `infra/docker-compose.yml` sigue sirviendo para desarrollo local.

## Deploy objetivo

- frontend en Vercel
- backend en Render
- `nginx` solo como gateway/API publico
- frontend fuera de `nginx` en produccion

Guia concreta:

- [`docs/deploy-vercel-render.md`](docs/deploy-vercel-render.md)

## Hardening web

El reverse proxy `nginx` aplica headers defensivos sin tocar la logica del negocio:

- `X-Content-Type-Options`
- `X-Frame-Options`
- `Referrer-Policy`
- `Permissions-Policy`
- `security.txt` en `/.well-known/security.txt`

Comportamiento por entorno:

- `APP_ENV=dev`: CSP en `Report-Only` y sin HSTS para no romper desarrollo local
- `APP_ENV=production` con `PUBLIC_BASE_URL=https://...`: CSP en enforcement y HSTS activo

## Documentacion tecnica

- contrato API: `docs/api.md`
- auth/email: `docs/auth.md`
- chat realtime/permisos: `docs/chat.md`
