#!/bin/bash

# Croupier Services Stop Script
# This script stops all go-zero microservices

set -e

echo "Stopping Croupier Go-Zero Services..."

# Function to stop a service
stop_service() {
    local service_name=$1
    local pid_file="$service_name.pid"

    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        echo "Stopping $service_name (PID: $pid)..."

        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid"
            echo "$service_name stopped"
        else
            echo "$service_name process not found"
        fi

        rm -f "$pid_file"
    else
        echo "PID file for $service_name not found"
    fi
}

# Stop all services
stop_service "server"
stop_service "agent"
stop_service "edge"

echo "All services stopped successfully!"