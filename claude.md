# Project Guidelines for Claude Code

## Go Coding Standards

### Code Style
- Follow standard Go conventions and use `gofmt` formatting
- Use meaningful variable and function names
- Prefer short, descriptive names for local variables
- Use PascalCase for exported functions/types, camelCase for unexported
- Keep functions small and focused on a single responsibility

### Error Handling
- Always handle errors explicitly - never ignore them
- Use wrapped errors with `fmt.Errorf("operation failed: %w", err)` for context
- Return errors as the last return value
- Prefer custom error types for domain-specific errors

### Project Structure
```
cmd/          - Main applications
internal/     - Private application code
pkg/          - Public library code  
api/          - API definitions (OpenAPI, protobuf)
configs/      - Configuration files
scripts/      - Build and deployment scripts
```

### Dependencies
- Prefer standard library when possible
- Use Go modules for dependency management
- Pin specific versions in go.mod for reproducible builds
- Avoid external dependencies for simple tasks

### Testing
- Write table-driven tests when testing multiple scenarios
- Use testify/assert for test assertions
- Mock external dependencies using interfaces
- Aim for >80% test coverage on business logic
- Place tests in `*_test.go` files in the same package

### Architecture Preferences
- Use dependency injection for better testability
- Implement interfaces in consuming packages, not providing packages
- Keep business logic separate from HTTP handlers
- Use context.Context for cancellation and timeouts
- Prefer composition over inheritance

### Logging
- Use structured logging (logrus or zap)
- Log at appropriate levels (Debug, Info, Warn, Error)
- Include relevant context in log messages
- Don't log and return errors - choose one

### Performance
- Use benchmarks for performance-critical code
- Profile before optimizing
- Prefer sync.Pool for object reuse when appropriate
- Use buffered channels when you know the capacity

## Project-Specific Rules

### Database
- Use sqlx for database operations
- Always use prepared statements
- Handle database transactions explicitly
- Use migrations for schema changes

### API Design
- Follow RESTful conventions
- Use proper HTTP status codes
- Validate input at API boundaries
- Return consistent error response format

### Configuration
- Use environment variables for configuration
- Provide sensible defaults
- Validate configuration on startup

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
- Keep container images lightweight (use Alpine or distroless)
- Use multi-stage builds for Go applications
- Don't run containers as root user
- Use specific image tags, never `:latest` in production
- Group related resources in the same namespace

### Services & Networking
- Use Services for pod-to-pod communication
- Prefer ClusterIP for internal services
- Use Ingress for external HTTP/HTTPS access
- Set appropriate service ports and target ports

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
- Mobile-first responsive design
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
    - htmx for AJAX interactions
    - Tailwind CSS (via CDN) if styling gets complex
    - Chart.js for simple charts
    - Always specify version numbers in CDN URLs

### File Structure
```
web/
├── index.html
├── css/
│   └── style.css
├── js/
│   └── app.js
└── assets/
    └── images/
```

### API Integration
- Use JSON for data exchange
- Handle loading states in the UI
- Show user-friendly error messages
- Implement basic client-side validation
- Use HTTP status codes appropriately

## Code Review Checklist
When reviewing or suggesting changes, ensure:
- [ ] Proper error handling
- [ ] Context usage for cancellation
- [ ] Resource cleanup (defer statements)
- [ ] Thread safety considerations
- [ ] Input validation
- [ ] Appropriate logging level
- [ ] Test coverage for new functionality
- [ ] K8s resources have proper labels and resource limits
- [ ] Web UI is accessible and responsive
- [ ] No unnecessary JavaScript dependencies