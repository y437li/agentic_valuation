#!/bin/sh
# Docker entrypoint - starts both backend and frontend

echo "Starting TIED Platform..."

# Start backend in background
./api_server &
BACKEND_PID=$!
echo "Backend started (PID: $BACKEND_PID)"

# Start frontend
npm start &
FRONTEND_PID=$!
echo "Frontend started (PID: $FRONTEND_PID)"

# Wait for both processes
wait $BACKEND_PID $FRONTEND_PID
