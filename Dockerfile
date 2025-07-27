# Dockerfile for Shopping Cart Service with OpenTelemetry Metrics
# Multi-stage build for optimal image size and security

# Build stage
FROM golang:1.21-alpine AS builder

# Set build arguments
ARG VERSION=1.0.0
ARG BUILD_TIME
ARG GIT_COMMIT

# Install git and ca-certificates (needed for fetching dependencies)
RUN apk add --no-cache git ca-certificates tzdata

# Create non-root user for security
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download
RUN go mod verify

# Copy source code
COPY main.go ./

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static" -X main.version='${VERSION}' -X main.buildTime='${BUILD_TIME}' -X main.gitCommit='${GIT_COMMIT} \
    -a -installsuffix cgo \
    -o cart-service main.go

# Final stage - minimal runtime image
FROM scratch

# Import from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/passwd /etc/passwd

# Copy the binary
COPY --from=builder /build/cart-service /cart-service

# Use non-root user
USER appuser

# Expose port
EXPOSE 8080

# Add health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/cart-service", "-health-check"]

# Set metadata labels
LABEL maintainer="devops@company.com" \
      description="Shopping Cart Service with OpenTelemetry Metrics" \
      version="${VERSION}" \
      org.opencontainers.image.title="shopping-cart-service" \
      org.opencontainers.image.description="A Go-based shopping cart service with comprehensive OpenTelemetry metrics" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_TIME}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.source="https://github.com/company/shopping-cart-service"

# Run the application
ENTRYPOINT ["/cart-service"]