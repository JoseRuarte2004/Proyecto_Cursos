# enrollments-api

Variables principales:
- `ENROLLMENTS_API_ADDR`
- `ENROLLMENTS_DB_DSN`
- `JWT_SECRET`
- `RABBITMQ_URL`
- `COURSES_API_BASE_URL`
- `COURSES_INTERNAL_TOKEN`
- `USERS_API_BASE_URL`
- `USERS_INTERNAL_TOKEN`
- `ENROLL_INTERNAL_TOKEN`

Endpoints:
- `GET /health`
- `GET /ready`
- `GET /metrics`
- `POST /enrollments/reserve`
- `POST /enrollments/confirm` (`X-Internal-Token`)
- `GET /me/enrollments`
- `GET /admin/enrollments`
- `GET /teacher/courses/:courseId/enrollments`
- `GET /courses/:courseId/availability`
- `GET /internal/users/:userId/courses/:courseId/pending` (`X-Internal-Token`)
- `GET /internal/courses/:courseId/students/:studentId/enrolled` (`X-Internal-Token`)
- `GET /internal/courses/:courseId/students` (`X-Internal-Token`)
- `DELETE /internal/courses/:courseId/enrollments` (`X-Internal-Token`)

Auth:
- Usuario: `Authorization: Bearer <jwt>`
- Interna: `X-Internal-Token: <ENROLL_INTERNAL_TOKEN>`

Curl minimo:
```sh
curl -X POST http://localhost:8084/enrollments/reserve \
  -H "Authorization: Bearer $TOKEN_STUDENT" \
  -H "Content-Type: application/json" \
  -d "{\"courseId\":\"$COURSE_ID\"}"
```

Nota:
- `POST /enrollments/reserve` ahora devuelve `403 EMAIL_NOT_VERIFIED` si el usuario no verifico su email en `users-api`.
