# Build stage
FROM golang:1.19-alpine AS builder

# Set working directory
WORKDIR /app

# Install necessary build tools
RUN apk add --no-cache git make

# Copy go.mod and go.sum to download dependencies
COPY go.mod ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN make build

# Final stage
FROM alpine:latest

# Set working directory
WORKDIR /app

# Install SSL certificates
RUN apk add --no-cache ca-certificates

# Create directories for file storage
RUN mkdir -p /app/downloads /app/uploads

# Copy the binary from the builder stage
COPY --from=builder /app/bin/file-sharing-utility /app/

# Expose necessary ports
EXPOSE 8080 1080

# Set environment variables
ENV LISTEN_ADDR=0.0.0.0:8080
ENV SOCKS_ADDR=0.0.0.0:1080
ENV DOWNLOAD_PATH=/app/downloads
ENV UPLOAD_PATH=/app/uploads

# Run the application
CMD ["/app/file-sharing-utility", "-listen", "${LISTEN_ADDR}", "-socks", "${SOCKS_ADDR}", "-download-path", "${DOWNLOAD_PATH}", "-upload-path", "${UPLOAD_PATH}"] 