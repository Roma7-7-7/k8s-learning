# Queue-Based Auto-Scaling Controller

## Overview

The Text Processing Controller implements intelligent auto-scaling for worker pods based on Redis queue depth. Unlike traditional Kubernetes HPA (Horizontal Pod Autoscaler) that scales based on CPU/Memory metrics, this controller scales based on actual workload demand.

## How It Works

### Architecture

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ API Service │───▶│ Redis Queue │───▶│ N Workers   │
│             │    │             │    │ (Dynamic)   │
└─────────────┘    └─────▲───────┘    └─────────────┘
                         │                     ▲
                  ┌──────┴──────┐             │
                  │ Controller  │─────────────┘
                  │ Monitors &  │ Scales based on
                  │ Scales      │ queue depth
                  └─────────────┘
```

### Controller Logic

**Monitoring:**
- Queries Redis queue depth every 30 seconds (configurable)
- Tracks both main queue (`text_tasks`) and priority queue (`text_tasks:priority`)
- Ignores failed queue in scaling calculations

**Scaling Decisions:**
```go
if queueDepth > 20 {
    // Scale UP: Add up to 2 workers at a time
    targetReplicas = min(currentReplicas + 2, neededReplicas)
} else if queueDepth < 5 && currentReplicas > 1 {
    // Scale DOWN: Remove 1 worker at a time
    targetReplicas = currentReplicas - 1
} else {
    // No change needed
    targetReplicas = currentReplicas
}
```

**Constraints:**
- **Minimum replicas:** 1 (always keep at least one worker running)
- **Maximum replicas:** 10 (prevent resource exhaustion)
- **Scale-up rate:** Maximum 2 workers added per reconciliation cycle
- **Scale-down rate:** Maximum 1 worker removed per reconciliation cycle

### Scaling Parameters

| Parameter | Value | Description |
|-----------|--------|-------------|
| Scale-up threshold | 20 jobs | When queue depth exceeds this, scale up |
| Scale-down threshold | 5 jobs | When queue depth falls below this, scale down |
| Jobs per worker capacity | 10 jobs | Estimated processing capacity per worker |
| Reconcile interval | 30 seconds | How often controller checks queue depth |
| Min replicas | 1 | Minimum number of workers (never scale to 0) |
| Max replicas | 10 | Maximum number of workers |

## Benefits Over Static Scaling

### Static Workers (Before)
```
✗ Fixed 2 workers running 24/7
✗ High load periods: Queue backs up, slow processing
✗ Low load periods: Workers idle, wasting resources  
✗ No adaptation to workload changes
```

### Auto-Scaled Workers (After)
```
✓ Dynamic worker count based on actual demand
✓ High load: Auto-scale to 8 workers, clear backlog fast
✓ Low load: Scale down to 1 worker, save resources
✓ Responsive to traffic patterns
✓ Resource-efficient
```

## Usage

### Deploy with Auto-Scaling

```bash
# Deploy the complete system including controller
make k8s-local

# The controller is automatically included and will start managing worker scaling
```

### Monitor Auto-Scaling

```bash
# Watch worker deployment scaling in real-time
kubectl get deployment worker -n k8s-learning -w

# View controller logs
kubectl logs -l app=controller -n k8s-learning -f

# Check scaling events
kubectl get events -n k8s-learning | grep worker
```

### Test Auto-Scaling

```bash
# Run auto-scaling demonstration
make test-autoscaling

# Generate load to trigger scaling
make run-stress-test

# Monitor scaling in another terminal
kubectl get deployment worker -n k8s-learning -w
```

### Local Development

Run the controller locally while connecting to minikube cluster:

```bash
# 1. Port-forward Redis for controller access
kubectl port-forward svc/redis-service 6379:6379 -n k8s-learning &

# 2. Set environment variables
export REDIS_HOST=localhost
export REDIS_PORT=6379
export LOG_LEVEL=debug

# 3. Run controller locally with debug logging
./build/text-controller -zap-devel -zap-log-level=debug

# 4. In another terminal, generate load
make run-stress-test

# 5. Watch the controller scale workers based on queue depth
kubectl get deployment worker -n k8s-learning -w
```

## Metrics

The controller exposes Prometheus metrics:

- `textprocessing_queue_depth{queue_name}` - Current queue depth
- `textprocessing_active_workers` - Number of active workers
- `textprocessing_autoscaling_events_total{direction}` - Scaling events
- `textprocessing_current_replicas{job_name}` - Current replica count

## Configuration

Environment variables for controller configuration:

```bash
# Redis connection
REDIS_HOST=redis-service
REDIS_PORT=6379
REDIS_PASSWORD=secret

# Auto-scaling behavior  
RECONCILE_INTERVAL=30s
METRICS_COLLECTION_INTERVAL=15s

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
```

## Real-World Scaling Example

**Scenario:** E-commerce text processing during Black Friday

```
09:00 - Normal load (5 jobs) → 1 worker
10:00 - Traffic increases (15 jobs) → 1 worker (below threshold)
11:00 - Black Friday rush (50 jobs) → Scale up to 5 workers
11:30 - Peak load (100 jobs) → Scale up to 10 workers (max)
12:00 - Load decreasing (30 jobs) → Keep 10 workers (within threshold)
13:00 - Back to normal (8 jobs) → Scale down to 1 worker
```

**Result:** 
- ✅ Fast processing during peak load
- ✅ Resource savings during low load  
- ✅ Automatic adaptation without manual intervention

## Troubleshooting

**Controller not scaling:**
1. Check Redis connectivity: `kubectl logs -l app=controller -n k8s-learning`
2. Verify RBAC permissions: Controller needs `update` on `deployments`
3. Check queue depth: `kubectl exec -it deployment/redis -n k8s-learning -- redis-cli llen text_tasks`

**Unexpected scaling behavior:**
1. Review scaling logic in logs
2. Check if queue depth calculations are correct
3. Verify min/max replica constraints

**Performance issues:**
1. Adjust `RECONCILE_INTERVAL` for faster/slower response
2. Tune scaling thresholds based on your workload
3. Monitor resource utilization of scaled workers

This intelligent auto-scaling transforms your text processing system from static to adaptive, ensuring optimal performance and resource utilization!