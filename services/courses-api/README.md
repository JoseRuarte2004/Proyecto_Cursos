# courses-api

Variables principales:
- `COURSES_API_ADDR`
- `COURSES_DB_DSN`
- `COURSES_REDIS_ADDR`
- `COURSES_CACHE_TTL`
- `JWT_SECRET`
- `COURSES_INTERNAL_TOKEN`
- `USERS_API_BASE_URL`
- `USERS_INTERNAL_TOKEN`
- `ENROLLMENTS_API_BASE_URL`
- `ENROLL_INTERNAL_TOKEN`
- `OPENAI_API_KEY`
- `OPENAI_MODEL`

Endpoints:
- `GET /health`
- `GET /ready`
- `GET /metrics`
- `GET /courses`
- `GET /courses/:id`
- `POST /recommend`
- `POST /courses`
- `PATCH /courses/:id`
- `DELETE /courses/:id`
- `POST /courses/:id/teachers`
- `GET /courses/:id/teachers`
- `DELETE /courses/:id/teachers/:teacherId`
- `GET /teacher/me/courses`
- `GET /internal/courses/:id` (`X-Internal-Token`)
- `GET /internal/courses/:id/teachers/:teacherId/assigned` (`X-Internal-Token`)
- `GET /internal/courses/:id/teachers` (`X-Internal-Token`)

Nota:
- Al borrar un curso (`DELETE /courses/:id`) se eliminan tambien sus inscripciones en `enrollments-api` via endpoint interno.

Auth:
- Usuario: `Authorization: Bearer <jwt>`
- Interna: `X-Internal-Token: <COURSES_INTERNAL_TOKEN>`

Curl minimo:
```sh
curl -X POST http://localhost:8082/courses \
  -H "Authorization: Bearer $TOKEN_ADMIN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Go","description":"Backend","category":"backend","price":100,"currency":"USD","capacity":10,"status":"draft"}'
```
