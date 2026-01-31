# TIED Platform - Docker Deployment
# Multi-stage build for minimal image size

# ============================================
# Stage 1: Build Go Backend
# ============================================
FROM golang:1.21-alpine AS backend-builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY pkg/ ./pkg/
COPY config/ ./config/

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o api_server ./cmd/api

# ============================================
# Stage 2: Build Next.js Frontend
# ============================================
FROM node:20-alpine AS frontend-builder

WORKDIR /app

# Copy package files
COPY web-ui/package*.json ./
RUN npm ci

# Copy source code
COPY web-ui/ ./

# Build
RUN npm run build

# ============================================
# Stage 3: Production Runtime
# ============================================
FROM alpine:3.19 AS production

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    nodejs \
    npm \
    pandoc

# Copy backend binary
COPY --from=backend-builder /app/api_server ./api_server
COPY --from=backend-builder /app/config ./config

# Copy frontend build
COPY --from=frontend-builder /app/.next ./.next
COPY --from=frontend-builder /app/node_modules ./node_modules
COPY --from=frontend-builder /app/package.json ./package.json
COPY --from=frontend-builder /app/public ./public

# Copy startup script
COPY scripts/docker-entrypoint.sh ./entrypoint.sh
RUN chmod +x ./entrypoint.sh

# Expose ports
EXPOSE 8082 3000

# Health check
HEALTHCHECK --interval=30s --timeout=3s \
    CMD wget -q --spider http://localhost:8082/api/config || exit 1

# Start services
ENTRYPOINT ["./entrypoint.sh"]
