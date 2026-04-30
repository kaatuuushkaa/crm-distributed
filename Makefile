.PHONY: help infra-up infra-down full-up full-down monitoring-up monitoring-down logs \
        run-task run-notif run-doc \
        build build-task build-notif build-doc \
        proto lint test test-cover \
        migrate-up migrate-down \
        docker-build \
        k8s-up k8s-down k8s-apply \
        bench bench-spike bench-soak

#Variables

BINARY_DIR := bin
COMPOSE_FILE := deploy/docker-compose.yml
CLUSTER_NAME := crm-local

-include .env
export

#Help

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
             		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2}' \
             		| sort

#Infrastructure

infra-up: ## Start infrastructure only: postgres, redis, kafka, minio (run services local)
	docker compose -f $(COMPOSE_FILE) --profile dev up -d
	@echo ""
	@echo " Infrastructure is up."
	@echo " Run services locally:"
	@echo " make run-task (port 8080)"
	@echo " make run-notif (ports 8081, 50051)"
	@echo " make run-doc (ports 8082, 50052)"
	@echo ""
	@echo " Kafka UI: http://localhost:8090"

infra-down: ## Stop infrastructure containers (keeps volumes)
	docker compose -f $(COMPOSE_FILE) --profile dev down

infra-clean: ## Stop infrastructure and remove volumes (full reset)
	docker compose -f $(COMPOSE_FILE) --profile dev down -v

full-up: ## Start everything in containers (infra + all three services)
	docker compose -f $(COMPOSE_FILE) --profile full up -d --build

full-down: ## Stop all containers
	docker compose -f $(COMPOSE_FILE) --profile full down

monitoring-up: ## Start full stack + Prometheus, Grafana, Loki, Jaeger
	docker compose -f $(COMPOSE_FILE) --profile monitoring up -d --build
	@echo ""
	@echo "  Monitoring stack is up."
	@echo "  Grafana:    http://localhost:3000  (admin / admin)"
	@echo "  Prometheus: http://localhost:9090"
	@echo "  Jaeger:     http://localhost:16686"
	@echo "  Kafka UI:   http://localhost:8090"

monitoring-down: ## Stop monitoring stack
	docker compose -f $(COMPOSE_FILE) --profile monitoring down

logs: ## Follow logs for all running containers
	docker compose -f $(COMPOSE_FILE) logs -f

logs-task: ## Follow task-service logs only
	docker compose -f $(COMPOSE_FILE) logs -f task-service

logs-notif: ## Follow notification-service logs only
	docker compose -f $(COMPOSE_FILE) logs -f notification-service

logs-doc: ## Follow document-service logs only
	docker compose -f $(COMPOSE_FILE) logs -f document-service

# Run locally

run-task: ## Run task-service locally (requires infra-up)
	go run ./cmd/task-service/...

run-notif: ## Run notification-service locally (requires infra-up)
	go run ./cmd/notification-service/...

run-doc: ## Run document-service locally (requires infra-up)
	go run ./cmd/document-service/...

#Build

build: build-task build-notif build-doc ## Build all service binaries into ./bin/

build-task: ## Build task-service binary
	@mkdir -p $(BINARY_DIR)
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY_DIR)/task-service ./cmd/task-service/...
	@echo "bin/task-service"

build-notif: ## Build notification-service binary
	@mkdir -p $(BINARY_DIR)
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY_DIR)/notification-service ./cmd/notification-service/...
	@echo "bin/notification-service"

build-doc: ## Build document-service binary
	@mkdir -p $(BINARY_DIR)
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY_DIR)/document-service ./cmd/document-service/...
	@echo "bin/document-service"

#Proto codegen

proto: ## Generate Go code from .proto files (requires protoc)
	@which protoc > /dev/null 2>&1 || \
		(echo "  ✗ protoc not found. Install: brew install protobuf" && exit 1)
	@which protoc-gen-go > /dev/null 2>&1 || \
		go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@which protoc-gen-go-grpc > /dev/null 2>&1 || \
		go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	protoc \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/notification/notification.proto
	protoc \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/document/document.proto
	@echo "  ✓ Proto generated"

#Quality

lint: ## Run golangci-lint
	@which golangci-lint > /dev/null 2>&1 || (echo "  ✗ golangci-lint not found. Install: brew install golangci-lint" && exit 1)
	golangci-lint run ./...

lint-fix: ## Run golangci-lint with auto-fix where possible
	golangci-lint run --fix ./...

test: ## Run all tests with race detector
	go test -race -count=1 ./...

test-cover: ## Run tests and open HTML coverage report
	go test -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "  ✓ Coverage report: open coverage.html"

test-short: ## Run only short tests (skip integration)
	go test -short -race -count=1 ./...

vet: ## Run go vet
	go vet ./...

#Migrations

migrate-up: ## Apply all pending migrations
	migrate -path ./migrations -database "postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=$(POSTGRES_SSLMODE)" up

migrate-down: ## Rollback last migration
	migrate -path ./migrations -database "postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=$(POSTGRES_SSLMODE)" down 1

migrate-status: ## Show migration status
	migrate -path ./migrations -database "postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=$(POSTGRES_SSLMODE)" version

#Docker

docker-build: ## Build all Docker images locally
	docker build -f deploy/docker/task-service.Dockerfile -t crm-task-service:local .
	docker build -f deploy/docker/notification-service.Dockerfile -t crm-notification-service:local .
	docker build -f deploy/docker/document-service.Dockerfile -t crm-document-service:local .
	@echo "All images built"

#Kubernetes

k8s-up: ## Create kind cluster, load images, deploy everything
	kind create cluster --name $(CLUSTER_NAME) --config deploy/k8s/kind-config.yaml
	$(MAKE) docker-build
	kind load docker-image crm-task-service:local         --name $(CLUSTER_NAME)
	kind load docker-image crm-notification-service:local --name $(CLUSTER_NAME)
	kind load docker-image crm-document-service:local     --name $(CLUSTER_NAME)
	$(MAKE) k8s-apply

k8s-apply: ## Apply all Kubernetes manifests
	kubectl apply -f deploy/k8s/infra/
	kubectl apply -f deploy/k8s/task-service/
	kubectl apply -f deploy/k8s/notification-service/
	kubectl apply -f deploy/k8s/document-service/

k8s-down: ## Delete kind cluster
	kind delete cluster --name $(CLUSTER_NAME)

k8s-status: ## Show pod status in crm namespace
	kubectl get pods -n crm -o wide

#Benchmarks

bench: ## Run k6 ramp-up scenario (0 → 100 VU over 2 min)
	@which k6 > /dev/null 2>&1 || (echo "  ✗ k6 not found. Install: brew install k6" && exit 1)
	k6 run benchmarks/k6/ramp-up.js

bench-spike: ## Run k6 spike scenario (sudden burst of traffic)
	k6 run benchmarks/k6/spike.js

bench-soak: ## Run k6 soak scenario (sustained load for 10 min)
	k6 run benchmarks/k6/soak.js

bench-python: ## Run same k6 scenario against Python FastAPI service
	k6 run -e BASE_URL=http://localhost:8000 benchmarks/k6/ramp-up.js