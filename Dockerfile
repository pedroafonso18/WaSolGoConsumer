# Build stage
FROM golang:1.24.4-alpine AS builder

WORKDIR /app

# Install git (for go mod) and build tools
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o wasolgo ./cmd/wasolgo/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/wasolgo /app/wasolgo
COPY --from=builder /app/.env /app/.env

# If you need CA certificates for HTTPS
RUN apk add --no-cache ca-certificates

CMD ["./wasolgo"]
