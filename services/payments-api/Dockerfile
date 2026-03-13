FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/payments-api ./services/payments-api

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /bin/payments-api /app/payments-api
EXPOSE 8085
CMD ["/app/payments-api"]
