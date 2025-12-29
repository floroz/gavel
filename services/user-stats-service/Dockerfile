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
COPY services/user-stats-service/ services/user-stats-service/

# Build binaries
RUN go build -o /bin/stats-service ./services/user-stats-service/cmd/api/main.go
RUN go build -o /bin/stats-worker ./services/user-stats-service/cmd/worker/main.go

# Final Stage
FROM alpine:3.21
WORKDIR /app

# Copy binaries
COPY --from=builder /bin/stats-service /app/stats-service
COPY --from=builder /bin/stats-worker /app/stats-worker
COPY --from=builder /go/bin/goose /app/goose

# Copy migrations
COPY services/user-stats-service/migrations /app/migrations

# Default command
CMD ["/app/stats-service"]
