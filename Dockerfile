# Multi-stage build for TracePR
# Stage 1: Build the Go application
FROM golang:1.24-alpine AS go-builder

# Set working directory
WORKDIR /app

# Install git and ca-certificates for Go modules
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o tracepr .

# Stage 2: Python dependencies
FROM python:3.11-slim AS python-builder

# Set working directory
WORKDIR /app

# Copy requirements file
COPY requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir --user -r requirements.txt

# Stage 3: Final runtime image
FROM alpine:latest

# Install ca-certificates and python3
RUN apk --no-cache add ca-certificates python3 py3-pip

# Create non-root user
RUN addgroup -g 1001 -S tracepr && \
    adduser -S -D -H -u 1001 -h /app -s /sbin/nologin -G tracepr -g tracepr tracepr

# Set working directory
WORKDIR /app

# Copy the binary from go-builder stage
COPY --from=go-builder /app/tracepr .

# Copy Python dependencies from python-builder stage
COPY --from=python-builder /root/.local /home/tracepr/.local

# Copy configuration files
COPY prd.md ./
COPY alerts/ ./alerts/
COPY dashboard/ ./dashboard/

# Create necessary directories
RUN mkdir -p /app/logs /app/data && \
    chown -R tracepr:tracepr /app

# Switch to non-root user
USER tracepr

# Set environment variables
ENV PATH=/home/tracepr/.local/bin:$PATH
ENV PORT=8080
ENV ENVIRONMENT=production

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ./tracepr --help || exit 1

# Run the application
CMD ["./tracepr"] 