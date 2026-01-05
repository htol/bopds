# ===========================================
# BUILD ARGUMENTS (Defaults to Production)
# ===========================================
ARG GO_VERSION=1.25
ARG NODE_VERSION=22

# ===========================================
# STAGE 1: Base Build Image
# ===========================================
FROM golang:${GO_VERSION}-bookworm AS base

# Allow Go toolchain auto-download
ENV GOTOOLCHAIN=auto

# Install build dependencies (gcc, g++ for CGO/SQLite, Node.js)
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    g++ \
    libc6-dev \
    make \
    git \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js
ARG NODE_VERSION
RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_VERSION}.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# ===========================================
# STAGE 2: Frontend Build
# ===========================================
FROM base AS frontend-builder

# Copy frontend package files
COPY frontend/package*.json ./frontend/
RUN cd frontend && npm ci

# Copy frontend source and build
COPY frontend/ ./frontend/
RUN cd frontend && npm run build

# ===========================================
# STAGE 3: Backend Build
# ===========================================
FROM base AS backend-builder

# Download Go dependencies (cached layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy frontend build from previous stage
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

# Build the Go application with CGO enabled for SQLite support
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o bopds .

# ===========================================
# STAGE 4: Development Image
# ===========================================
FROM base AS development

# Install Air for hot reload (use version compatible with Go 1.23)
RUN go install github.com/cosmtrek/air@v1.49.0

# Set working directory
WORKDIR /app

# Download Go dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code (for hot-reload)
COPY . .

# Copy frontend package files and install dev dependencies
COPY frontend/package*.json ./frontend/
RUN cd frontend && npm install

# Expose port
EXPOSE 3001

# Set default environment variables
ENV PORT=3001
ENV DB_PATH=/data/books.db
ENV LIBRARY_PATH=/library
ENV LOG_LEVEL=debug

# Run with Air for hot-reload in development
CMD ["air", "-c", ".air.toml"]

# ===========================================
# STAGE 5: Production Image
# ===========================================
FROM debian:bookworm-slim AS production

# Install runtime dependencies only
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libsqlite3-0 \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user for security
RUN useradd -m -u 1000 bopds

# Set working directory
WORKDIR /app

# Copy binary from backend-builder
COPY --from=backend-builder /app/bopds /app/bopds

# Copy frontend build
COPY --from=backend-builder /app/frontend/dist /app/frontend/dist

# Create necessary directories
RUN mkdir -p /data /library && \
    chown -R bopds:bopds /app /data /library

# Switch to non-root user
USER bopds

# Expose port
EXPOSE 3001

# Set default environment variables
ENV PORT=3001
ENV DB_PATH=/data/books.db
ENV LIBRARY_PATH=/library
ENV LOG_LEVEL=info

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:3001/ || exit 1

# Run the application
CMD ["/app/bopds", "serve"]

# ===========================================
# STAGE 6: Target Selection (Default: Production)
# ===========================================
FROM production AS target
