FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN set -eux; \
    printf '%s\n' \
      'https://dl-cdn.alpinelinux.org/alpine/v3.22/main' \
      'https://dl-cdn.alpinelinux.org/alpine/v3.22/community' > /etc/apk/repositories; \
    ok=0; \
    for i in 1 2 3; do \
      if apk add --no-cache git ca-certificates gcc musl-dev; then ok=1; break; fi; \
      echo "apk install failed, retry $i/3"; \
      sleep $((i * 3)); \
    done; \
    [ "$ok" -eq 1 ]

# Copy go.mod and go.sum
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project
COPY . .

# Build the application with CGO enabled for SQLite
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o /bronivik-crm ./cmd/bot

# Final image
FROM alpine:latest

# Install runtime dependencies
RUN set -eux; \
    printf '%s\n' \
      'https://dl-cdn.alpinelinux.org/alpine/v3.22/main' \
      'https://dl-cdn.alpinelinux.org/alpine/v3.22/community' > /etc/apk/repositories; \
    ok=0; \
    for i in 1 2 3; do \
      if apk add --no-cache ca-certificates tzdata sqlite-libs; then ok=1; break; fi; \
      echo "apk install failed, retry $i/3"; \
      sleep $((i * 3)); \
    done; \
    [ "$ok" -eq 1 ]

WORKDIR /app

# Copy binary from builder
COPY --from=builder /bronivik-crm .

# Create directories
RUN mkdir -p /app/configs /app/data /app/logs

# Copy configs
COPY configs/ ./configs/

# Add healthcheck
HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --spider http://localhost:8090/healthz || exit 1

# Run the application
CMD ["./bronivik-crm"]

