FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/courses-api ./services/courses-api

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /bin/courses-api /app/courses-api
EXPOSE 8082
CMD ["/app/courses-api"]
