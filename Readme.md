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
│   └── controller/
│       └── main.go                 # Controller entrypoint
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


## Implementation Status

### ✅ Completed Components
- **API Service (`text-api`)**: Full REST API implementation with:
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

### 🚧 In Progress
- Worker Service (`text-worker`)
- Controller (`text-controller`)
- Web UI (`text-ui`)
- Kubernetes manifests
- Docker images

### 📋 Pending
- Database and Redis StatefulSets
- Helm charts
- CI/CD pipeline
- Monitoring and observability

## API Endpoints

### Job Management
- `POST /api/v1/jobs` - Submit text processing job with file upload
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
- `make build` - Build all binaries (api, worker, controller)
- `make build-api` - Build API service only
- `make build-worker` - Build worker service only
- `make build-controller` - Build controller service only

### Running Services
- `make run-api` - Build and run API service
- `make run-worker` - Build and run worker service
- `make run-controller` - Build and run controller service

### Docker
- `make docker-build` - Build all Docker images
- `make docker-push` - Push all Docker images

### Kubernetes
- `make k8s-deploy` - Deploy to Kubernetes
- `make k8s-delete` - Delete from Kubernetes
- `make generate-crds` - Generate CRD manifests

### Testing
- `make test-coverage` - Run tests with coverage report

### Cleanup
- `make clean` - Clean build artifacts
