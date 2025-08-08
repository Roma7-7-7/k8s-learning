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
- Simple authentication (optional enhancement)

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
**Language**: HTML/CSS/JavaScript (or React)  
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
- ConfigMap (nginx configuration)

### 5. Database (`postgresql`)
**Type**: PostgreSQL StatefulSet  
**Purpose**: Persistent storage for job metadata and status

**Schema**:
```sql
-- Jobs table
CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255),
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

## Custom Resource Definitions

### TextProcessingJob CRD
```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: textprocessingjobs.textprocessing.example.com
spec:
  group: textprocessing.example.com
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              jobId:
                type: string
              processingType:
                type: string
                enum: ["wordcount", "linecount", "uppercase", "lowercase", "replace", "extract"]
              parameters:
                type: object
              priority:
                type: integer
                minimum: 1
                maximum: 10
          status:
            type: object
            properties:
              phase:
                type: string
                enum: ["Pending", "Running", "Succeeded", "Failed"]
              startTime:
                type: string
              completionTime:
                type: string
              workerId:
                type: string
  scope: Namespaced
  names:
    plural: textprocessingjobs
    singular: textprocessingjob
    kind: TextProcessingJob
```

## Security Configuration

### Service Accounts & RBAC

#### API Service Account
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: text-api
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: api-minimal
rules:
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["get", "list"]
- apiGroups: ["textprocessing.example.com"]
  resources: ["textprocessingjobs"]
  verbs: ["create", "get", "list", "update", "patch"]
```

#### Worker Service Account
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: text-worker
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: worker-minimal
rules:
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["get", "list"]
```

#### Controller Service Account
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: text-controller
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: text-controller
rules:
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["create", "get", "list", "watch", "update", "patch", "delete"]
- apiGroups: ["textprocessing.example.com"]
  resources: ["textprocessingjobs"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create"]
```

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

## Deployment Phases

### Phase 1: Basic Services (Weeks 1-2)
- Deploy API, Worker, Database, Redis
- Basic image processing functionality
- Simple queue-based task processing

### Phase 2: Advanced K8s Features (Weeks 3-4)
- Add health checks and probes
- Implement proper RBAC
- Add Helm charts
- StatefulSet optimizations

### Phase 3: Custom Controller (Weeks 5-7)
- Implement ImageProcessingJob CRD
- Build controller with reconciliation
- Auto-scaling based on queue metrics

### Phase 4: Observability (Weeks 8-9)
- Add Prometheus metrics
- Implement structured logging
- Create monitoring dashboards
- Distributed tracing setup

### Phase 5: Production Readiness (Week 10)
- Security hardening
- Performance optimization
- Disaster recovery testing
- Load testing and tuning

## Key Learning Outcomes

### Kubernetes Concepts Covered
- Pod lifecycle and container management
- Services and networking
- Deployments and StatefulSets
- Jobs and CronJobs
- ConfigMaps and Secrets
- Persistent Volumes
- RBAC and security
- Custom Resource Definitions
- Controllers and operators
- Horizontal Pod Autoscaling
- Network Policies

### Go Skills Developed
- REST API development
- Background job processing
- Kubernetes client-go library
- Controller-runtime framework
- Database integration
- Redis integration
- Error handling and logging
- Testing strategies

### DevOps Practices
- Infrastructure as Code (Terraform)
- CI/CD pipeline concepts
- Container orchestration
- Monitoring and observability
- Security best practices
- Disaster recovery planning

## Success Metrics

### Technical Metrics
- API response time < 50ms
- Job processing throughput > 100 texts/second
- Worker auto-scaling latency < 30 seconds
- System availability > 99.9%

### Learning Metrics
- Ability to explain Kubernetes architecture
- Demonstrate controller pattern implementation
- Troubleshoot common K8s issues
- Implement security best practices
- Design scalable microservices architecture