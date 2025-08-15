#!/bin/bash

set -e

echo "🎯 Testing Queue-Based Auto-Scaling"
echo "=================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check prerequisites
echo -e "${BLUE}Checking prerequisites...${NC}"
if ! command_exists kubectl; then
    echo -e "${RED}Error: kubectl not found${NC}"
    exit 1
fi

if ! command_exists minikube; then
    echo -e "${RED}Error: minikube not found${NC}"
    exit 1
fi

# Check if minikube is running
if ! minikube status >/dev/null 2>&1; then
    echo -e "${RED}Error: minikube is not running${NC}"
    echo "Please start minikube first: minikube start"
    exit 1
fi

# Function to get worker replica count
get_worker_replicas() {
    kubectl get deployment worker -n k8s-learning -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "0"
}

# Function to get queue depth (mock - would need Redis connection)
get_queue_depth() {
    # This is a mock function - in reality you'd query Redis
    # For demo purposes, we'll simulate queue depth
    echo $(( RANDOM % 50 ))
}

echo -e "${GREEN}✓ Prerequisites check passed${NC}"
echo ""

echo -e "${BLUE}Current worker status:${NC}"
kubectl get deployment worker -n k8s-learning 2>/dev/null || echo "Worker deployment not found"
echo ""

echo -e "${BLUE}Simulating auto-scaling workflow:${NC}"
echo ""

# Step 1: Show initial state
echo -e "${YELLOW}Step 1: Initial state${NC}"
INITIAL_REPLICAS=$(get_worker_replicas)
echo "• Current worker replicas: $INITIAL_REPLICAS"
echo ""

# Step 2: Simulate high queue load
echo -e "${YELLOW}Step 2: Simulating high queue load scenario${NC}"
echo "• Imagine queue depth: 35 jobs"
echo "• Scale-up threshold: 20 jobs"
echo "• Expected action: Scale UP workers"
echo "• Logic: 35 jobs > 20 threshold → Add workers"
echo ""

# Step 3: Show controller logic
echo -e "${YELLOW}Step 3: Controller scaling logic${NC}"
echo "• Jobs per worker capacity: 10"
echo "• Needed workers: ceil(35 ÷ 10) = 4 workers"
echo "• Current workers: $INITIAL_REPLICAS"
echo "• Scale up by: min(2, needed-current) replicas"
echo ""

# Step 4: Simulate low queue load
echo -e "${YELLOW}Step 4: Simulating low queue load scenario${NC}"
echo "• Imagine queue depth: 2 jobs"
echo "• Scale-down threshold: 5 jobs"
echo "• Expected action: Scale DOWN workers"
echo "• Logic: 2 jobs < 5 threshold → Remove 1 worker"
echo ""

# Step 5: Show constraints
echo -e "${YELLOW}Step 5: Scaling constraints${NC}"
echo "• Minimum replicas: 1 (always keep at least 1 worker)"
echo "• Maximum replicas: 10 (prevent resource exhaustion)"
echo "• Scale up by: max 2 workers at a time (gradual scaling)"
echo "• Scale down by: 1 worker at a time (conservative scaling)"
echo ""

echo -e "${GREEN}📊 Auto-scaling demonstration complete!${NC}"
echo ""
echo -e "${BLUE}To see real auto-scaling in action:${NC}"
echo "1. Deploy the system: make k8s-local"
echo "2. Run controller locally: ./build/text-controller -zap-devel"
echo "3. Generate load: make run-stress-test"
echo "4. Watch scaling: kubectl get deployment worker -n k8s-learning -w"
echo ""
echo -e "${GREEN}Controller will automatically adjust worker replicas based on Redis queue depth!${NC}"