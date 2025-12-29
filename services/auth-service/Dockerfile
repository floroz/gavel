FROM golang:1.25-alpine AS builder
WORKDIR /app

# Install git for potential dependencies
RUN apk add --no-cache git

# Install goose for migrations
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY pkg/ pkg/
COPY services/auth-service/ services/auth-service/

# Build binaries
RUN go build -o /bin/auth-service ./services/auth-service/cmd/api/main.go

# Final Stage
FROM alpine:3.21
WORKDIR /app

# Copy binaries
COPY --from=builder /bin/auth-service /app/auth-service
COPY --from=builder /go/bin/goose /app/goose

# Copy migrations
COPY services/auth-service/migrations /app/migrations

# Default command (can be overridden)
CMD ["/app/auth-service"]

