# chat-api

Microservicio de chat en tiempo real con:
- Go + Chi + Gorilla WebSocket
- PostgreSQL (persistencia de mensajes)
- Docker

## Variables de entorno

- `CHAT_API_ADDR` (default `:8082`)
- `CHAT_DB_DSN` (obligatoria)
- `JWT_SECRET` (default `change-me`)
- `CHAT_SERVICE_NAME` (default `chat-api`)
- `COURSES_API_BASE_URL`
- `COURSES_INTERNAL_TOKEN`
- `ENROLLMENTS_API_BASE_URL`
- `ENROLL_INTERNAL_TOKEN`
- `USERS_API_BASE_URL`
- `USERS_INTERNAL_TOKEN`

## Endpoints

- `GET /api/chat/history/{room_id}?limit=50&token=<jwt>`
- `GET /api/chat/ws?token=<jwt>&room=class_123`
- `GET /courses/{courseId}/messages`
- `POST /courses/{courseId}/messages`
- `GET /courses/{courseId}/private/contacts`
- `GET /courses/{courseId}/private/{otherUserId}/messages`
- `POST /courses/{courseId}/private/{otherUserId}/messages`
- `GET /ws/courses/{courseId}`
- `GET /ws/courses/{courseId}/private/{otherUserId}`

Aliases directos del servicio (sin prefijo gateway):
- `GET /history/{room_id}`
- `GET /ws`

## Estructura

```text
services/chat-api/
  Dockerfile
  main.go
  README.md
  migrations/
    0001_init.sql
    0002_ensure_messages_table.sql
    embed.go
  internal/
    app/
      jwt.go
      migrations.go
      router.go
    config/
      config.go
    domain/
      message.go
    repository/
      postgres.go
    service/
      service.go
    ws/
      client.go
      hub.go
```
