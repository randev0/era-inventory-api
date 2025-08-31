# Variables
IMAGE_NAME ?= era-inventory-api
REGISTRY ?= ghcr.io
FULL_IMAGE_NAME = $(REGISTRY)/$(IMAGE_NAME)
VERSION ?= $(shell git describe --tags --always --dirty)
GOOS ?= linux
GOARCH ?= amd64

# Default target
.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the Go binary locally
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o bin/api ./cmd/api

.PHONY: build-windows
build-windows: ## Build the Go binary for Windows
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/api.exe ./cmd/api

.PHONY: test
test: ## Run tests
	go test -v ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html

.PHONY: docker-build
docker-build: ## Build Docker image
	docker build -t $(IMAGE_NAME):$(VERSION) .
	docker tag $(IMAGE_NAME):$(VERSION) $(IMAGE_NAME):latest

.PHONY: docker-run
docker-run: ## Run Docker container locally
	docker run -p 8080:8080 --env-file .env $(IMAGE_NAME):latest

.PHONY: docker-compose-up
docker-compose-up: ## Start all services with Docker Compose
	docker-compose up -d

.PHONY: docker-compose-down
docker-compose-down: ## Stop all services with Docker Compose
	docker-compose down

.PHONY: docker-compose-logs
docker-compose-logs: ## Show Docker Compose logs
	docker-compose logs -f

.PHONY: docker-push
docker-push: ## Push Docker image to registry
	docker tag $(IMAGE_NAME):$(VERSION) $(FULL_IMAGE_NAME):$(VERSION)
	docker tag $(IMAGE_NAME):latest $(FULL_IMAGE_NAME):latest
	docker push $(FULL_IMAGE_NAME):$(VERSION)
	docker push $(FULL_IMAGE_NAME):latest

.PHONY: docker-push-ghcr
docker-push-ghcr: ## Push to GitHub Container Registry
	@echo "Logging in to GitHub Container Registry..."
	@echo "Please ensure you have logged in with: docker login ghcr.io -u USERNAME -p TOKEN"
	docker tag $(IMAGE_NAME):$(VERSION) $(FULL_IMAGE_NAME):$(VERSION)
	docker tag $(IMAGE_NAME):latest $(FULL_IMAGE_NAME):latest
	docker push $(FULL_IMAGE_NAME):$(VERSION)
	docker push $(FULL_IMAGE_NAME):latest

.PHONY: docker-push-dockerhub
docker-push-dockerhub: ## Push to Docker Hub
	@echo "Logging in to Docker Hub..."
	@echo "Please ensure you have logged in with: docker login"
	docker tag $(IMAGE_NAME):$(VERSION) $(IMAGE_NAME):$(VERSION)
	docker tag $(IMAGE_NAME):latest $(IMAGE_NAME):latest
	docker push $(IMAGE_NAME):$(VERSION)
	docker push $(IMAGE_NAME):latest

.PHONY: lint
lint: ## Run linting
	golangci-lint run

.PHONY: fmt
fmt: ## Format code
	go fmt ./...

.PHONY: mod-tidy
mod-tidy: ## Tidy Go modules
	go mod tidy
	go mod verify

.PHONY: security-scan
security-scan: ## Run security scan on Docker image
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
		-v $(PWD):/workspace \
		aquasec/trivy image $(IMAGE_NAME):$(VERSION)

.PHONY: all
all: clean test build docker-build ## Run all: clean, test, build, and docker-build

.PHONY: release
release: clean test docker-build docker-push ## Full release process
