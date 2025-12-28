FROM golang:1.24-alpine AS builder
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
COPY services/bid-service/ services/bid-service/

# Build binaries
RUN go build -o /bin/bid-service ./services/bid-service/cmd/api/main.go
RUN go build -o /bin/bid-worker ./services/bid-service/cmd/worker/main.go

# Final Stage
FROM alpine:3.21
WORKDIR /app

# Copy binaries
COPY --from=builder /bin/bid-service /app/bid-service
COPY --from=builder /bin/bid-worker /app/bid-worker
COPY --from=builder /go/bin/goose /app/goose

# Copy migrations
COPY services/bid-service/migrations /app/migrations

# Default command (can be overridden)
CMD ["/app/bid-service"]
