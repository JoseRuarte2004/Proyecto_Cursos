FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/enrollments-api ./services/enrollments-api

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /bin/enrollments-api /app/enrollments-api
EXPOSE 8084
CMD ["/app/enrollments-api"]
