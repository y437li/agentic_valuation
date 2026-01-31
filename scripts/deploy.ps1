# TIED Platform - Deployment Script (Windows)
# Usage: .\deploy.ps1 [-Mode dev|prod]

param(
    [string]$Mode = "dev"
)

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  TIED Platform - Deployment ($Mode)" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

# Check prerequisites
Write-Host ""
Write-Host "[1/5] Checking prerequisites..." -ForegroundColor Yellow

# Check Go
try {
    $goVersion = go version
    Write-Host "✓ $goVersion" -ForegroundColor Green
} catch {
    Write-Host "✗ Go is not installed" -ForegroundColor Red
    exit 1
}

# Check Node.js
try {
    $nodeVersion = node --version
    Write-Host "✓ Node.js $nodeVersion" -ForegroundColor Green
} catch {
    Write-Host "✗ Node.js is not installed" -ForegroundColor Red
    exit 1
}

# Check Pandoc
try {
    $pandocVersion = pandoc --version | Select-Object -First 1
    Write-Host "✓ $pandocVersion" -ForegroundColor Green
} catch {
    Write-Host "✗ Pandoc is not installed" -ForegroundColor Red
    Write-Host "  Install: winget install JohnMacFarlane.Pandoc" -ForegroundColor Yellow
    exit 1
}

# Check .env file
if (-Not (Test-Path ".env")) {
    Write-Host "✗ .env file not found" -ForegroundColor Red
    Write-Host "  Copy .env.example to .env and configure" -ForegroundColor Yellow
    exit 1
}
Write-Host "✓ .env file exists" -ForegroundColor Green

# Install dependencies
Write-Host ""
Write-Host "[2/5] Installing Go dependencies..." -ForegroundColor Yellow
go mod download
go mod tidy
Write-Host "✓ Go dependencies installed" -ForegroundColor Green

Write-Host ""
Write-Host "[3/5] Installing Node.js dependencies..." -ForegroundColor Yellow
Push-Location web-ui
npm install
Pop-Location
Write-Host "✓ Node.js dependencies installed" -ForegroundColor Green

# Build
Write-Host ""
Write-Host "[4/5] Building..." -ForegroundColor Yellow

if ($Mode -eq "prod") {
    # Production build
    Write-Host "Building Go binary..."
    $env:CGO_ENABLED = 0
    go build -ldflags="-s -w" -o dist/api_server.exe ./cmd/api
    Write-Host "✓ Go binary: dist/api_server.exe" -ForegroundColor Green
    
    Write-Host "Building Next.js..."
    Push-Location web-ui
    npm run build
    Pop-Location
    Write-Host "✓ Next.js build complete" -ForegroundColor Green
} else {
    # Dev build - just verify compilation
    go build ./cmd/api
    Write-Host "✓ Go build verified" -ForegroundColor Green
}

# Start services
Write-Host ""
Write-Host "[5/5] Starting services..." -ForegroundColor Yellow

if ($Mode -eq "prod") {
    Write-Host "Production mode - run these commands manually:" -ForegroundColor Yellow
    Write-Host "  Backend:  .\dist\api_server.exe" -ForegroundColor White
    Write-Host "  Frontend: cd web-ui; npm start" -ForegroundColor White
} else {
    # Development mode - start in separate windows
    Write-Host "Starting in development mode..." -ForegroundColor Yellow
    
    # Start backend in new window
    Start-Process powershell -ArgumentList "-NoExit", "-Command", "go run cmd/api/main.go" -WorkingDirectory $PWD
    
    # Start frontend in new window
    Start-Process powershell -ArgumentList "-NoExit", "-Command", "npm run dev" -WorkingDirectory "$PWD\web-ui"
    
    Write-Host ""
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host "  Backend:  http://localhost:8082" -ForegroundColor White
    Write-Host "  Frontend: http://localhost:3000" -ForegroundColor White
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host "Close the spawned windows to stop services" -ForegroundColor Yellow
}
