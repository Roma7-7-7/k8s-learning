#!/bin/bash

set -e

# Accept optional image tag parameter
IMAGE_TAG=${1:-latest}

echo "Deploying to local Kubernetes (image tag: ${IMAGE_TAG})..."

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "kubectl not found. Please install kubectl first."
    exit 1
fi

# Check if kustomize is available
if ! command -v kustomize &> /dev/null; then
    echo "kustomize not found. Please install kustomize first."
    exit 1
fi

# Build and deploy using kustomize with image tag override
echo "Applying Kubernetes manifests..."
kustomize build deployments/overlays/development | \
    sed "s|k8s-learning/api:latest|k8s-learning/api:${IMAGE_TAG}|g" | \
    sed "s|k8s-learning/worker:latest|k8s-learning/worker:${IMAGE_TAG}|g" | \
    sed "s|k8s-learning/controller:latest|k8s-learning/controller:${IMAGE_TAG}|g" | \
    sed "s|k8s-learning/web:latest|k8s-learning/web:${IMAGE_TAG}|g" | \
    kubectl apply -f -

echo "Waiting for deployments to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/postgres -n k8s-learning
kubectl wait --for=condition=available --timeout=300s deployment/redis -n k8s-learning

echo "Database and Redis are ready. Waiting for API and Worker..."
kubectl wait --for=condition=available --timeout=300s deployment/api -n k8s-learning || echo "API deployment may need more time to be ready"
kubectl wait --for=condition=available --timeout=300s deployment/worker -n k8s-learning || echo "Worker deployment may need more time to be ready"
kubectl wait --for=condition=available --timeout=300s deployment/web -n k8s-learning || echo "Web deployment may need more time to be ready"
kubectl wait --for=condition=available --timeout=300s deployment/controller -n k8s-learning || echo "Controller deployment may need more time to be ready"

echo "Deployment completed!"
echo ""
echo "To check status:"
echo "  kubectl get pods -n k8s-learning"
echo ""
echo "To access the API locally:"
echo "  kubectl port-forward svc/api 8080:8080 -n k8s-learning"
echo ""
echo "To view logs:"
echo "  kubectl logs -l app=api -n k8s-learning"
echo "  kubectl logs -l app=worker -n k8s-learning"
echo "  kubectl logs -l app=controller -n k8s-learning"
