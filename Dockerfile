# Build stage
FROM golang:1.25.3-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application (consolidated binary)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /go-nd ./cmd/gond

# Runtime stage
FROM alpine:3.23

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /go-nd /app/go-nd

# Create non-root user
RUN adduser -D -u 1000 appuser
USER appuser

# Expose ports (HTTP and gRPC)
EXPOSE 8080 50051

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["/app/go-nd"]
