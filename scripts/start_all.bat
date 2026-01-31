@echo off
echo ==========================================
echo   Agentic Valuation Platform - Startup
echo ==========================================

:: Start Go Backend API
echo [1/2] Starting Go API Server...
start "Go API Server" cmd /k "go run cmd/api/main.go"


:: Wait for Backend Readiness
echo [Info] Waiting for backend to initialize (port 8080)...
:wait_loop
curl -s http://localhost:8080/api/config >nul 2>&1
if errorlevel 1 (
    timeout /t 1 >nul
    goto wait_loop
)
echo [Info] Backend is ready!

:: Start Next.js Frontend
echo [2/2] Starting Next.js Frontend...
start "Next.js Frontend" cmd /k "cd web-ui && npm run dev"

echo.
echo ==========================================
echo   Backend: http://localhost:8080
echo   Frontend: http://localhost:3000
echo ==========================================
pause
