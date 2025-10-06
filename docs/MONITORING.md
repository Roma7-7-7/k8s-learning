# Monitoring Setup

This project uses Prometheus for metrics collection and Grafana for visualization.

## Architecture

- **Prometheus**: Collects metrics from API service, Kubernetes components, and nodes
- **Grafana**: Provides dashboards for visualizing metrics
- **Metrics Endpoint**: Each service exposes `/metrics` endpoint for Prometheus scraping

## Deployment

The monitoring stack is deployed in the `monitoring` namespace:

```bash
# Deploy monitoring stack
kubectl apply -f deployments/base/monitoring/monitoring.yaml

# Check status
kubectl get pods -n monitoring
```

## Accessing Monitoring Tools

### Grafana

Grafana is exposed via NodePort on port 30300:

```bash
# Easiest method - use Makefile
make grafana

# Alternative: Get Grafana URL (minikube)
minikube service grafana -n monitoring --url

# Alternative: Manual port-forward
kubectl port-forward -n monitoring svc/grafana 3000:3000
```

**Default Credentials:**
- Username: `admin`
- Password: `admin`

Access at: http://localhost:3000

### Prometheus

Prometheus is accessible via ClusterIP:

```bash
# Easiest method - use Makefile
make prometheus

# Alternative: Manual port-forward
kubectl port-forward -n monitoring svc/prometheus 9090:9090
```

Access at: http://localhost:9090

### Monitoring Status

Check the status of the monitoring stack:

```bash
# Using Makefile
make monitoring-status

# Or manually
kubectl get pods -n monitoring
kubectl get svc -n monitoring
```

## Available Metrics

### API Service Metrics

The API service exposes the following custom metrics:

#### HTTP Metrics
- `http_requests_total` - Total number of HTTP requests (labels: method, path, status)
- `http_request_duration_seconds` - HTTP request duration histogram (labels: method, path)
- `http_request_size_bytes` - HTTP request size histogram (labels: method, path)
- `http_response_size_bytes` - HTTP response size histogram (labels: method, path)

#### Job Metrics
- `jobs_created_total` - Total number of jobs created
- `jobs_queued_total` - Total number of jobs queued (labels: priority)

#### Database Metrics
- `db_connections_active` - Number of active database connections
- `db_queries_total` - Total number of database queries (labels: operation)
- `db_query_duration_seconds` - Database query duration histogram (labels: operation)

#### Redis Metrics
- `redis_operations_total` - Total number of Redis operations (labels: operation)
- `redis_operation_duration_seconds` - Redis operation duration histogram (labels: operation)

### Kubernetes Metrics

Prometheus also scrapes:
- **API Server**: Kubernetes API server metrics
- **Nodes**: Node-level metrics (CPU, memory, disk, network)
- **Pods**: Pod-level metrics from all services

## Prometheus Configuration

Prometheus is configured to scrape:

1. **API Service**: Discovers endpoints via Kubernetes service discovery
   - Service name: `api` in `k8s-learning` namespace
   - Scrape interval: 15s

2. **Kubernetes Components**:
   - API Server
   - Nodes (via proxy)
   - Pods with annotation `prometheus.io/scrape: "true"`

3. **Prometheus Self-monitoring**

### Pod Annotations for Scraping

To enable Prometheus scraping for a pod, add these annotations:

```yaml
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "8080"
    prometheus.io/path: "/metrics"
```

## Creating Grafana Dashboards

### 1. Access Grafana

Navigate to http://localhost:3000 and login with admin/admin.

### 2. Import Pre-built Dashboards

For Kubernetes metrics, you can import community dashboards:

1. Click **"+"** → **"Import"**
2. Enter dashboard ID:
   - **315**: Kubernetes cluster monitoring
   - **8588**: Kubernetes deployment metrics
   - **6417**: Kubernetes cluster overview

### 3. Create Custom API Dashboard

Create a new dashboard for API service metrics:

#### Panel Examples:

**HTTP Request Rate**
```promql
rate(http_requests_total{service="api"}[5m])
```

**HTTP Request Duration (p95)**
```promql
histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, method, path))
```

**Jobs Created Rate**
```promql
rate(jobs_created_total[5m])
```

**API Service Availability**
```promql
up{job="api"}
```

**HTTP Error Rate**
```promql
rate(http_requests_total{status=~"5.."}[5m])
```

### 4. Sample Dashboard JSON

A sample dashboard configuration will be added to `deployments/base/monitoring/dashboards/` for easy import.

## Querying Metrics

### PromQL Examples

**Top 5 slowest endpoints:**
```promql
topk(5,
  histogram_quantile(0.95,
    sum(rate(http_request_duration_seconds_bucket[5m])) by (le, path)
  )
)
```

**Request count by status code:**
```promql
sum(rate(http_requests_total[5m])) by (status)
```

**Pod CPU usage:**
```promql
sum(rate(container_cpu_usage_seconds_total{namespace="k8s-learning"}[5m])) by (pod)
```

**Pod Memory usage:**
```promql
sum(container_memory_working_set_bytes{namespace="k8s-learning"}) by (pod)
```

## Troubleshooting

### Prometheus Not Scraping API

1. Check API service is running:
   ```bash
   kubectl get pods -n k8s-learning -l app=api
   ```

2. Verify metrics endpoint is accessible:
   ```bash
   kubectl port-forward -n k8s-learning svc/api 8080:8080
   curl http://localhost:8080/metrics
   ```

3. Check Prometheus targets:
   - Go to Prometheus UI → Status → Targets
   - Look for `api` job and check status

4. Verify service discovery:
   - Go to Prometheus UI → Status → Service Discovery
   - Check if `api` endpoints are discovered

### Grafana Data Source Not Working

1. Check Prometheus service is accessible from Grafana:
   ```bash
   kubectl exec -n monitoring deployment/grafana -- wget -O- http://prometheus:9090/-/healthy
   ```

2. Verify data source configuration in Grafana:
   - Configuration → Data Sources → Prometheus
   - URL should be: `http://prometheus:9090`

### Missing Kubernetes Metrics

1. Check Prometheus RBAC permissions:
   ```bash
   kubectl get clusterrolebinding prometheus -o yaml
   ```

2. Check Prometheus logs:
   ```bash
   kubectl logs -n monitoring deployment/prometheus
   ```

## Cleanup

To remove the monitoring stack:

```bash
kubectl delete -f deployments/base/monitoring/monitoring.yaml
```

## Future Enhancements

- [ ] Add persistent storage for Prometheus (currently uses emptyDir)
- [ ] Add persistent storage for Grafana dashboards
- [ ] Add alerting rules via Alertmanager
- [ ] Add metrics for Worker service
- [ ] Add metrics for Controller service
- [ ] Create pre-configured dashboards via ConfigMap
- [ ] Add Grafana dashboard provisioning
- [ ] Set up long-term metrics storage (Thanos/Cortex)
- [ ] Add application-level SLIs/SLOs
