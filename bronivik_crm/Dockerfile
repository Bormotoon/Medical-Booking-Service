FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates gcc musl-dev

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
RUN apk --no-cache add ca-certificates tzdata sqlite-libs

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

