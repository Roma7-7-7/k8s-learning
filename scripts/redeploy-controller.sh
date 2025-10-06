#!/bin/bash

# DEPRECATED: Use 'make k8s-redeploy SERVICE=controller' instead
# This script is kept for backwards compatibility

set -e

echo "⚠️  DEPRECATED: This script is deprecated"
echo "Use: make k8s-redeploy SERVICE=controller"
echo ""
echo "🚀 Quick Controller Redeploy"
echo "=========================="

# Build controller binary
echo "📦 Building controller binary..."
make build-controller

# Build Docker image
echo "🐳 Building controller Docker image..."
docker build -f docker/Dockerfile.controller -t k8s-learning/controller:dev .

# Load image into minikube
echo "📤 Loading image into minikube..."
minikube image load k8s-learning/controller:dev

# Delete existing controller pod to force restart
echo "🔄 Restarting controller..."
kubectl delete pod -l app=controller -n k8s-learning --ignore-not-found=true

# Wait for new pod to be ready
echo "⏳ Waiting for controller to be ready..."
kubectl wait --for=condition=ready pod -l app=controller -n k8s-learning --timeout=60s

echo "✅ Controller redeploy complete!"
echo ""
echo "📊 Controller status:"
kubectl get pods -l app=controller -n k8s-learning

echo ""
echo "📝 To view logs:"
echo "kubectl logs -l app=controller -n k8s-learning -f"