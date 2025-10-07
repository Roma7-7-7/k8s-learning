#!/bin/bash
set -e

DASHBOARD_DIR="deployments/base/monitoring/dashboards"
NAMESPACE="monitoring"
CONFIGMAP_NAME="grafana-dashboards"

echo "📊 Deploying Grafana dashboards to namespace: $NAMESPACE"

# Check if dashboard directory exists
if [ ! -d "$DASHBOARD_DIR" ]; then
    echo "❌ Dashboard directory not found: $DASHBOARD_DIR"
    exit 1
fi

# Count dashboard files
DASHBOARD_COUNT=$(find "$DASHBOARD_DIR" -name "*.json" | wc -l | tr -d ' ')
if [ "$DASHBOARD_COUNT" -eq 0 ]; then
    echo "⚠️  No dashboard JSON files found in $DASHBOARD_DIR"
    exit 0
fi

echo "📁 Found $DASHBOARD_COUNT dashboard(s)"

# Delete existing ConfigMap if it exists
kubectl delete configmap "$CONFIGMAP_NAME" -n "$NAMESPACE" --ignore-not-found=true

# Create ConfigMap from dashboard files
kubectl create configmap "$CONFIGMAP_NAME" \
    --from-file="$DASHBOARD_DIR" \
    -n "$NAMESPACE"

echo "✅ Dashboards deployed successfully"
echo ""
echo "Dashboards will be available in Grafana after a few seconds."
echo "Access Grafana at http://localhost:3000 (use 'make k8s-forward')"
