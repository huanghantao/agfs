# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev sqlite-dev python3 py3-pip npm \
    autoconf automake libtool python3-dev jq-dev fuse3-dev

# Install uv for Python package management
RUN pip3 install --break-system-packages uv

# Set working directory
WORKDIR /build

# Copy SDK first
COPY agfs-sdk /agfs-sdk

# Copy go mod files
COPY agfs-server/go.mod agfs-server/go.sum ./
RUN go mod download

# Copy source code
COPY agfs-server .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o agfs-server cmd/server/main.go

# Build agfs-shell
WORKDIR /build-shell
COPY agfs-shell .
RUN python3 build.py

# Build agfs-fuse
WORKDIR /build-fuse
COPY agfs-fuse .
RUN go build -o agfs-fuse ./cmd/agfs-fuse

# Runtime stage
FROM alpine:latest

# Install runtime dependencies (including Python 3 for agfs-shell and fuse3 for agfs-fuse)
RUN apk add --no-cache ca-certificates sqlite-libs python3 jq oniguruma fuse3 wget

# Create app directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/agfs-server .

# Copy configuration files to root
COPY --from=builder /build/config.example.yaml /config.example.yaml
COPY --from=builder /build/config.example.yaml /config.yaml

# Copy agfs-shell portable distribution
COPY --from=builder /build-shell/dist/agfs-shell-portable /usr/local/agfs-shell
RUN ln -s /usr/local/agfs-shell/agfs-shell /usr/local/bin/agfs-shell
RUN ln -s /usr/local/agfs-shell/agfs-shell /usr/local/bin/agfs

# Copy agfs-fuse binary
COPY --from=builder /build-fuse/agfs-fuse /usr/local/bin/agfs-fuse

# Copy docker entrypoint script
COPY agfs-server/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Create directories
RUN mkdir -p /data /mnt/agfs

# Expose default port
EXPOSE 8080

# Run agfs-server with agfs-fuse mount
ENTRYPOINT ["/docker-entrypoint.sh"]
