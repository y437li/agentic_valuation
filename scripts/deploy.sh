#!/bin/bash
# TIED Platform - Deployment Script
# Usage: ./deploy.sh [dev|prod]

set -e

MODE=${1:-dev}
echo "=========================================="
echo "  TIED Platform - Deployment ($MODE)"
echo "=========================================="

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Check prerequisites
echo ""
echo "[1/5] Checking prerequisites..."

# Check Go
if ! command -v go &> /dev/null; then
    echo -e "${RED}✗ Go is not installed${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Go $(go version | cut -d' ' -f3)${NC}"

# Check Node.js
if ! command -v node &> /dev/null; then
    echo -e "${RED}✗ Node.js is not installed${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Node.js $(node --version)${NC}"

# Check Pandoc
if ! command -v pandoc &> /dev/null; then
    echo -e "${RED}✗ Pandoc is not installed${NC}"
    echo "  Install: brew install pandoc (Mac) or apt install pandoc (Linux)"
    exit 1
fi
echo -e "${GREEN}✓ Pandoc $(pandoc --version | head -1)${NC}"

# Check .env file
if [ ! -f ".env" ]; then
    echo -e "${RED}✗ .env file not found${NC}"
    echo "  Copy .env.example to .env and configure"
    exit 1
fi
echo -e "${GREEN}✓ .env file exists${NC}"

# Install dependencies
echo ""
echo "[2/5] Installing Go dependencies..."
go mod download
go mod tidy
echo -e "${GREEN}✓ Go dependencies installed${NC}"

echo ""
echo "[3/5] Installing Node.js dependencies..."
cd web-ui
npm install
cd ..
echo -e "${GREEN}✓ Node.js dependencies installed${NC}"

# Build
echo ""
echo "[4/5] Building..."

if [ "$MODE" = "prod" ]; then
    # Production build
    echo "Building Go binary..."
    CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/api_server ./cmd/api
    echo -e "${GREEN}✓ Go binary: dist/api_server${NC}"
    
    echo "Building Next.js..."
    cd web-ui
    npm run build
    cd ..
    echo -e "${GREEN}✓ Next.js build complete${NC}"
else
    # Dev build - just verify compilation
    go build ./cmd/api
    echo -e "${GREEN}✓ Go build verified${NC}"
fi

# Start services
echo ""
echo "[5/5] Starting services..."

if [ "$MODE" = "prod" ]; then
    echo "Production mode - use PM2 or systemd to manage processes"
    echo "  Backend: ./dist/api_server"
    echo "  Frontend: cd web-ui && npm start"
else
    # Development mode
    echo "Starting in development mode..."
    
    # Start backend
    go run cmd/api/main.go &
    BACKEND_PID=$!
    echo "Backend started (PID: $BACKEND_PID)"
    
    # Start frontend
    cd web-ui
    npm run dev &
    FRONTEND_PID=$!
    echo "Frontend started (PID: $FRONTEND_PID)"
    cd ..
    
    echo ""
    echo "=========================================="
    echo "  Backend:  http://localhost:8082"
    echo "  Frontend: http://localhost:3000"
    echo "=========================================="
    echo "Press Ctrl+C to stop all services"
    
    # Wait for Ctrl+C
    trap "kill $BACKEND_PID $FRONTEND_PID 2>/dev/null" EXIT
    wait
fi
