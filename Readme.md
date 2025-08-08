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
│   └── models/
│       └── job.go                  # Data structures
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
├── terraform/
│   ├── modules/
│   │   └── k8s-cluster/            # Cluster provisioning
│   ├── environments/
│   │   ├── dev/                    # Development environment
│   │   └── prod/                   # Production environment
│   └── main.tf                     # Root configuration
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
