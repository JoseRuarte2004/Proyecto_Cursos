# Frontend

Frontend SPA para `proyecto-cursos`.

## Stack

- React + Vite
- TypeScript
- TailwindCSS
- React Router
- TanStack Query
- React Hook Form + Zod
- framer-motion
- sonner

## Variables de entorno

Crear `frontend/.env` si querés override local:

```env
VITE_API_BASE_URL=http://localhost:8080/api
```

Default:

- `http://localhost:8080/api`

## Cómo correr

```sh
cd frontend
npm install
npm run dev
```

Frontend dev:

- `http://localhost:5173`

Build estático:

```sh
cd frontend
npm run build
```

El build queda en `frontend/dist` y Nginx lo sirve desde Docker Compose en:

- `http://localhost:8080`

## Rutas principales

Públicas:

- `/home`
- `/courses/:id`
- `/login`
- `/register`
- `/checkout/redirect`
- `/checkout/success`
- `/checkout/failure`

Alumno:

- `/me/courses`
- `/courses/:courseId/classroom`

Admin:

- `/admin`
- `/admin/courses`
- `/admin/courses/:id/teachers`
- `/admin/courses/:courseId/lessons`
- `/admin/users`
- `/admin/enrollments`

Teacher:

- `/teacher`
- `/teacher/my-courses`
- `/teacher/courses/:courseId/enrollments`
- `/teacher/courses/:courseId/lessons`

## Permisos y roles

- JWT en `localStorage` con key `cursos_online_session`
- El rol se resuelve desde `GET /me`
- Guards implementados:
  - `RequireAuth`
  - `RequireRole`

## Notas

- El checkout hoy es mock. El frontend usa `POST /orders`, recibe `checkoutUrl` y, si detecta `checkout.mock`, habilita una simulación visual del webhook para completar el flujo.
- Para que Nginx sirva la SPA compilada, necesitás `frontend/dist` generado con `npm run build`.
