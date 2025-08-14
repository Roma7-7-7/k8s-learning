#!/bin/bash

set -e

echo "Cleaning up Kubernetes resources..."

# Delete all resources in the namespace
kubectl delete namespace k8s-learning --ignore-not-found=true

echo "Cleanup completed!"
echo "All resources in k8s-learning namespace have been removed."