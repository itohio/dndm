# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY ../../../go.mod ../../../go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY ../../../. .

# Build embedded component
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/embedded ./examples/embedded/embedded

# Build host component
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/host ./examples/embedded/host

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binaries from builder
COPY --from=builder /app/bin/embedded ./bin/embedded
COPY --from=builder /app/bin/host ./bin/host

# Default command (will be overridden by docker-compose)
CMD ["sh"]
