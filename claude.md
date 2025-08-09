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

## Task Completion Protocol

After completing each significant task or implementation milestone:

1. **Update CLAUDE.md**:
   - Add any new build commands or development workflows
   - Update project-specific rules if patterns emerge
   - Document any new dependencies or architectural decisions
   - Add to the Code Review Checklist if new patterns are established

2. **Update README.md** (if necessary):
   - Update build status or completion phases
   - Add new API endpoints or features to documentation
   - Update deployment instructions if changed
   - Modify architecture diagrams if significant changes made

3. **Commit Changes**:
   - Use descriptive commit messages following conventional commits
   - Include both implementation and documentation updates in commits
   - Update any relevant phase completion status

## Development Commands

### Core Development Workflow
```bash
make fmt          # Format Go code
make lint         # Run golangci-lint for code quality checks  
make test         # Run all tests
make build        # Build all binaries
make all          # Format, lint, test, and build
```

### Code Quality
- Always run `make lint` before committing changes
- Use `make fmt` to format code according to Go standards
- Run `make test-coverage` to ensure adequate test coverage
- The project uses golangci-lint with comprehensive rules defined in `.golangci.yml`

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
- [ ] Web UI is accessible and responsive
- [ ] No unnecessary JavaScript dependencies
- [ ] Documentation updated (CLAUDE.md and README.md if needed)