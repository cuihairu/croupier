#!/bin/bash

# Croupier Services Startup Script
# This script starts all go-zero microservices

set -e

echo "Starting Croupier Go-Zero Services..."

# Function to start a service
start_service() {
    local service_name=$1
    local service_dir=$2

    echo "Starting $service_name service..."
    cd "$service_dir"

    if [ ! -f "go.mod" ]; then
        echo "go.mod not found in $service_dir, initializing..."
        go mod init "github.com/cuihairu/croupier/services/$service_name"
    fi

    # Build and run in background
    go build -o bin/$service_name .
    ./bin/$service_name &

    local pid=$!
    echo "$service_name started with PID: $pid"
    echo $pid > ../$service_name.pid

    cd ..
}

# Create bin directories
mkdir -p server/bin agent/bin edge/bin

# Start services
start_service "server" "server"
start_service "agent" "agent"
start_service "edge" "edge"

echo "All services started successfully!"
echo ""
echo "Service Status:"
echo "- Server Service: http://localhost:8888"
echo "- Agent Service:  http://localhost:8889"
echo "- Edge Service:   http://localhost:8890"
echo ""
echo "To stop services, run: ./stop-services.sh"