# course-content-api

Variables principales:
- `COURSE_CONTENT_API_ADDR`
- `CONTENT_DB_DSN`
- `JWT_SECRET`
- `COURSES_API_BASE_URL`
- `COURSES_INTERNAL_TOKEN`
- `ENROLLMENTS_API_BASE_URL`
- `ENROLL_INTERNAL_TOKEN`

Endpoints:
- `GET /health`
- `GET /ready`
- `GET /metrics`
- `GET /courses/:courseId/lessons`
- `POST /courses/:courseId/lessons`
- `PATCH /courses/:courseId/lessons/:lessonId`
- `DELETE /courses/:courseId/lessons/:lessonId`

Auth:
- Usuario: `Authorization: Bearer <jwt>`
- Llamadas internas salientes: `X-Internal-Token` hacia `courses-api` y `enrollments-api`

Curl minimo:
```sh
curl -X POST http://localhost:8083/courses/$COURSE_ID/lessons \
  -H "Authorization: Bearer $TOKEN_ADMIN" \
  -H "Content-Type: application/json" \
  -d '{"title":"Intro","description":"Modulo 1","orderIndex":1,"videoUrl":"https://video.test/1"}'
```
