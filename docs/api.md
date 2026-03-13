# API para Frontend

Host sugerido via Nginx:
- `http://localhost:8080`

Base URL recomendada para frontend:
- `http://localhost:8080/api`

Compatibilidad:
- Las rutas actuales directas siguen existiendo (`/users/*`, `/courses/*`, `/course-content/*`, `/enrollments/*`, `/payments/*`).
- Para frontend conviene usar el gateway con prefijos por servicio:
  - `http://localhost:8080/api/users`
  - `http://localhost:8080/api/courses`
  - `http://localhost:8080/api/content`
  - `http://localhost:8080/api/enrollments`
  - `http://localhost:8080/api/payments`

Headers comunes:
- `Authorization: Bearer <token>`
- `Content-Type: application/json`
- `X-Request-Id: <uuid>` opcional

Errores:
```json
{
  "error": {
    "code": "FORBIDDEN",
    "message": "forbidden"
  },
  "requestId": "8a0a7c7e-77e5-4f3d-bc08-7c3c1f76d3f9"
}
```

## Auth

### POST /auth/register
Proposito: crear usuario alumno.

Gateway:
- `POST /api/users/auth/register`

Curl:
```sh
curl -X POST http://localhost:8080/api/users/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Ana","email":"ana@example.com","password":"secret123","phone":"111","dni":"30111222","address":"Calle 123"}'
```

Response:
```json
{
  "id": "c4c0f93c-e6d8-46f5-a4d1-b45127b87c31",
  "name": "Ana",
  "email": "ana@example.com",
  "role": "student"
}
```

Notas:
- El response no cambia.
- El usuario se crea con `emailVerified=false`.
- El backend envia un mail de verificacion con link a `/auth/verify-email`.

### POST /auth/login
Proposito: autenticar usuario y obtener JWT.

Gateway:
- `POST /api/users/auth/login`

Curl:
```sh
curl -X POST http://localhost:8080/api/users/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"ana@example.com","password":"secret123"}'
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "c4c0f93c-e6d8-46f5-a4d1-b45127b87c31",
    "name": "Ana",
    "email": "ana@example.com",
    "role": "student"
  }
}
```

### GET /auth/verify-email?token=
Proposito: verificar el email del usuario usando el token recibido por mail.

Gateway:
- `GET /auth/verify-email?token=...`

Curl:
```sh
curl "http://localhost:8080/auth/verify-email?token=$TOKEN"
```

Response:
```json
{
  "status": "verified"
}
```

### POST /auth/verify-email/request
Proposito: reenviar email de verificacion.

Gateway:
- `POST /api/users/auth/verify-email/request`

Curl:
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

### POST /auth/password/forgot
Proposito: iniciar recuperacion de password.

Gateway:
- `POST /api/users/auth/password/forgot`

Curl:
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

Nota:
- Siempre devuelve `200` para no revelar si el email existe.

### POST /auth/password/reset
Proposito: actualizar password con token de recuperacion.

Gateway:
- `POST /api/users/auth/password/reset`

Curl:
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

### GET /me
Proposito: obtener perfil del usuario autenticado.

Gateway:
- `GET /api/users/me`

Curl:
```sh
curl http://localhost:8080/api/users/me \
  -H "Authorization: Bearer $TOKEN"
```

Response:
```json
{
  "id": "c4c0f93c-e6d8-46f5-a4d1-b45127b87c31",
  "name": "Ana",
  "email": "ana@example.com",
  "role": "student"
}
```

## Catalogo

### GET /courses?limit=&offset=
Proposito: listar cursos publicados para el catalogo.

Gateway:
- `GET /api/courses/courses?limit=10&offset=0`

Curl:
```sh
curl "http://localhost:8080/api/courses/courses?limit=10&offset=0"
```

Response:
```json
[
  {
    "id": "f68d1cb4-1520-42b1-8399-9ea14bbfd4b1",
    "title": "Go Avanzado",
    "description": "Testing y concurrencia",
    "category": "backend",
    "imageUrl": "https://cdn.example.com/go.png",
    "price": 19999.9,
    "currency": "ARS",
    "capacity": 40,
    "status": "published",
    "createdBy": "admin-id",
    "createdAt": "2026-02-28T20:10:00Z",
    "updatedAt": "2026-02-28T20:20:00Z"
  }
]
```

### GET /courses/:id
Proposito: ver detalle de un curso publicado.

Gateway:
- `GET /api/courses/courses/:id`

Curl:
```sh
curl http://localhost:8080/api/courses/courses/$COURSE_ID
```

Response:
```json
{
  "id": "f68d1cb4-1520-42b1-8399-9ea14bbfd4b1",
  "title": "Go Avanzado",
  "description": "Testing y concurrencia",
  "category": "backend",
  "imageUrl": "https://cdn.example.com/go.png",
  "price": 19999.9,
  "currency": "ARS",
  "capacity": 40,
  "status": "published",
  "createdBy": "admin-id",
  "createdAt": "2026-02-28T20:10:00Z",
  "updatedAt": "2026-02-28T20:20:00Z"
}
```

### GET /courses/:courseId/lessons
Proposito: listar clases del curso si el usuario tiene permiso.

Gateway:
- `GET /api/content/courses/:courseId/lessons`

Curl:
```sh
curl http://localhost:8080/api/content/courses/$COURSE_ID/lessons \
  -H "Authorization: Bearer $TOKEN"
```

Response:
```json
[
  {
    "id": "lesson-1",
    "courseId": "f68d1cb4-1520-42b1-8399-9ea14bbfd4b1",
    "title": "Introduccion",
    "description": "Primer modulo",
    "orderIndex": 1,
    "videoUrl": "https://video.test/intro",
    "createdAt": "2026-02-28T20:30:00Z",
    "updatedAt": "2026-02-28T20:30:00Z"
  }
]
```

## Inscripcion + Pago

### POST /enrollments/reserve
Proposito: reservar cupo para un alumno.

Gateway:
- `POST /api/enrollments/enrollments/reserve`

Curl:
```sh
curl -X POST http://localhost:8080/api/enrollments/enrollments/reserve \
  -H "Authorization: Bearer $TOKEN_STUDENT" \
  -H "Content-Type: application/json" \
  -d "{\"courseId\":\"$COURSE_ID\"}"
```

Response:
```json
{
  "id": "enrollment-id",
  "userId": "student-id",
  "courseId": "f68d1cb4-1520-42b1-8399-9ea14bbfd4b1",
  "status": "pending",
  "createdAt": "2026-02-28T20:40:00Z"
}
```

Nota:
- Si el usuario no verifico su email, devuelve `403 EMAIL_NOT_VERIFIED`.

### POST /orders
Proposito: crear una orden y obtener `checkoutUrl` para redireccion.

Gateway:
- `POST /api/payments/orders`

Curl:
```sh
curl -X POST http://localhost:8080/api/payments/orders \
  -H "Authorization: Bearer $TOKEN_STUDENT" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: front-order-001" \
  -d "{\"courseId\":\"$COURSE_ID\",\"provider\":\"mercadopago\"}"
```

Response:
```json
{
  "orderId": "order-id",
  "checkoutUrl": "https://checkout.mock/mercadopago/order-id",
  "provider": "mercadopago",
  "idempotencyKey": "front-order-001",
  "status": "created"
}
```

Nota:
- `checkoutUrl` hoy es mock. El frontend igual debe redirigir ahi para dejar el flujo listo.
- No hay integracion real con Mercado Pago todavia.

### GET /me/enrollments
Proposito: ver cursos reservados/activos del alumno.

Gateway:
- `GET /api/enrollments/me/enrollments`

Curl:
```sh
curl http://localhost:8080/api/enrollments/me/enrollments \
  -H "Authorization: Bearer $TOKEN_STUDENT"
```

Response:
```json
[
  {
    "courseId": "f68d1cb4-1520-42b1-8399-9ea14bbfd4b1",
    "status": "active",
    "createdAt": "2026-02-28T20:40:00Z",
    "course": {
      "id": "f68d1cb4-1520-42b1-8399-9ea14bbfd4b1",
      "title": "Go Avanzado",
      "category": "backend",
      "imageUrl": "https://cdn.example.com/go.png",
      "price": 19999.9,
      "currency": "ARS",
      "status": "published"
    }
  }
]
```

## Admin

### POST /courses
Proposito: crear curso.

Gateway:
- `POST /api/courses/courses`

Curl:
```sh
curl -X POST http://localhost:8080/api/courses/courses \
  -H "Authorization: Bearer $TOKEN_ADMIN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Nuevo curso","description":"Desc","category":"backend","price":100,"currency":"USD","capacity":20,"status":"draft"}'
```

Response:
```json
{
  "id": "course-id",
  "title": "Nuevo curso",
  "description": "Desc",
  "category": "backend",
  "price": 100,
  "currency": "USD",
  "capacity": 20,
  "status": "draft",
  "createdBy": "admin-id",
  "createdAt": "2026-02-28T20:10:00Z",
  "updatedAt": "2026-02-28T20:10:00Z"
}
```

### PATCH /courses/:id
Proposito: editar curso o publicarlo.

Gateway:
- `PATCH /api/courses/courses/:id`

Curl:
```sh
curl -X PATCH http://localhost:8080/api/courses/courses/$COURSE_ID \
  -H "Authorization: Bearer $TOKEN_ADMIN" \
  -H "Content-Type: application/json" \
  -d '{"status":"published"}'
```

Response:
```json
{
  "id": "course-id",
  "title": "Nuevo curso",
  "description": "Desc",
  "category": "backend",
  "price": 100,
  "currency": "USD",
  "capacity": 20,
  "status": "published",
  "createdBy": "admin-id",
  "createdAt": "2026-02-28T20:10:00Z",
  "updatedAt": "2026-02-28T20:20:00Z"
}
```

### POST /courses/:id/teachers
Proposito: asignar profesor a un curso.

Gateway:
- `POST /api/courses/courses/:id/teachers`

Curl:
```sh
curl -X POST http://localhost:8080/api/courses/courses/$COURSE_ID/teachers \
  -H "Authorization: Bearer $TOKEN_ADMIN" \
  -H "Content-Type: application/json" \
  -d "{\"teacherId\":\"$TEACHER_ID\"}"
```

Response:
```json
{
  "status": "assigned"
}
```

### POST /courses/:courseId/lessons
Proposito: crear lesson de un curso.

Gateway:
- `POST /api/content/courses/:courseId/lessons`

Curl:
```sh
curl -X POST http://localhost:8080/api/content/courses/$COURSE_ID/lessons \
  -H "Authorization: Bearer $TOKEN_ADMIN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Modulo 1","description":"Intro","orderIndex":1,"videoUrl":"https://video.test/1"}'
```

Response:
```json
{
  "id": "lesson-id",
  "courseId": "course-id",
  "title": "Modulo 1",
  "description": "Intro",
  "orderIndex": 1,
  "videoUrl": "https://video.test/1",
  "createdAt": "2026-02-28T20:30:00Z",
  "updatedAt": "2026-02-28T20:30:00Z"
}
```

## Teacher

### GET /teacher/me/courses
Proposito: listar cursos asignados al profesor autenticado.

Gateway:
- `GET /api/courses/teacher/me/courses`

Curl:
```sh
curl http://localhost:8080/api/courses/teacher/me/courses \
  -H "Authorization: Bearer $TOKEN_TEACHER"
```

Response:
```json
[
  {
    "id": "course-id",
    "title": "Go Avanzado",
    "description": "Testing y concurrencia",
    "category": "backend",
    "price": 19999.9,
    "currency": "ARS",
    "capacity": 40,
    "status": "published",
    "createdBy": "admin-id",
    "createdAt": "2026-02-28T20:10:00Z",
    "updatedAt": "2026-02-28T20:20:00Z"
  }
]
```

## Flujo Comprar

Flujo recomendado para frontend:
1. `POST /api/enrollments/enrollments/reserve` con `{ "courseId": "..." }`
2. `POST /api/payments/orders` con `{ "courseId": "...", "provider": "mercadopago" }`
3. Backend devuelve `{ orderId, checkoutUrl }`
4. Front hace redirect a `checkoutUrl` con `window.location.href = checkoutUrl`
5. Al volver, front consulta `GET /api/enrollments/me/enrollments`

Notas:
- `checkoutUrl` hoy es mock y sirve para congelar el contrato del front.
- El backend ya deja listo el punto donde despues se enchufa Mercado Pago real sin cambiar este flujo.
