# Frontend

SPA React/Vite de `proyecto-cursos`.

## Deploy en Vercel

- Root directory: `frontend`
- Install command: `npm install`
- Build command: `npm run build`
- Output directory: `dist`

## Variable publica obligatoria

```env
VITE_API_BASE_URL=https://api.example.com/api
```

## Desarrollo local

Crear `frontend/.env` si queres un override local:

```env
VITE_API_BASE_URL=http://localhost:8080/api
```

Luego:

```sh
cd frontend
npm install
npm run dev
```

- frontend local: `http://localhost:5173`

## Produccion

En produccion el frontend no se sirve desde `nginx`. Se despliega por separado en Vercel y consume la API publica del backend.
