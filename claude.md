# Project Guidelines for Claude Code

## Go Coding Standards

### Code Style
- Follow standard Go conventions and use `gofmt` formatting
- Use meaningful variable and function names
- Prefer short, descriptive names for local variables
- Use PascalCase for exported functions/types, camelCase for unexported
- Keep functions small and focused on a single responsibility
- Don't add package name to struct name (e.g., `User` instead of `UserModel` or `ModelUser` if part of `model` package)
- Run `make lint` before committing to ensure code quality and consistency
- Use `golangci-lint` for comprehensive code analysis and style checking
- Use `log` instead of `logger` to define logger struct field, arguments and parameters

### Error Handling
- Always handle errors explicitly - never ignore them
- Use wrapped errors with `fmt.Errorf("operation: %w", err)` for context
- Do not use `fmt.Errorf("failed to do something: %v", err)` - prefer simple `fmt.Errorf("to do something: %w", err)`
- Return errors as the last return value
- Prefer custom error types for domain-specific errors
- Use `errors.Is` and `errors.As` for error checking

### Dependencies
- Prefer standard library when possible
- Use Go modules for dependency management
- Pin specific versions in go.mod for reproducible builds
- Avoid external dependencies for simple tasks
- Use github.com/kelseyhightower/envconfig for environment variable configuration
- Use github.com/golang-migrate/migrate for database migrations
- Use sigs.k8s.io/controller-runtime for Kubernetes controllers
- Use github.com/prometheus/client_golang for metrics exposition

### Testing
- Write table-driven tests when testing multiple scenarios
- Use testify/assert for test assertions
- Use gomock for mocking dependencies in tests
- Mock external dependencies using interfaces
- Aim for >80% test coverage on business logic
- Place tests in `*_test.go` files in the same package
- Skip tests for simple models/structs - focus on business logic and handlers

### Architecture Preferences
- Use dependency injection for better testability
- Implement interfaces in consuming packages, not providing packages
- Keep business logic separate from HTTP handlers
- Use context.Context for cancellation and timeouts
- Prefer composition over inheritance

### Logging
- Use structured logging with Go's slog package
- Log at appropriate levels (Debug, Info, Warn, Error)
- Include relevant context in log messages
- Don't log and return errors - choose one

### Performance
- Use benchmarks for performance-critical code
- Profile before optimizing
- Use buffered channels when you know the capacity

## Project-Specific Rules

### Database
- Use sqlx for database operations
- Always use prepared statements
- Handle database transactions explicitly
- Use golang-migrate for database migrations with numbered files: `000001_description.up.sql` and `000001_description.down.sql`
- Run migrations automatically during API service startup
- Store migrations in the `migrations/` directory at project root

### API Design
- Follow RESTful conventions
- Use proper HTTP status codes
- Validate input at API boundaries
- Return consistent error response format

### Configuration
- Use environment variables for configuration with envconfig struct tags
- Provide sensible defaults via `default:` struct tags
- Validate configuration on startup after environment processing
- Use clear environment variable naming (e.g., DB_HOST, REDIS_PORT)

## Don'ts
- Don't use `panic()` for normal error conditions
- Don't ignore context cancellation
- Don't use `interface{}` unless absolutely necessary
- Don't mutate slices/maps passed as parameters without documenting it
- Don't use `init()` functions unless absolutely necessary

## Kubernetes

### Deployment
- Use Deployments over bare Pods
- Always set resource requests and limits
- Use meaningful labels and selectors
- Prefer ConfigMaps for configuration over hardcoded values
- Use Secrets for sensitive data
- Set appropriate readiness and liveness probes

### Configuration
- Keep container images lightweight (use Alpine)
- Use multi-stage builds for Go applications
- Don't run containers as root user
- Group related resources in the same namespace

### Controller Development
- Use controller-runtime for building Kubernetes controllers
- Implement periodic reconciliation with time.Ticker for queue-based controllers
- Always use context.Context for cancellation support
- Use Patch instead of Update for deployment modifications to avoid conflicts
- Implement graceful shutdown handling for controllers
- Expose metrics via Prometheus for monitoring
- Use slog for structured logging with appropriate log levels
- Set proper RBAC permissions (ServiceAccount, Role, RoleBinding)
- Implement health and readiness probes for controller pods

### Health Endpoints
All services must expose standardized health endpoints following Kubernetes conventions:

**Required Endpoints:**
- `/livez` - Liveness probe: Basic check that the process is running and not deadlocked
  - Returns HTTP 200 OK if alive
  - Simple check, no external dependencies
  - Used by Kubernetes to restart unhealthy pods
- `/readyz` - Readiness probe: Check if service can accept traffic
  - Returns HTTP 200 OK if ready, 503 Service Unavailable if not ready
  - Must check all critical dependencies (database, Redis, etc.)
  - Used by Kubernetes to route traffic only to ready pods
- `/healthz` - General health check (typically an alias for `/livez`)
  - Follows Kubernetes ecosystem naming convention

**Implementation:**
- All services use **port 8080** for all endpoints (metrics, health, API)
- Single HTTP server per service serves all endpoints (metrics, health, business logic)
- Expose health endpoints on the same port as metrics endpoint
- Use context-aware health checks with reasonable timeouts
- Log health check failures for debugging
- Keep liveness checks simple to avoid false positives
- Make readiness checks comprehensive to ensure service quality

**Deployment Configuration:**
```yaml
ports:
- name: http
  containerPort: 8080
  protocol: TCP
livenessProbe:
  httpGet:
    path: /livez
    port: 8080
  initialDelaySeconds: 15-30
  periodSeconds: 10-20
readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5-10
```

**Backwards Compatibility:**
- For existing services, maintain old endpoints (`/health`, `/ready`) as aliases
- Gradually migrate to Kubernetes-standard endpoints

### Services & Networking
- Use Services for pod-to-pod communication
- Prefer ClusterIP for internal services
- Use Ingress for external HTTP/HTTPS access
- Set appropriate service ports and target ports

### Ingress Configuration
- For API routing with path prefixes (e.g., `/api`), use `ImplementationSpecific` pathType for proper pattern matching
- When routing `/api/*` to backend services, ensure the backend expects the correct path structure
- Test ingress routing thoroughly, especially when mixing web UI and API services
- Use `kubectl describe ingress` to verify routing rules are applied correctly

### YAML Structure
```yaml
# Prefer this organization:
# 1. Deployment
# 2. Service  
# 3. ConfigMap/Secret
# 4. Ingress (if needed)
```

## Web UI Guidelines

### Architecture
- Keep it simple: plain HTML, CSS, and vanilla JavaScript
- No build process required - files should run directly in browser
- Minimal third-party dependencies, only from CDN sources
- Progressive enhancement approach

### HTML
- Use semantic HTML5 elements
- Include proper meta tags (viewport, charset)
- Use meaningful class names and IDs
- Ensure accessibility with proper ARIA labels and alt text

### CSS
- Use modern CSS features (Grid, Flexbox, CSS variables)
- Avoid CSS frameworks - write custom minimal CSS
- Use CSS custom properties for theming
- Keep styles in a single CSS file when possible

### JavaScript
- Use ES6+ features (modules, arrow functions, async/await)
- Prefer vanilla JavaScript over frameworks
- Use fetch API for HTTP requests
- Handle errors gracefully with try/catch
- Use event delegation for dynamic content

### CDN Dependencies (if absolutely needed)
- Prefer these lightweight options:
    - Tailwind CSS (via CDN) if styling gets complex
    - Always specify version numbers in CDN URLs

### API Integration
- Use JSON for data exchange
- Handle loading states in the UI
- Show user-friendly error messages
- Implement basic client-side validation
- Use HTTP status codes appropriately

## Stress Testing Support

### Delay Parameter
The API now supports an optional `delay_ms` parameter for stress testing:
- **Parameter**: `delay_ms` (integer, 0-60000 milliseconds)
- **Purpose**: Simulates longer-running tasks to test queue behavior under load
- **Implementation**: Worker sleeps for specified duration before starting actual text processing
- **Database**: Stored in `jobs.delay_ms` column for tracking and analysis
- **Validation**: Input validation ensures delays are within reasonable bounds (max 1 minute)

### Usage
```bash
# Submit job with 5-second processing delay
curl -X POST "http://localhost:8080/api/v1/jobs" \
  -F "file=@test.txt" \
  -F "processing_type=wordcount" \
  -F "delay_ms=5000"
```

### Implementation Details
- Delay is applied at the beginning of worker processing, before actual text operations
- Uses context-aware sleep to support graceful cancellation
- Logged for debugging and monitoring purposes
- API response includes delay value for verification

### Stress Testing Tool

The project includes a dedicated stress testing tool (`cmd/stress-test`) for automated load testing:

**Build Commands:**
```bash
make build-stress-test     # Build stress test tool
make run-stress-test       # Build and run with default parameters
```

**Features:**
- Concurrent request generation with configurable worker count
- Randomized processing delays to simulate varying workloads
- Comprehensive metrics collection (latency, throughput, error rates)
- File upload simulation with real multipart form data
- Test duration control and graceful termination

**Usage:**
```bash
# Default test (30s duration, 2 workers, test-files/sample.txt)
make run-stress-test

# Custom parameters
./build/stress-test --file test-files/sample.txt \
  --duration 60 \
  --concurrency 5 \
  --min-process-delay 1000 \
  --max-process-delay 5000
```

**Parameters:**
- `--file`: Path to test file (required)
- `--min-process-delay`: Min delay in ms (default: 0)
- `--max-process-delay`: Max delay in ms (default: 30000)
- `--concurrency`: Number of concurrent requests (default: 1)
- `--query-delay`: Delay between requests in ms (default: 10)
- `--duration`: Test duration in seconds (default: 60)
- `--api-endpoint`: API endpoint URL (default: "http://localhost:8080/api/v1/jobs")

## Task Completion Protocol

After completing each significant task or implementation milestone:

1. **Update CLAUDE.md**:
   - Add any new build commands or development workflows
   - Update project-specific rules if patterns emerge
   - Document any new dependencies or architectural decisions
   - Add to the Code Review Checklist if new patterns are established

2. **Update STATUS.md**:
   - Move items from Pending to Completed as features are implemented
   - Add new pending items as they are identified
   - Update feature descriptions with implementation details

3. **Update README.md** (if necessary):
   - Update quick start instructions if setup changes
   - Add new make commands to Development Commands section
   - Keep it minimal - detailed docs belong in STATUS.md or separate docs

4. **Commit Changes**:
   - Use descriptive commit messages following conventional commits
   - Include both implementation and documentation updates in commits

## Development Commands

The build system uses a **parameterized approach** with `SERVICE=<name>` for flexible operations:

### Core Development Workflow
```bash
make fmt          # Format Go code
make lint         # Run golangci-lint for code quality checks
make test         # Run all tests
make build        # Build all Go services
make all          # Format, lint, test, and build
```

### Parameterized Build System

**Services:** `api`, `worker`, `controller`, `web`

All major commands support `SERVICE` parameter:
- Omit `SERVICE` or use `SERVICE=all` for all services
- Specify `SERVICE=<name>` for single service
- Specify `SERVICE="svc1 svc2"` for multiple services

```bash
# Build examples
make build                      # Build all services
make build SERVICE=api          # Build only API
make build SERVICE="api worker" # Build multiple services

# Run locally (requires SERVICE)
make run SERVICE=api            # Build and run API
make run SERVICE=controller     # Build and run controller

# Docker build
make docker-build               # Build all Docker images
make docker-build SERVICE=web   # Build only web image

# Kubernetes build
make k8s-build                  # Build all K8s images
make k8s-build SERVICE=worker   # Build only worker image
```

### Kubernetes Workflow Commands

```bash
# Full deployment (all services)
make k8s-local         # Build, load, deploy all services
make k8s-build         # Build Docker images with unique tags
make k8s-load          # Load images into minikube
make k8s-deploy        # Deploy to K8s

# Fast single-service redeploy (recommended for development)
make k8s-redeploy SERVICE=controller  # Rebuild + redeploy controller
make k8s-redeploy SERVICE=api         # Rebuild + redeploy API

# Quick reload without full deploy
make k8s-reload                       # Rebuild + reload all images
make k8s-reload SERVICE=worker        # Rebuild + reload worker only

# Utilities
make k8s-status                       # Show resource status
make k8s-logs                         # Show all service logs
make k8s-logs SERVICE=controller      # Follow controller logs (live)
make k8s-restart                      # Restart all deployments
make k8s-restart SERVICE=api          # Restart API deployment only
```

#### Image Tagging Strategy

The build system uses **git SHA + timestamp** tags for Kubernetes images to ensure every build is unique and avoids caching issues:

- **Format**: `<git-sha>-<timestamp>` (e.g., `cfb577f-1759830200`)
- **Benefits**:
  - Minikube always pulls fresh images (no cache conflicts)
  - Git SHA provides traceability to code state
  - Timestamp ensures uniqueness for uncommitted changes
  - Supports iterative development without committing

**How it works:**
- Each `make k8s-build` generates a unique tag
- `make k8s-redeploy` uses same tag across build/load/deploy steps
- Deployment is updated via `kubectl set image` to use the new tag
- Kubernetes automatically pulls and rolls out the new image

### Testing Commands
```bash
make test              # Run unit tests
make test-coverage     # Run tests with coverage report
make run-stress-test   # Run load/stress testing
```

### Monitoring Commands
```bash
# Deploy monitoring stack
kubectl apply -f deployments/base/monitoring/monitoring.yaml

# Port forward all services (recommended)
make k8s-forward
# This forwards:
#   - API:        http://localhost:8080 (metrics, health)
#   - Worker:     http://localhost:8181 (metrics, health)
#   - Controller: http://localhost:8282 (metrics, health)
#   - Grafana:    http://localhost:3000 (admin/admin)
#   - Prometheus: http://localhost:9090

# Check monitoring status
make monitoring-status

# Deploy/update Grafana dashboards
make deploy-dashboards

# Remove monitoring stack
kubectl delete -f deployments/base/monitoring/monitoring.yaml
```

### Adding New Services

When adding a new service:
1. Add to `SERVICES` list in Makefile (top of file)
2. Add to `GO_SERVICES` if it's a Go service
3. Create `cmd/<service>/main.go`
4. Create `docker/Dockerfile.<service>`
5. No other Makefile changes needed - parameterized targets handle it automatically

### Code Quality
- Always run `make lint` before committing changes
- Use `make fmt` to format code according to Go standards
- Run `make test-coverage` to ensure adequate test coverage
- The project uses golangci-lint with comprehensive rules defined in `.golangci.yml`

## Auto-Scaling Architecture

The project implements queue-based auto-scaling using a custom Kubernetes controller:

### Controller Design
- Monitors Redis queue depth (main + priority queues) every 30 seconds
- Scales worker deployment up/down based on queue thresholds
- Uses Kubernetes client-go and controller-runtime libraries
- Exposes Prometheus metrics for monitoring

### Scaling Logic
- **Scale up**: When queue depth > 20, add up to 2 workers (max 10 total)
- **Scale down**: When queue depth < 5, remove 1 worker (min 1 total)
- **Jobs per worker**: Estimated capacity of 10 jobs per worker
- Uses Patch operations to avoid resource version conflicts

### Configuration
- `RECONCILE_INTERVAL`: Controller reconciliation interval (default: 30s)
- `METRICS_COLLECTION_INTERVAL`: Metrics update interval (default: 15s)
- Environment variables: `REDIS_HOST`, `REDIS_PORT`, `LOG_LEVEL`

See `docs/AUTO_SCALING.md` for comprehensive architecture details.

## Monitoring

### Prometheus Metrics

All services should expose metrics via the `/metrics` endpoint for Prometheus scraping:

**Standard Metrics to Expose:**
- HTTP metrics: request count, duration, size (requests/responses)
- Application-specific metrics: jobs, operations, events
- Resource metrics: database connections, cache hits/misses
- Error metrics: error rates, failure counts

**Metric Naming Conventions:**
- Use snake_case (e.g., `http_requests_total`)
- Include units in metric names (e.g., `_seconds`, `_bytes`, `_total`)
- Use appropriate metric types:
  - Counter: monotonically increasing values (e.g., `requests_total`)
  - Gauge: values that can go up or down (e.g., `connections_active`)
  - Histogram: distributions (e.g., `request_duration_seconds`)

**Pod Annotations for Scraping:**
Add these annotations to enable Prometheus scraping:
```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
```

### Grafana Dashboards

- Create dashboard JSON files in `deployments/base/monitoring/dashboards/`
- Include panels for key metrics: latency, throughput, errors, resource usage
- Use consistent color schemes and units
- Add threshold lines for SLOs/SLAs
- Deploy dashboards with `make deploy-dashboards` after creating/updating JSON files
- Dashboards are automatically loaded via ConfigMap provisioning

**Available Dashboards:**
- `api-dashboard.json` - API service metrics (requests, latency, errors)
- `worker-dashboard.json` - Worker metrics (jobs, queues, database, Redis operations)

See `docs/MONITORING.md` for detailed setup and dashboard creation.

## Code Review Checklist
When reviewing or suggesting changes, ensure:
- [ ] Code passes `make lint` without errors
- [ ] Proper error handling
- [ ] Context usage for cancellation
- [ ] Resource cleanup (defer statements)
- [ ] Thread safety considerations
- [ ] Input validation
- [ ] Appropriate logging level
- [ ] Test coverage for new functionality
- [ ] K8s resources have proper labels and resource limits
- [ ] K8s controllers use Patch instead of Update for modifications
- [ ] Controllers implement graceful shutdown
- [ ] Health endpoints (/livez, /readyz, /healthz) are implemented for all services
- [ ] Liveness and readiness probes are configured in deployment YAMLs
- [ ] Readiness checks verify all critical dependencies (DB, Redis, etc.)
- [ ] Web UI is accessible and responsive
- [ ] No unnecessary JavaScript dependencies
- [ ] Documentation updated (STATUS.md for features, CLAUDE.md for standards, README.md for setup changes)