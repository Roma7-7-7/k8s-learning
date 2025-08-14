#!/bin/bash

set -e

echo "Building Docker images for local Kubernetes deployment..."

# Build API image
echo "Building API image..."
docker build -f docker/Dockerfile.api -t k8s-learning/api:latest -t k8s-learning/api:dev .

# Build Worker image
echo "Building Worker image..."
docker build -f docker/Dockerfile.worker -t k8s-learning/worker:latest -t k8s-learning/worker:dev .

# Build Web image
echo "Building Web image..."
docker build -f docker/Dockerfile.web -t k8s-learning/web:latest -t k8s-learning/web:dev .

echo "Docker images built successfully!"
echo "Images:"
echo "  k8s-learning/api:latest"
echo "  k8s-learning/api:dev"  
echo "  k8s-learning/worker:latest"
echo "  k8s-learning/worker:dev"
echo "  k8s-learning/web:latest"
echo "  k8s-learning/web:dev"