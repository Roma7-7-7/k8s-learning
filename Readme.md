# Text Processing Task Queue - Project Specification

## Project Overview
A cloud-native text processing system built with Go and deployed on Kubernetes. The system accepts text file upload requests, queues processing tasks, and handles them asynchronously using distributed workers.

## Architecture Components

### 1. API Service (`text-api`)
**Language**: Go  
**Type**: REST API  
**Purpose**: Handle user requests, manage job metadata, interface with queue

**Key Features**:
- HTTP endpoints for job submission and status checking
- Text file upload handling with validation
- Job metadata storage in PostgreSQL
- Redis queue integration for task publishing

**Dependencies**:
- PostgreSQL (job metadata)
- Redis (task queue)
- File storage (text files)

**Kubernetes Resources**:
- Deployment
- Service (ClusterIP)
- ConfigMap (API configuration)
- Secret (database credentials, Redis password)
- ServiceAccount with minimal RBAC

### 2. Worker Service (`text-worker`)
**Language**: Go  
**Type**: Background job processor  
**Purpose**: Poll queue for tasks, process text files, update job status

**Key Features**:
- Redis queue consumer
- Text processing (word count, line count, format conversion, find/replace)
- Result storage to file system
- Status updates to PostgreSQL
- Graceful shutdown handling

**Dependencies**:
- Redis (task queue)
- PostgreSQL (status updates)
- File storage (text files)

**Kubernetes Resources**:
- Deployment (horizontal scaling)
- ConfigMap (worker configuration)
- Secret (credentials)
- ServiceAccount with minimal RBAC

### 3. Task Controller (`text-controller`)
**Language**: Go using controller-runtime  
**Type**: Kubernetes controller  
**Purpose**: Watch custom resources, manage Kubernetes Jobs, auto-scale workers

**Key Features**:
- Custom Resource Definition (CRD) for TextProcessingJob
- Controller pattern with reconciliation loops
- Automatic worker scaling based on queue depth
- Job lifecycle management
- Metrics collection and reporting

**Dependencies**:
- Kubernetes API
- Redis (queue monitoring)
- Custom Resource Definitions

**Kubernetes Resources**:
- Deployment
- CustomResourceDefinition (ImageProcessingJob)
- ServiceAccount with Job management RBAC
- ClusterRole and ClusterRoleBinding

### 4. Web UI (`text-ui`)
**Language**: HTML/CSS/JavaScript 
**Type**: Static web application  
**Purpose**: User interface for job submission and monitoring

**Key Features**:
- Text file upload form
- Job status dashboard
- Result download links
- Simple responsive design

**Dependencies**:
- API Service

**Kubernetes Resources**:
- Deployment
- Service (ClusterIP)

### 5. Database (`postgresql`)
**Type**: PostgreSQL StatefulSet  
**Purpose**: Persistent storage for job metadata and status

**Schema**:
```sql
-- Jobs table
CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_filename VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    processing_type VARCHAR(100) NOT NULL,
    parameters JSONB,
    status VARCHAR(50) DEFAULT 'pending',
    delay_ms INTEGER DEFAULT 0,
    result_path VARCHAR(500),
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    worker_id VARCHAR(255)
);

-- Index for efficient queries
CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_created_at ON jobs(created_at);
CREATE INDEX idx_jobs_delay_ms ON jobs(delay_ms);
```

**Kubernetes Resources**:
- StatefulSet
- Service (ClusterIP)
- PersistentVolumeClaim
- Secret (database credentials)
- ConfigMap (PostgreSQL configuration)

### 6. Redis Queue (`redis`)
**Type**: Redis StatefulSet  
**Purpose**: Task queue and caching

**Queue Structure**:
- `text_tasks` - Main processing queue
- `text_tasks:priority` - High priority tasks
- `text_tasks:failed` - Failed task retry queue
- `workers:heartbeat` - Worker health tracking

**Kubernetes Resources**:
- StatefulSet
- Service (ClusterIP)
- PersistentVolumeClaim
- Secret (Redis password)
- ConfigMap (Redis configuration)

## Directory Structure

```
text-processing-queue/
├── cmd/
│   ├── api/
│   │   └── main.go                 # API server entrypoint
│   ├── worker/
│   │   └── main.go                 # Worker entrypoint
│   ├── controller/
│   │   └── main.go                 # Controller entrypoint
│   └── stress-test/
│       └── main.go                 # Stress testing tool
├── internal/
│   ├── api/
│   │   ├── handlers/               # HTTP handlers
│   │   ├── middleware/             # Authentication, logging
│   │   └── server.go               # Server setup
│   ├── worker/
│   │   ├── processors/             # Text processing logic
│   │   ├── queue/                  # Queue consumer
│   │   └── worker.go               # Worker orchestration
│   ├── controller/
│   │   ├── textprocessingjob/      # CRD controller
│   │   └── metrics/                # Metrics collection
│   ├── storage/
│   │   ├── database/               # PostgreSQL operations
│   │   ├── queue/                  # Redis operations
│   │   └── filestore/              # File storage operations
├── deployments/
│   ├── base/
│   │   ├── api/                    # API Kubernetes manifests
│   │   ├── worker/                 # Worker Kubernetes manifests
│   │   ├── controller/             # Controller Kubernetes manifests
│   │   ├── database/               # PostgreSQL manifests
│   │   ├── redis/                  # Redis manifests
│   │   └── ui/                     # UI manifests
│   └── overlays/
│       ├── development/            # Dev environment configs
│       └── production/             # Prod environment configs
├── web/
│   ├── static/                     # HTML, CSS, JS files
│   └── nginx.conf                  # Nginx configuration
├── scripts/
│   ├── setup-cluster.sh            # Local development setup
│   ├── build-images.sh             # Docker image building
│   └── deploy.sh                   # Deployment automation
├── test-files/
│   └── sample.txt                  # Sample files for testing
├── docs/
│   ├── api.md                      # API documentation
│   ├── deployment.md               # Deployment guide
│   └── development.md              # Development guide
├── docker/
│   ├── api.Dockerfile              # API service image
│   ├── worker.Dockerfile           # Worker service image
│   ├── controller.Dockerfile       # Controller image
│   └── ui.Dockerfile               # UI service image
├── go.mod
├── go.sum
├── Makefile                        # Build and deployment tasks
└── README.md                       # Project overview and setup
```

## Processing Types

### Supported Operations (Simple Text Processing)
1. **Word Count** - Count total words in text
2. **Line Count** - Count total lines in text
3. **Case Conversion** - Convert to uppercase/lowercase
4. **Find & Replace** - Replace text patterns
5. **Text Extract** - Extract lines containing specific patterns
6. **Format Convert** - Convert between text formats (CSV to JSON, etc.)

### Processing Parameters (Simple JSON)
```json
{
  "wordcount": {},
  "linecount": {},
  "uppercase": {},
  "lowercase": {},
  "replace": {
    "find": "old_text",
    "replace_with": "new_text"
  },
  "extract": {
    "pattern": "ERROR",
    "case_sensitive": false
  }
}
```

## Stress Testing

The system includes comprehensive stress testing capabilities for performance analysis and load testing.

### Built-in Stress Test Tool

A dedicated stress testing tool (`cmd/stress-test`) provides automated load testing with configurable parameters:

**Features:**
- Concurrent request generation with configurable worker count
- Randomized processing delays to simulate varying workloads
- Comprehensive metrics collection (latency, throughput, error rates)
- File upload simulation with real multipart form data
- Test duration control and graceful termination

**Quick Usage:**
```bash
# Build and run with default parameters (30s duration, 2 concurrent workers)
make run-stress-test

# Build the stress test tool only
make build-stress-test

# Run with custom parameters
./build/stress-test --file test-files/sample.txt \
  --duration 60 \
  --concurrency 5 \
  --min-process-delay 1000 \
  --max-process-delay 5000 \
  --query-delay 50
```

**Command Line Options:**
- `--file` - Path to test file (required)
- `--min-process-delay` - Min processing delay in ms (default: 0)
- `--max-process-delay` - Max processing delay in ms (default: 30000)
- `--concurrency` - Number of concurrent requests (default: 1)
- `--query-delay` - Delay between requests in ms (default: 10)
- `--duration` - Test duration in seconds (default: 60)
- `--api-endpoint` - API endpoint URL (default: "http://localhost:8080/api/v1/jobs")

**Metrics Provided:**
- Total requests sent and success/failure rates
- Average, minimum, and maximum response latency
- Requests per second throughput
- Error breakdown by HTTP status code
- Test configuration summary

### API Delay Parameter Support

For granular performance testing, the API supports an optional delay parameter in job submissions:

- **delay_ms**: Optional integer parameter (0-60000) that adds artificial processing delay
- Useful for simulating longer-running tasks and testing queue behavior under load
- Applied at the worker level before actual text processing begins

**Usage Examples:**
```bash
# Standard job submission (no delay)
curl -X POST "http://localhost:8080/api/v1/jobs" \
  -F "file=@sample.txt" \
  -F "processing_type=wordcount"

# Job with 5-second delay for stress testing
curl -X POST "http://localhost:8080/api/v1/jobs" \
  -F "file=@sample.txt" \
  -F "processing_type=wordcount" \
  -F "delay_ms=5000"
```

### Stress Testing Scenarios

**Queue Depth Testing:**
```bash
# High concurrency with delays to build queue backlog
./build/stress-test --concurrency 10 --min-process-delay 5000 --max-process-delay 10000 --duration 120
```

**Throughput Testing:**
```bash
# Maximum throughput with minimal delays
./build/stress-test --concurrency 20 --max-process-delay 100 --query-delay 1 --duration 60
```

**Latency Analysis:**
```bash
# Low concurrency for baseline latency measurement
./build/stress-test --concurrency 1 --max-process-delay 1000 --duration 300
```


## Implementation Status

### ✅ Completed Components

#### **Web UI (`text-ui`)**
Static web interface with:
- Simple HTML/CSS/JavaScript implementation
- File upload form for job submission
- Job status monitoring and results display
- Integration with API endpoints
- Nginx-based serving with proper routing
- Kubernetes deployment with ClusterIP service

#### **Ingress Configuration**
External access routing with:
- Nginx ingress controller support
- Path-based routing (`/api` → API service, `/` → Web UI)
- Proper route separation for API and UI traffic
- Local development via `minikube tunnel`

#### **API Service (`text-api`)**
Full REST API implementation with:
- Job submission and status endpoints (`POST /api/v1/jobs`, `GET /api/v1/jobs/{id}`)
- File upload handling with validation
- PostgreSQL integration with golang-migrate
- Redis queue integration
- Health checks (`/health`, `/ready`, `/stats`)
- Structured logging with slog
- Environment-based configuration with envconfig
- Automatic database migrations on startup
- Comprehensive middleware (CORS, security, logging)
- Graceful shutdown handling
- Configurable migration paths via `DB_MIGRATIONS_URL`

#### **Worker Service (`text-worker`)**
Background job processor with:
- Redis queue consumer implementation
- Text processing capabilities (word count, line count, case conversion, find/replace, extract)
- Configurable processing delays for stress testing (0-60 second delay support)
- Database status updates
- File processing and result storage
- Worker heartbeat and health tracking
- Graceful shutdown handling
- Concurrent job processing

#### **Kubernetes Infrastructure**
Complete local deployment setup with:
- **PostgreSQL**: Database deployment with automatic `textprocessing` database creation
- **Redis**: Message queue and caching deployment
- **API Service**: Kubernetes deployment with 2 replicas, health checks, and init containers
- **Worker Service**: Kubernetes deployment with 2 replicas and shared storage
- **Storage**: Persistent volumes for database, Redis, uploads, and results
- **Configuration**: ConfigMaps and Secrets for environment-specific settings
- **Networking**: Internal ClusterIP services with port-forwarding for external access
- **Kustomize**: Environment-specific overlays (development/production)

#### **Docker Images**
Multi-stage Docker builds with:
- **API Image**: Optimized Go binary with migration files and non-root user
- **Worker Image**: Lightweight Alpine-based image with text processing capabilities
- **Security**: Non-root user, minimal base images, proper file permissions
- **Build Scripts**: Automated image building and tagging

#### **Development Tooling**
- **Makefile**: Comprehensive build, test, and deployment targets
- **Scripts**: Automated build (`build-images.sh`), deploy (`deploy-local.sh`), cleanup (`cleanup.sh`)
- **HTTP Client**: Complete API testing suite for JetBrains IDEs (`api-tests.http`)
- **Configuration Management**: Environment variables with sensible defaults
- **Database Migrations**: golang-migrate integration with automatic execution

### 🚧 In Progress
- **Controller (`text-controller`)**: Kubernetes controller for custom resources and auto-scaling

### 📋 Pending
- **Production Configuration**: Helm charts for production deployments
- **CI/CD Pipeline**: Automated testing and deployment
- **Monitoring**: Prometheus metrics and Grafana dashboards
- **Observability**: Distributed tracing and structured logging aggregation
- **Security**: RBAC, network policies, and security scanning
- **High Availability**: Multi-zone deployments and backup strategies

## API Endpoints

### Job Management
- `POST /api/v1/jobs` - Submit text processing job with file upload (supports optional `delay_ms` parameter for stress testing)
- `GET /api/v1/jobs/{id}` - Get job status and details
- `GET /api/v1/jobs` - List jobs with pagination
- `GET /api/v1/jobs/{id}/result` - Download processed result file

### Health & Monitoring
- `GET /health` - Basic health check
- `GET /ready` - Readiness probe (checks DB and Redis connectivity)
- `GET /stats` - Queue statistics and job counts

### Supported Processing Types
- `wordcount` - Count total words in text
- `linecount` - Count total lines in text
- `uppercase` - Convert text to uppercase
- `lowercase` - Convert text to lowercase
- `replace` - Find and replace text patterns
- `extract` - Extract lines containing specific patterns

## Configuration

The application uses environment variables for configuration with production-oriented defaults. **Required** configuration values must be set before deployment.

### Server Configuration
- `PORT=8080` - HTTP server port
- `HOST=0.0.0.0` - HTTP server bind address
- `READ_TIMEOUT=10s` - HTTP read timeout
- `WRITE_TIMEOUT=10s` - HTTP write timeout
- `IDLE_TIMEOUT=120s` - HTTP idle timeout
- `SHUTDOWN_TIMEOUT=30s` - Graceful shutdown timeout

### Database Configuration (**Required Values**)
- `DB_HOST` - **REQUIRED** PostgreSQL host
- `DB_PORT=5432` - PostgreSQL port
- `DB_USER` - **REQUIRED** PostgreSQL username
- `DB_PASSWORD` - **REQUIRED** PostgreSQL password
- `DB_NAME` - **REQUIRED** PostgreSQL database name
- `DB_SSL_MODE=require` - PostgreSQL SSL mode (require, verify-ca, verify-full, disable)
- `DB_MAX_CONNS=20` - Maximum database connections
- `DB_MAX_IDLE=10` - Maximum idle database connections
- `DB_MIGRATIONS_URL=file://migrations` - Migration files URL path

### Redis Configuration (**Required Values**)
- `REDIS_HOST` - **REQUIRED** Redis host
- `REDIS_PORT=6379` - Redis port
- `REDIS_PASSWORD` - Redis password (optional but recommended)
- `REDIS_DB=0` - Redis database number

### Storage Configuration (**Required Values**)
- `UPLOAD_DIR` - **REQUIRED** Directory for uploaded files
- `RESULT_DIR` - **REQUIRED** Directory for processed results
- `MAX_FILE_SIZE=10485760` - Maximum file size in bytes (10MB)

### Logging Configuration
- `LOG_LEVEL=info` - Log level (debug, info, warn, error)
- `LOG_FORMAT=json` - Log format (json, text)

### Local Development Setup

For local development, you can use a `.env` file:

**Quick Setup:**
```bash
make setup-dev
```

**Manual Setup:**
1. **Copy the template**: `cp .env.distro .env`
2. **Edit the values**: Update `.env` with your local database and Redis settings
3. **Create directories**: `mkdir -p uploads results`
4. **Run the service**: `make run-api`

The application will automatically load `.env` if present.

**Important**: Never commit `.env` files to version control (already in `.gitignore`)

### Production Deployment Notes
- **SSL/TLS**: Database connections use SSL by default (`DB_SSL_MODE=require`)
- **Secrets**: All sensitive values (passwords, hosts) must be explicitly configured
- **Storage**: File storage paths must be explicitly set to prevent default directory usage  
- **Timeouts**: Conservative HTTP timeouts (10s) prevent resource exhaustion
- **Environment Variables**: Use Kubernetes Secrets/ConfigMaps instead of .env files

## Database Migrations

Migrations are stored as SQL files in the `migrations/` directory using golang-migrate naming convention:
```
000001_description.up.sql    # Forward migration
000001_description.down.sql  # Rollback migration
```

### Migration Workflow
1. Create new migration files: `migrations/000002_add_user_table.up.sql` and `migrations/000002_add_user_table.down.sql`
2. Write forward migration in `.up.sql` and rollback in `.down.sql`
3. Restart the API service to apply pending migrations automatically
4. Migrations are tracked in the `schema_migrations` table by golang-migrate

### Important Notes
- Migrations run automatically when the API service starts using golang-migrate library
- Only the API service accesses the database directly
- Full rollback support with `.down.sql` files
- Migration history is preserved and managed by golang-migrate

## Graceful Shutdown

The API service implements comprehensive graceful shutdown to handle Kubernetes pod termination:

### Signal Handling
- `SIGTERM`: Standard Kubernetes pod termination signal
- `SIGINT`: Local development interrupt (Ctrl+C)
- `SIGQUIT`: Emergency shutdown signal

### Shutdown Sequence
1. **Signal Reception**: Server receives termination signal and sets shutdown flag
2. **Traffic Rejection**: New non-health-check requests are rejected with 503 status
3. **Readiness Failure**: `/ready` endpoint returns 503 to stop Kubernetes traffic routing
4. **Connection Draining**: Existing HTTP connections are gracefully closed
5. **Resource Cleanup**: Database and Redis connections are closed
6. **Complete Shutdown**: Process exits cleanly

### Configuration
- `SHUTDOWN_TIMEOUT`: Maximum time to wait for graceful shutdown (default: 30s)
- Health checks (`/health`, `/ready`) remain available during shutdown
- Kubernetes should configure `terminationGracePeriodSeconds` >= `SHUTDOWN_TIMEOUT`

## API Testing with JetBrains IDEs

The project includes a comprehensive HTTP client file for testing all API endpoints directly from JetBrains IDEs (IntelliJ IDEA, GoLand, WebStorm, etc.).

### HTTP Client File: `api-tests.http`

This file contains pre-configured requests for all API endpoints with:
- **Health checks**: `/health`, `/ready`, `/stats`
- **Job management**: Create, list, get details, download results
- **All processing types**: wordcount, linecount, uppercase, lowercase, replace, extract
- **Error scenarios**: Invalid parameters, missing files, etc.
- **Complete workflows**: End-to-end testing scenarios

### Usage in JetBrains IDEs

1. **Open the file**: Navigate to `api-tests.http` in your IDE
2. **Start the API server**: Ensure your API is running on `localhost:8080`
3. **Run requests**: Click the ▶️ button next to any request to execute it
4. **File uploads**: The requests include embedded test content for file uploads
5. **Environment selection**: Choose "development" environment in the HTTP client toolbar
6. **Override variables**: Use environment files to customize `@baseUrl` and `@sampleJobId`

### Key Features

- **Multipart file uploads**: Properly formatted requests with embedded test files
- **All processing types**: Examples for each text processing operation
- **Parameter examples**: Correct JSON formatting for complex operations
- **Testing scenarios**: Comprehensive error cases and workflows
- **Sample data**: Pre-defined test content for each processing type

### Test Files

- `sample-files/test-file.txt`: Sample text file for manual testing
- Embedded content in HTTP requests for automated testing

### Example Usage

```http
### Create a word count job
POST {{baseUrl}}/api/v1/jobs
Content-Type: multipart/form-data; boundary=WebAppBoundary

--WebAppBoundary
Content-Disposition: form-data; name="file"; filename="sample.txt"
Content-Type: text/plain

Your test content here

--WebAppBoundary
Content-Disposition: form-data; name="processing_type"

wordcount
--WebAppBoundary--

### Create a job with processing delay for stress testing
POST {{baseUrl}}/api/v1/jobs
Content-Type: multipart/form-data; boundary=WebAppBoundary

--WebAppBoundary
Content-Disposition: form-data; name="file"; filename="sample.txt"
Content-Type: text/plain

Your test content here

--WebAppBoundary
Content-Disposition: form-data; name="processing_type"

wordcount
--WebAppBoundary
Content-Disposition: form-data; name="delay_ms"

5000
--WebAppBoundary--
```

The HTTP client provides a complete testing environment without requiring external tools like Postman or curl.

### Environment Variable Management

#### 1. Using Environment Files (Recommended)

**Shared Configuration** (`http-client.env.json`):
```json
{
  "development": {
    "baseUrl": "http://localhost:8080",
    "sampleJobId": "replace-with-actual-job-id"
  },
  "production": {
    "baseUrl": "https://your-api.com",
    "sampleJobId": "prod-job-id"
  }
}
```

**Private Overrides** (`http-client.private.env.json`):
```json
{
  "development": {
    "sampleJobId": "123e4567-e89b-12d3-a456-426614174000"
  }
}
```

#### 2. Alternative Methods

**Option A: Direct Editing** - Temporarily edit variables in `api-tests.http`:
```http
@baseUrl = http://localhost:8080
@sampleJobId = your-actual-job-id-here
```

**Option B: Environment Selection Dropdown** - Use the environment dropdown in the HTTP client toolbar to switch between `development`, `production`, etc.

**Option C: Run Configuration Variables** - Set variables in the IDE's run configuration dialog.

#### 3. Variable Override Priority
1. **http-client.private.env.json** (highest priority, git-ignored)
2. **http-client.env.json** (shared configuration)
3. **Inline variables** in `.http` file (lowest priority)

#### 4. Workflow Example
1. Create a job: `POST /api/v1/jobs` → Copy job ID from response
2. Update `http-client.private.env.json` with the real job ID
3. Test job endpoints: `GET /api/v1/jobs/{id}` and `GET /api/v1/jobs/{id}/result`

## Build Commands

### Development
- `make all` - Format, test, and build all components
- `make fmt` - Format Go code with gofmt
- `make test` - Run all tests
- `make lint` - Run linter (requires golangci-lint)
- `make deps` - Download and tidy dependencies

### Building
- `make build` - Build all binaries (api, worker, controller, stress-test)
- `make build-api` - Build API service only
- `make build-worker` - Build worker service only
- `make build-controller` - Build controller service only
- `make build-stress-test` - Build stress test tool only

### Running Services
- `make run-api` - Build and run API service
- `make run-worker` - Build and run worker service
- `make run-controller` - Build and run controller service
- `make run-stress-test` - Build and run stress test with default parameters

### Docker
- `make docker-build` - Build all Docker images
- `make docker-push` - Push all Docker images

### Kubernetes
- `make k8s-build` - Build Docker images for Kubernetes
- `make k8s-deploy` - Deploy to Kubernetes using kustomize
- `make k8s-delete` - Delete from Kubernetes
- `make k8s-load-images` - Load images into minikube
- `make k8s-local` - Complete local K8s workflow (build, load, deploy)
- `make k8s-port-forward` - Port forward API service to localhost:8080
- `make k8s-status` - Show status of all K8s resources
- `make k8s-logs` - Show logs for all services
- `make generate-crds` - Generate CRD manifests

### Testing
- `make test-coverage` - Run tests with coverage report
- `make run-stress-test` - Run load/stress testing with default configuration

### Cleanup
- `make clean` - Clean build artifacts

## Kubernetes Deployment

The project includes comprehensive Kubernetes manifests for local development and testing using minikube or similar local clusters.

### Quick Local Deployment

**Prerequisites:**
- Docker or compatible container runtime
- minikube or local Kubernetes cluster
- kubectl configured for your cluster
- kustomize installed

**Deploy Everything:**
```bash
# Complete workflow - builds, loads, and deploys
make k8s-local

# Access the API
make k8s-port-forward
# API available at http://localhost:8080

# Check status
make k8s-status

# View logs
make k8s-logs

# Clean up
make k8s-delete
```

### Manual Steps

```bash
# 1. Build Docker images
make k8s-build

# 2. Load images into minikube (if using minikube)
make k8s-load-images

# 3. Deploy to Kubernetes
make k8s-deploy

# 4. Check deployment status
kubectl get pods -n k8s-learning

# 5. Access API service
kubectl port-forward svc/api-service 8080:8080 -n k8s-learning
```

### Architecture Components

**Deployed Services:**
- **PostgreSQL**: Database with automatic `textprocessing` database creation
- **Redis**: Message queue and caching
- **API Service**: REST API with 2 replicas
- **Worker Service**: Background job processors with 2 replicas
- **Init Containers**: Wait for database readiness before API startup

**Storage:**
- **postgres-pvc**: 1GB persistent volume for PostgreSQL data
- **redis-pvc**: 512MB persistent volume for Redis data
- **uploads-pvc**: 2GB shared volume for uploaded files
- **results-pvc**: 2GB shared volume for processed results

**Configuration:**
- **app-config ConfigMap**: Environment variables for all services
- **app-secrets Secret**: Base64-encoded database and Redis credentials
- **postgres-init-script ConfigMap**: Database initialization SQL

### Environment-Specific Configuration

The deployment uses **kustomize overlays** for environment-specific settings:

**Development Overlay** (`deployments/overlays/development/`):
- Debug logging enabled (`LOG_LEVEL=debug`)
- Development image tags (`:dev`)
- Local-friendly configurations

**Base Configuration** (`deployments/base/`):
- Production-ready defaults
- Resource limits and requests
- Health checks and probes
- Persistent volume claims

### Database Setup

The PostgreSQL deployment automatically:
1. **Creates the database**: Uses `POSTGRES_DB=textprocessing` environment variable
2. **Runs init scripts**: Executes SQL from ConfigMap on first startup
3. **Provides persistence**: Data survives pod restarts via PVC
4. **Runs migrations**: API service runs golang-migrate on startup

**Database Initialization:**
- Database: `textprocessing` (auto-created)
- User: `postgres` / Password: `postgres` (for local dev)
- Init script ensures database exists before API starts
- Migrations run automatically when API starts

### Migration Configuration

Migrations use environment-specific paths:
- **Local development**: `DB_MIGRATIONS_URL=file://migrations` (relative path)
- **Kubernetes**: `DB_MIGRATIONS_URL=file:///app/migrations` (absolute path in container)
- **Custom environments**: Override via ConfigMap or environment variables

### Networking

**Internal Services** (ClusterIP):
- `postgres-service:5432` - PostgreSQL database
- `redis-service:6379` - Redis queue
- `api-service:8080` - API service (accessed via port-forward)

**External Access:**
- **Development**: Use `minikube tunnel` with ingress configuration
  - API available at `http://localhost/api/*`
  - Web UI available at `http://localhost/`
- **Alternative**: Use `kubectl port-forward svc/api-service 8080:8080 -n k8s-learning` for API-only access
- **Production**: Configure proper Ingress with domain names and TLS

### Resource Management

**Resource Limits:**
- **PostgreSQL**: 512MB memory, 500m CPU
- **Redis**: 256MB memory, 200m CPU  
- **API**: 512MB memory, 500m CPU
- **Worker**: 512MB memory, 500m CPU

**Scaling:**
- API: 2 replicas (can be scaled horizontally)
- Worker: 2 replicas (can be scaled based on queue depth)
- Database/Redis: 1 replica (stateful services)

### Monitoring and Debugging

```bash
# Check all resources
make k8s-status

# View logs from all services
make k8s-logs

# Debug specific service
kubectl logs -l app=api -n k8s-learning --follow
kubectl logs -l app=worker -n k8s-learning --follow
kubectl logs -l app=postgres -n k8s-learning
kubectl logs -l app=redis -n k8s-learning

# Describe resources
kubectl describe pods -n k8s-learning
kubectl describe pvc -n k8s-learning

# Access pod shell for debugging
kubectl exec -it deployment/api -n k8s-learning -- sh
```

### Troubleshooting

**Common Issues:**

1. **Images not found**: Run `make k8s-load-images` for minikube
2. **Database connection failed**: Check if PostgreSQL pod is ready
3. **Migrations fail**: Verify `textprocessing` database was created
4. **Pods pending**: Check PVC creation and storage class
5. **Init container timeout**: Database may need more time to start

**Clean deployment:**
```bash
make k8s-delete  # Removes namespace and all resources
make k8s-local   # Fresh deployment
```
