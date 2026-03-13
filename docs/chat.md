# Chat por curso

Base URL sugerida via gateway:

- `http://localhost:8080/api/chat`

Variables relevantes:

- `CHAT_API_PORT`
- `CHAT_DB_DSN`
- `SERVICE_NAME=chat-api`
- `JWT_SECRET`
- `COURSES_API_BASE_URL`
- `ENROLLMENTS_API_BASE_URL`
- `CHAT_INTERNAL_TOKEN`
- `APP_ENV=dev|prod`
- `CHAT_ALLOWED_ORIGINS`
- `CHAT_FANOUT_MODE=memory|redis|rabbit`
- `REDIS_ADDR`
- `RABBITMQ_URL`

## Permisos

- `admin`: acceso siempre.
- `teacher`: solo si esta asignado al curso.
- `student`: solo si tiene enrollment `active` en el curso.

Si no cumple, la API responde `403`.

## Endpoints

### Health

- `GET /health`
- `GET /ready`

### Historial

- `GET /courses/:courseId/messages?limit=&before=`
- `before` usa timestamp `RFC3339`, por ejemplo `2026-03-01T21:30:00Z`
- respuesta en orden ascendente dentro de la pagina

Ejemplo:

```bash
curl http://localhost:8080/api/chat/courses/COURSE_ID/messages?limit=50 \
  -H "Authorization: Bearer TOKEN"
```

Respuesta:

```json
[
  {
    "id": "7b0d4dc7-ef3f-4e6e-8a60-7c5ab9126f7a",
    "courseId": "COURSE_ID",
    "senderId": "USER_ID",
    "senderRole": "teacher",
    "content": "Bienvenidos al curso",
    "createdAt": "2026-03-01T21:30:00.000000Z"
  }
]
```

### Enviar mensaje por REST

- `POST /courses/:courseId/messages`

```bash
curl -X POST http://localhost:8080/api/chat/courses/COURSE_ID/messages \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content":"Hola equipo"}'
```

### Contactos para chat privado

- `GET /courses/:courseId/private/contacts`
- Responde los usuarios con los que se puede abrir un chat privado en ese curso.
- Reglas:
  - `teacher` ve alumnos con enrollment `active`.
  - `student` ve docentes asignados al curso.
  - `admin` ve ambos.

Ejemplo:

```bash
curl http://localhost:8080/api/chat/courses/COURSE_ID/private/contacts \
  -H "Authorization: Bearer TOKEN"
```

### Historial privado

- `GET /courses/:courseId/private/:otherUserId/messages?limit=&before=`
- Solo habilitado para pares `teacher <-> student` del mismo curso.

```bash
curl http://localhost:8080/api/chat/courses/COURSE_ID/private/OTHER_USER_ID/messages?limit=50 \
  -H "Authorization: Bearer TOKEN"
```

### Enviar mensaje privado por REST

- `POST /courses/:courseId/private/:otherUserId/messages`

```bash
curl -X POST http://localhost:8080/api/chat/courses/COURSE_ID/private/OTHER_USER_ID/messages \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content":"Hola por privado"}'
```

### WebSocket realtime

- `GET /ws/courses/:courseId`
- `GET /ws/courses/:courseId/private/:otherUserId`
- `Origin` valido es obligatorio.
- En `APP_ENV=dev` se permiten solo `http://localhost:*` y `http://127.0.0.1:*`.
- En `APP_ENV=prod` se permiten solo los origins configurados en `CHAT_ALLOWED_ORIGINS`.
- En `APP_ENV=dev` el browser puede seguir usando `?token=<jwt>`.
- En `APP_ENV=prod` no se acepta `?token=`. Usar `Authorization: Bearer ...` o `Sec-WebSocket-Protocol: bearer, <jwt>`.

Mensajes enviados al conectar:

```json
{
  "type": "hello",
  "courseId": "COURSE_ID",
  "userId": "USER_ID",
  "role": "student"
}
```

Mensaje saliente del cliente:

```json
{
  "type": "message",
  "content": "Hola a todos"
}
```

Broadcast del servidor:

```json
{
  "type": "message",
  "id": "MESSAGE_ID",
  "courseId": "COURSE_ID",
  "senderId": "USER_ID",
  "senderRole": "teacher",
  "content": "Hola a todos",
  "createdAt": "2026-03-01T21:30:00.000000Z"
}
```

Ejemplo JS en dev:

```ts
const token = localStorage.getItem("cursos_online_session");
const parsed = token ? JSON.parse(token) : null;
const ws = new WebSocket(
  `ws://localhost:8080/api/chat/ws/courses/${courseId}?token=${parsed?.token ?? ""}`,
);

ws.onmessage = (event) => {
  const payload = JSON.parse(event.data);
  console.log(payload);
};

ws.onopen = () => {
  ws.send(JSON.stringify({ type: "message", content: "Hola curso" }));
};
```

Ejemplo JS en prod:

```ts
const token = localStorage.getItem("cursos_online_session");
const parsed = token ? JSON.parse(token) : null;
const ws = new WebSocket(
  `wss://app.example.com/api/chat/ws/courses/${courseId}`,
  ["bearer", parsed?.token ?? ""],
);
```
