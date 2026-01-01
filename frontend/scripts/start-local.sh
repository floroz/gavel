#!/bin/bash

# Configuration
NAMESPACE_APP="default"
NAMESPACE_INGRESS="ingress-nginx"

# Service Mappings (LocalPort:RemotePort)
# We forward to port 80 on the service, which maps to the container port
PORT_INGRESS=4000
PORT_AUTH=4001
PORT_BID=4002
PORT_STATS=4003

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}Starting local development environment connection...${NC}"

# Array to store PIDs of background processes
PIDS=()

# Cleanup function
cleanup() {
    echo -e "\n${RED}Stopping port-forwards...${NC}"
    for pid in "${PIDS[@]}"; do
        if ps -p $pid > /dev/null; then
            kill $pid
        fi
    done
    echo -e "${GREEN}Cleaned up. Goodbye!${NC}"
}

# Set trap for cleanup on SIGINT (Ctrl+C) and SIGTERM
trap cleanup SIGINT SIGTERM EXIT

# Helper function to start port forward
start_forward() {
    local name=$1
    local cmd=$2
    local port=$3
    
    echo -n "Starting $name on port $port... "
    $cmd > /dev/null 2>&1 &
    local pid=$!
    PIDS+=($pid)
    
    # Wait for port to be open
    local retries=0
    while ! nc -z localhost $port >/dev/null 2>&1; do
        sleep 0.5
        retries=$((retries+1))
        if [ $retries -gt 20 ]; then
            echo -e "${RED}Failed!${NC}"
            return 1
        fi
    done
    echo -e "${GREEN}Ready${NC}"
}

# 1. Forward Ingress (for Browser client)
# Note: Adjust service name if your ingress release name differs. 
# Standard helm install with name 'nginx-ingress' usually results in 'nginx-ingress-controller'
start_forward "Ingress" \
    "kubectl port-forward svc/nginx-ingress-ingress-nginx-controller -n $NAMESPACE_INGRESS $PORT_INGRESS:80" \
    $PORT_INGRESS

# 2. Forward Auth Service (for SSR)
start_forward "Auth Service" \
    "kubectl port-forward svc/auth-service -n $NAMESPACE_APP $PORT_AUTH:80" \
    $PORT_AUTH

# 3. Forward Bid Service (for SSR)
start_forward "Bid Service" \
    "kubectl port-forward svc/bid-service -n $NAMESPACE_APP $PORT_BID:80" \
    $PORT_BID

# 4. Forward User Stats Service (for SSR)
start_forward "User Stats Service" \
    "kubectl port-forward svc/user-stats-service -n $NAMESPACE_APP $PORT_STATS:80" \
    $PORT_STATS

echo -e "\n${GREEN}✔ All connections established!${NC}"
echo -e "----------------------------------------"
echo -e "Update your ${BLUE}frontend/.env.local${NC} with:"
echo -e "VITE_API_URL=http://localhost:$PORT_INGRESS"
echo -e "VITE_AUTH_SERVICE_URL=http://localhost:$PORT_AUTH"
echo -e "VITE_BID_SERVICE_URL=http://localhost:$PORT_BID"
echo -e "VITE_USER_STATS_SERVICE_URL=http://localhost:$PORT_STATS"
echo -e "----------------------------------------"
echo -e "Press ${RED}Ctrl+C${NC} to stop."

# Wait indefinitely to keep script running
wait