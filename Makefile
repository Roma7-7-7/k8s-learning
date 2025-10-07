# Text Processing Queue - Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt

# Service definitions
SERVICES := api worker controller web
GO_SERVICES := api worker controller
STRESS_TEST_BINARY=stress-test

# Build directory
BUILD_DIR=build

# Docker/K8s parameters
DOCKER_REGISTRY=localhost:5000
IMAGE_TAG=latest
K8S_NAMESPACE=k8s-learning
K8S_TAG=dev

# Generate unique tag for K8s images (git SHA + timestamp)
# Use K8S_TAG_OVERRIDE if set, otherwise generate new tag
GIT_SHA := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
TIMESTAMP := $(shell date +%s)
ifdef K8S_TAG_OVERRIDE
	UNIQUE_TAG := $(K8S_TAG_OVERRIDE)
else
	UNIQUE_TAG := $(GIT_SHA)-$(TIMESTAMP)
endif

# Service to operate on (default: all)
SERVICE ?= all

# Helper function to get services list
ifeq ($(SERVICE),all)
	SELECTED_SERVICES=$(SERVICES)
	SELECTED_GO_SERVICES=$(GO_SERVICES)
else
	SELECTED_SERVICES=$(SERVICE)
	SELECTED_GO_SERVICES=$(filter $(SERVICE),$(GO_SERVICES))
endif

.PHONY: all build clean test deps fmt lint help web monitoring-status deploy-dashboards k8s-forward

# Default target
all: fmt test build

#
# Go Build Targets
#

# Build Go binaries
build:
	@mkdir -p $(BUILD_DIR)
	@$(foreach svc,$(SELECTED_GO_SERVICES), \
		echo "Building $(svc)..."; \
		$(GOBUILD) -o $(BUILD_DIR)/text-$(svc) -v ./cmd/$(svc) || exit 1; \
	)
	@echo "✅ Build complete"

# Build stress test tool
build-stress-test:
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(STRESS_TEST_BINARY) -v ./cmd/stress-test

#
# Development Run Targets
#

run:
	@if [ "$(SERVICE)" = "all" ]; then \
		echo "❌ Error: Specify a service with SERVICE=<name>"; \
		echo "Example: make run SERVICE=api"; \
		exit 1; \
	fi
	@if [ ! -f "cmd/$(SERVICE)/main.go" ]; then \
		echo "❌ Error: Service '$(SERVICE)' not found"; \
		exit 1; \
	fi
	@echo "🚀 Running $(SERVICE)..."
	@$(GOBUILD) -o $(BUILD_DIR)/text-$(SERVICE) ./cmd/$(SERVICE) && ./$(BUILD_DIR)/text-$(SERVICE)

run-stress-test:
	@$(GOBUILD) -o $(BUILD_DIR)/$(STRESS_TEST_BINARY) ./cmd/stress-test && \
	./$(BUILD_DIR)/$(STRESS_TEST_BINARY) --file test-files/sample.txt --duration 30 --concurrency 2 --min-process-delay 500 --max-process-delay 2000

#
# Test Targets
#

test:
	$(GOTEST) -v ./...

test-coverage:
	$(GOTEST) -coverprofile=coverage.out -v ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

#
# Code Quality Targets
#

fmt:
	$(GOFMT) -w .

lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not found, install with: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.54.2" && exit 1)
	golangci-lint run

clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

deps:
	$(GOMOD) tidy
	$(GOMOD) download

#
# Docker Targets
#

docker-build:
	@$(foreach svc,$(SELECTED_SERVICES), \
		echo "🐳 Building $(svc) Docker image..."; \
		docker build -f docker/Dockerfile.$(svc) -t $(DOCKER_REGISTRY)/text-$(svc):$(IMAGE_TAG) . || exit 1; \
	)
	@echo "✅ Docker build complete"

docker-push:
	@$(foreach svc,$(SELECTED_SERVICES), \
		echo "📤 Pushing $(svc) image..."; \
		docker push $(DOCKER_REGISTRY)/text-$(svc):$(IMAGE_TAG) || exit 1; \
	)
	@echo "✅ Docker push complete"

#
# Kubernetes Targets
#

k8s-build:
	@$(foreach svc,$(SELECTED_SERVICES), \
		echo "🐳 Building $(svc) for K8s (tag: $(UNIQUE_TAG))..."; \
		docker build -f docker/Dockerfile.$(svc) -t k8s-learning/$(svc):$(UNIQUE_TAG) . || exit 1; \
	)
	@echo "✅ K8s images built successfully with tag: $(UNIQUE_TAG)"

k8s-load:
	@$(foreach svc,$(SELECTED_SERVICES), \
		echo "📤 Loading $(svc):$(UNIQUE_TAG) into minikube..."; \
		minikube image load k8s-learning/$(svc):$(UNIQUE_TAG) || exit 1; \
	)
	@echo "✅ Images loaded into minikube"

k8s-deploy:
	@echo "🚀 Deploying to Kubernetes with tag $(UNIQUE_TAG)..."
	@./scripts/deploy-local.sh $(UNIQUE_TAG)
	@echo "✅ Deployment complete"

k8s-clean:
	@echo "🧹 Cleaning all Kubernetes resources..."
	@echo "⚠️  This will delete:"
	@echo "   - All deployments, services, PVCs in k8s-learning namespace"
	@echo "   - Monitoring stack in monitoring namespace"
	@echo "   - All loaded images from minikube"
	@echo ""
	@read -p "Are you sure? (y/N): " confirm && [ "$$confirm" = "y" ] || exit 1
	@echo ""
	@echo "🗑️  Deleting k8s-learning namespace resources..."
	@kubectl delete namespace k8s-learning --ignore-not-found=true --timeout=60s
	@echo "🗑️  Deleting monitoring namespace resources..."
	@kubectl delete namespace monitoring --ignore-not-found=true --timeout=60s
	@echo "🧹 Cleaning minikube images..."
	@echo "Finding and removing all k8s-learning images from minikube..."
	@minikube ssh -- 'docker images --format "{{.Repository}}:{{.Tag}}" | grep "^k8s-learning/" | xargs -r docker rmi -f' 2>/dev/null || true
	@echo "✅ Cleanup complete"

k8s-redeploy:
	@if [ "$(SERVICE)" = "all" ]; then \
		echo "❌ Error: Specify a service with SERVICE=<name>"; \
		echo "Example: make k8s-redeploy SERVICE=controller"; \
		exit 1; \
	fi
	@echo "🔄 Redeploying $(SERVICE) with tag $(UNIQUE_TAG)..."
	@$(MAKE) build SERVICE=$(SERVICE)
	@$(MAKE) k8s-build SERVICE=$(SERVICE) K8S_TAG_OVERRIDE=$(UNIQUE_TAG)
	@$(MAKE) k8s-load SERVICE=$(SERVICE) K8S_TAG_OVERRIDE=$(UNIQUE_TAG)
	@echo "🔄 Updating deployment to use new image..."
	@kubectl set image deployment/$(SERVICE) $(SERVICE)=k8s-learning/$(SERVICE):$(UNIQUE_TAG) -n $(K8S_NAMESPACE)
	@echo "⏳ Waiting for rollout to complete..."
	@kubectl rollout status deployment/$(SERVICE) -n $(K8S_NAMESPACE) --timeout=60s
	@echo "✅ $(SERVICE) redeployed successfully with tag $(UNIQUE_TAG)"
	@echo ""
	@echo "📊 Status:"
	@kubectl get pods -l app=$(SERVICE) -n $(K8S_NAMESPACE)
	@echo ""
	@echo "📝 To view logs: kubectl logs -l app=$(SERVICE) -n $(K8S_NAMESPACE) -f"

# Complete local K8s workflow
k8s-local:
	@echo "🚀 Starting complete K8s workflow with tag $(UNIQUE_TAG)..."
	@$(MAKE) k8s-build K8S_TAG_OVERRIDE=$(UNIQUE_TAG)
	@$(MAKE) k8s-load K8S_TAG_OVERRIDE=$(UNIQUE_TAG)
	@$(MAKE) k8s-deploy K8S_TAG_OVERRIDE=$(UNIQUE_TAG)
	@echo "📊 Deploying monitoring stack..."
	@kubectl apply -f deployments/base/monitoring/monitoring.yaml
	@echo "✅ Monitoring stack deployed"

# Quick rebuild and reload (without full deploy)
k8s-reload:
	@echo "🔄 Rebuilding and reloading with tag $(UNIQUE_TAG)..."
	@$(MAKE) k8s-build K8S_TAG_OVERRIDE=$(UNIQUE_TAG)
	@$(MAKE) k8s-load K8S_TAG_OVERRIDE=$(UNIQUE_TAG)
	@echo "✅ Images rebuilt and reloaded. Restart pods to use new images:"
	@echo "   kubectl rollout restart deployment -n $(K8S_NAMESPACE)"

#
# Kubernetes Utilities
#

k8s-forward:
	@echo "🔌 Setting up port forwarding for all services..."
	@echo ""
	@echo "Services will be available at:"
	@echo "  • API:        http://localhost:8080"
	@echo "  • Grafana:    http://localhost:3000 (admin/admin)"
	@echo "  • Prometheus: http://localhost:9090"
	@echo "  • Controller: http://localhost:8081/metrics"
	@echo ""
	@echo "Press Ctrl+C to stop all port forwards"
	@echo ""
	@trap 'kill 0' EXIT; \
	kubectl port-forward -n $(K8S_NAMESPACE) svc/api 8080:8080 & \
	kubectl port-forward -n monitoring svc/grafana 3000:3000 & \
	kubectl port-forward -n monitoring svc/prometheus 9090:9090 & \
	kubectl port-forward -n $(K8S_NAMESPACE) svc/controller-metrics-service 8081:8081 & \
	wait

k8s-status:
	@echo "=== Pods ==="
	@kubectl get pods -n $(K8S_NAMESPACE) -o wide
	@echo ""
	@echo "=== Services ==="
	@kubectl get svc -n $(K8S_NAMESPACE)
	@echo ""
	@echo "=== PVCs ==="
	@kubectl get pvc -n $(K8S_NAMESPACE)

k8s-logs:
	@if [ "$(SERVICE)" = "all" ]; then \
		for svc in postgres redis $(SERVICES); do \
			echo "=== $$svc Logs ==="; \
			kubectl logs -l app=$$svc -n $(K8S_NAMESPACE) --tail=20 2>/dev/null || echo "No logs for $$svc"; \
			echo ""; \
		done; \
	else \
		echo "=== $(SERVICE) Logs ==="; \
		kubectl logs -l app=$(SERVICE) -n $(K8S_NAMESPACE) --tail=50 -f; \
	fi

k8s-restart:
	@if [ "$(SERVICE)" = "all" ]; then \
		echo "♻️  Restarting all deployments..."; \
		kubectl rollout restart deployment -n $(K8S_NAMESPACE); \
	else \
		echo "♻️  Restarting $(SERVICE)..."; \
		kubectl rollout restart deployment/$(SERVICE) -n $(K8S_NAMESPACE); \
	fi
	@echo "✅ Restart initiated"

#
# Monitoring
#

monitoring-status:
	@echo "=== Monitoring Pods ==="
	@kubectl get pods -n monitoring -o wide
	@echo ""
	@echo "=== Monitoring Services ==="
	@kubectl get svc -n monitoring

deploy-dashboards:
	@./scripts/deploy-grafana-dashboards.sh

#
# Development Helpers
#

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
	@echo "3. Run: make run SERVICE=api"

web:
	@echo "Starting web UI development server..."
	@echo "Access the web UI at: http://localhost:3000"
	@echo "Make sure your API server is running on http://localhost:8080"
	@echo "Press Ctrl+C to stop the server"
	@cd web && python3 -m http.server 3000

#
# Help
#

help:
	@echo "Text Processing Queue - Build & Deploy System"
	@echo ""
	@echo "Usage: make TARGET [SERVICE=<service>]"
	@echo ""
	@echo "Services: $(SERVICES)"
	@echo "  - Specify SERVICE=<name> to operate on a single service"
	@echo "  - Specify SERVICE=\"svc1 svc2\" for multiple services"
	@echo "  - Omit SERVICE or use SERVICE=all for all services"
	@echo ""
	@echo "Build Targets:"
	@echo "  build              Build Go services [SERVICE=all]"
	@echo "  build-stress-test  Build stress testing tool"
	@echo "  docker-build       Build Docker images [SERVICE=all]"
	@echo "  docker-push        Push Docker images [SERVICE=all]"
	@echo "  k8s-build          Build images for K8s [SERVICE=all]"
	@echo ""
	@echo "Development Targets:"
	@echo "  run                Build and run a service [SERVICE required]"
	@echo "  run-stress-test    Run stress test with default params"
	@echo "  setup-dev          Setup local dev environment"
	@echo "  web                Start web UI dev server"
	@echo ""
	@echo "Kubernetes Targets:"
	@echo "  k8s-local          Complete workflow: build, load, deploy [SERVICE=all]"
	@echo "  k8s-build          Build K8s images [SERVICE=all]"
	@echo "  k8s-load           Load images into minikube [SERVICE=all]"
	@echo "  k8s-reload         Quick rebuild & reload [SERVICE=all]"
	@echo "  k8s-deploy         Deploy to K8s (full deployment)"
	@echo "  k8s-redeploy       Fast redeploy single service [SERVICE required]"
	@echo "  k8s-restart        Restart K8s deployments [SERVICE=all]"
	@echo "  k8s-clean          Destroy everything (namespaces + images)"
	@echo ""
	@echo "Kubernetes Utilities:"
	@echo "  k8s-status         Show K8s resource status"
	@echo "  k8s-logs           Show logs [SERVICE=all or specific]"
	@echo "  k8s-forward        Port forward all services (API, Grafana, Prometheus, Controller)"
	@echo ""
	@echo "Monitoring:"
	@echo "  monitoring-status  Show monitoring stack status"
	@echo "  deploy-dashboards  Deploy Grafana dashboards from JSON files"
	@echo ""
	@echo "Test Targets:"
	@echo "  test               Run unit tests"
	@echo "  test-coverage      Run tests with coverage"
	@echo ""
	@echo "Code Quality:"
	@echo "  fmt                Format Go code"
	@echo "  lint               Run linter"
	@echo "  clean              Clean build artifacts"
	@echo "  deps               Download dependencies"
	@echo ""
	@echo "Examples:"
	@echo "  make build                         # Build all services"
	@echo "  make build SERVICE=api             # Build only API"
	@echo "  make run SERVICE=controller        # Run controller locally"
	@echo "  make k8s-redeploy SERVICE=worker   # Quick worker redeploy"
	@echo "  make k8s-build SERVICE=\"api worker\"  # Build specific services"
	@echo "  make k8s-logs SERVICE=controller   # Follow controller logs"
