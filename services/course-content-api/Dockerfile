FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/course-content-api ./services/course-content-api

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /bin/course-content-api /app/course-content-api
EXPOSE 8083
CMD ["/app/course-content-api"]
