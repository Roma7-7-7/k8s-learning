# Text Processing Queue - Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt

# Binary names
API_BINARY=text-api
WORKER_BINARY=text-worker
CONTROLLER_BINARY=text-controller

# Build directory
BUILD_DIR=build

# Docker parameters
DOCKER_REGISTRY=localhost:5000
IMAGE_TAG=latest

.PHONY: all build clean test deps fmt lint help web

# Default target
all: fmt test build

# Build all binaries
build: build-api build-worker

# Build individual components
build-api:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(API_BINARY) -v ./cmd/api

build-worker:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(WORKER_BINARY) -v ./cmd/worker

build-controller:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(CONTROLLER_BINARY) -v ./cmd/controller

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -coverprofile=coverage.out -v ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Format code
fmt:
	$(GOFMT) -w .

# Lint code (requires golangci-lint)
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not found, install with: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.54.2" && exit 1)
	golangci-lint run

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	$(GOMOD) tidy
	$(GOMOD) download

# Build Docker images
docker-build: docker-build-api docker-build-worker docker-build-controller docker-build-ui

docker-build-api:
	docker build -f docker/api.Dockerfile -t $(DOCKER_REGISTRY)/$(API_BINARY):$(IMAGE_TAG) .

docker-build-worker:
	docker build -f docker/worker.Dockerfile -t $(DOCKER_REGISTRY)/$(WORKER_BINARY):$(IMAGE_TAG) .

docker-build-controller:
	docker build -f docker/controller.Dockerfile -t $(DOCKER_REGISTRY)/$(CONTROLLER_BINARY):$(IMAGE_TAG) .

docker-build-ui:
	docker build -f docker/ui.Dockerfile -t $(DOCKER_REGISTRY)/text-ui:$(IMAGE_TAG) .

# Push Docker images
docker-push: docker-push-api docker-push-worker docker-push-controller docker-push-ui

docker-push-api:
	docker push $(DOCKER_REGISTRY)/$(API_BINARY):$(IMAGE_TAG)

docker-push-worker:
	docker push $(DOCKER_REGISTRY)/$(WORKER_BINARY):$(IMAGE_TAG)

docker-push-controller:
	docker push $(DOCKER_REGISTRY)/$(CONTROLLER_BINARY):$(IMAGE_TAG)

docker-push-ui:
	docker push $(DOCKER_REGISTRY)/text-ui:$(IMAGE_TAG)

# Kubernetes deployment
k8s-deploy:
	kubectl apply -f deployments/base/

k8s-delete:
	kubectl delete -f deployments/base/

# Development helpers
run-api:
	$(GOBUILD) -o $(BUILD_DIR)/$(API_BINARY) ./cmd/api && ./$(BUILD_DIR)/$(API_BINARY)

run-worker:
	$(GOBUILD) -o $(BUILD_DIR)/$(WORKER_BINARY) ./cmd/worker && ./$(BUILD_DIR)/$(WORKER_BINARY)

run-controller:
	$(GOBUILD) -o $(BUILD_DIR)/$(CONTROLLER_BINARY) ./cmd/controller && ./$(BUILD_DIR)/$(CONTROLLER_BINARY)

# Generate CRD manifests (requires controller-gen)
generate-crds:
	@which controller-gen > /dev/null || (echo "controller-gen not found, install with: go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest" && exit 1)
	controller-gen crd paths=./internal/models/... output:crd:artifacts:config=deployments/base/controller/

# Local development setup
setup-dev:
	@echo "Setting up local development environment..."
	@if [ ! -f .env ]; then \
		echo "Creating .env file from template..."; \
		cp .env.distro .env; \
		echo "✓ Created .env file"; \
		echo "⚠ Please edit .env with your local database and Redis settings"; \
	else \
		echo "✓ .env file already exists"; \
	fi
	@mkdir -p uploads results
	@echo "✓ Created upload and result directories"
	@echo ""
	@echo "Next steps:"
	@echo "1. Edit .env file with your database and Redis connection details"
	@echo "2. Start PostgreSQL and Redis services"
	@echo "3. Run: make run-api"

# Run web UI development server
web:
	@echo "Starting web UI development server..."
	@echo "Access the web UI at: http://localhost:3000"
	@echo "Make sure your API server is running on http://localhost:8080"
	@echo "Press Ctrl+C to stop the server"
	@cd web && python3 -m http.server 3000

# Help
help:
	@echo "Available targets:"
	@echo "  all              - Format, test, and build all components"
	@echo "  build            - Build all binaries"
	@echo "  build-api        - Build API service"
	@echo "  build-worker     - Build worker service"
	@echo "  build-controller - Build controller service"
	@echo "  test             - Run all tests"
	@echo "  test-coverage    - Run tests with coverage report"
	@echo "  fmt              - Format Go code"
	@echo "  lint             - Run linter (requires golangci-lint)"
	@echo "  clean            - Clean build artifacts"
	@echo "  deps             - Download and tidy dependencies"
	@echo "  docker-build     - Build all Docker images"
	@echo "  docker-push      - Push all Docker images"
	@echo "  k8s-deploy       - Deploy to Kubernetes"
	@echo "  k8s-delete       - Delete from Kubernetes"
	@echo "  run-api          - Build and run API service"
	@echo "  run-worker       - Build and run worker service"
	@echo "  run-controller   - Build and run controller service"
	@echo "  setup-dev        - Setup local development environment (.env file and directories)"
	@echo "  web              - Start web UI development server on http://localhost:3000"
	@echo "  generate-crds    - Generate CRD manifests"
	@echo "  help             - Show this help message"