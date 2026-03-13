FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/users-api ./services/users-api

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /bin/users-api /app/users-api
EXPOSE 8081
CMD ["/app/users-api"]
