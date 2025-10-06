# Text Processing Task Queue

A cloud-native text processing system built with Go and deployed on Kubernetes. The system accepts text file uploads, queues processing tasks, and handles them asynchronously using distributed workers with intelligent auto-scaling.

## Quick Start

### Prerequisites
- Go 1.21+
- Docker or compatible container runtime
- minikube or local Kubernetes cluster
- kubectl and kustomize

### Local Development

```bash
# Setup environment
make setup-dev
# Edit .env with your database and Redis settings

# Build and run services
make run-api       # Start API server
make run-worker    # Start worker
```

### Kubernetes Deployment

```bash
# Complete local deployment
make k8s-local

# Access the API
make k8s-port-forward
# API: http://localhost:8080
# Web UI: http://localhost/ (with minikube tunnel)

# Check status
make k8s-status

# Clean up
make k8s-delete
```

## Architecture

The system consists of five main components:

- **API Service**: REST API for job submission and status (`text-api`)
- **Worker Service**: Background job processor with auto-scaling (`text-worker`)
- **Controller Service**: Queue-based auto-scaler (`text-controller`)
- **Web UI**: Static interface for job management (`text-ui`)
- **Infrastructure**: PostgreSQL database and Redis queue

### Processing Types

- **wordcount** - Count words in text
- **linecount** - Count lines in text
- **uppercase/lowercase** - Case conversion
- **replace** - Find and replace patterns
- **extract** - Extract lines by pattern

## API Endpoints

- `POST /api/v1/jobs` - Submit job with file upload
- `GET /api/v1/jobs/{id}` - Get job status
- `GET /api/v1/jobs` - List jobs
- `GET /api/v1/jobs/{id}/result` - Download result
- `GET /health` - Health check
- `GET /ready` - Readiness probe
- `GET /stats` - Queue statistics

## Development Commands

```bash
# Core workflow
make all           # Format, lint, test, and build
make build         # Build all binaries
make test          # Run tests
make lint          # Run linter

# Kubernetes
make k8s-build     # Build Docker images
make k8s-deploy    # Deploy to K8s
make k8s-logs      # View logs

# Testing
make test-coverage      # Coverage report
make run-stress-test    # Load testing
make test-autoscaling   # Auto-scaling demo
```

## Configuration

See `.env.distro` for configuration template. All services use environment variables:

**Required:**
- Database: `DB_HOST`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`
- Redis: `REDIS_HOST`
- Storage: `UPLOAD_DIR`, `RESULT_DIR`

**Optional:**
- Server: `PORT`, `HOST`, timeouts
- Logging: `LOG_LEVEL`, `LOG_FORMAT`
- Auto-scaling: `RECONCILE_INTERVAL`

## Documentation

- [STATUS.md](STATUS.md) - Implementation status and roadmap
- [CLAUDE.md](CLAUDE.md) - Development guidelines and standards
- [docs/AUTO_SCALING.md](docs/AUTO_SCALING.md) - Auto-scaling architecture
- [api-tests.http](api-tests.http) - API testing suite for JetBrains IDEs

## Project Structure

```
k8s-learning/
├── cmd/                    # Service entrypoints
│   ├── api/
│   ├── worker/
│   ├── controller/
│   └── stress-test/
├── internal/               # Internal packages
│   ├── api/
│   ├── worker/
│   ├── controller/
│   └── storage/
├── deployments/            # K8s manifests (kustomize)
├── docker/                 # Dockerfiles
├── web/                    # Web UI
├── migrations/             # Database migrations
├── scripts/                # Build and deploy scripts
└── test-files/             # Test data
```

## Testing

The project includes comprehensive testing tools:

**HTTP Client Tests:**
Open `api-tests.http` in JetBrains IDEs (IntelliJ, GoLand) and run requests with the ▶️ button.

**Stress Testing:**
```bash
# Default test (30s, 2 workers)
make run-stress-test

# Custom parameters
./build/stress-test --file test-files/sample.txt \
  --duration 60 --concurrency 5 \
  --min-process-delay 1000 --max-process-delay 5000
```

**Auto-Scaling Demo:**
```bash
make test-autoscaling
# Watch workers scale in real-time
kubectl get deployment worker -n k8s-learning -w
```

## License

This is a learning project for Kubernetes and Go development.
