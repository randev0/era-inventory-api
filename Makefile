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

.PHONY: build-import-excel
build-import-excel: ## Build the Excel importer tool
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o bin/import-excel ./cmd/tools/import_excel

.PHONY: build-import-excel-windows
build-import-excel-windows: ## Build the Excel importer tool for Windows
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/import-excel.exe ./cmd/tools/import_excel

.PHONY: test
test: ## Run unit tests only
	go test ./... -race -count=1 -timeout=60s

.PHONY: test-int-up
test-int-up: ## Spin up test DB
	docker compose -f docker-compose.test.yml up -d --wait

.PHONY: test-int-db
test-int-db: ## Migrate + seed test DB
	TEST_DATABASE_URL=$${TEST_DATABASE_URL:-postgres://era:era@localhost:5432/era_test?sslmode=disable} \
	go run ./cmd/testmigrate && psql "$$TEST_DATABASE_URL" -f db/seeds/001_minimal.sql || true

.PHONY: test-int
test-int: ## Run only integration tests
	$(MAKE) test-int-up
	$(MAKE) test-int-db
	INTEGRATION=1 go test ./... -race -count=1 -timeout=90s -tags=integration

.PHONY: test-int-down
test-int-down: ## Stop test DB
	docker compose -f docker-compose.test.yml down -v

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

.PHONY: dev-up
dev-up: ## Start dev stack
	docker compose up -d

.PHONY: dev-down
dev-down: ## Stop dev stack
	docker compose down -v

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

.PHONY: ci
ci: ## Run full CI pipeline locally
	golangci-lint run --timeout=5m
	go test ./... -race
	TEST_DATABASE_URL="postgres://era:era@localhost:5432/era_test?sslmode=disable" INTEGRATION=1 go test ./... -v -tags=integration

.PHONY: all up migrate seed test openapi-validate logs psql docs metrics

all: migrate seed openapi-validate test
	@echo "__BUILD_OK__"

up:
	docker compose up -d db api

migrate:
	docker compose up migrate

seed:
	- docker compose up seed

test:
	go test ./... -v

openapi-validate:
	- docker run --rm -v ${PWD}:/spec openapitools/openapi-generator-cli validate -i /spec/internal/openapi/openapi.yaml

.PHONY: openapi
openapi: ## Generate OpenAPI docs
	@echo "OpenAPI spec is already generated and served at /openapi.yaml and /docs"

logs:
	docker compose logs -f api

psql:
	docker compose exec db psql -U postgres -d era

docs:
	@echo "Open http://localhost:8080/docs"

.PHONY: import-excel
import-excel: ## Import Excel file (usage: make import-excel FILE=path.xlsx ORG_ID=1 SITE_ID=5)
	@if [ -z "$(FILE)" ] || [ -z "$(ORG_ID)" ] || [ -z "$(SITE_ID)" ]; then \
		echo "Usage: make import-excel FILE=path.xlsx ORG_ID=1 SITE_ID=5"; \
		echo "Example: make import-excel FILE=sample.xlsx ORG_ID=1 SITE_ID=5"; \
		exit 1; \
	fi
	$(MAKE) build-import-excel
	./bin/import-excel --file=$(FILE) --org-id=$(ORG_ID) --site-id=$(SITE_ID) --mapping=configs/mapping/mbip_equipment.yaml

.PHONY: import-excel-help
import-excel-help: ## Show Excel importer help
	$(MAKE) build-import-excel
	./bin/import-excel --help || true

.PHONY: import-excel-test
import-excel-test: ## Test Excel importer with sample data
	@echo "Creating sample Excel file for testing..."
	@echo "This would create a sample Excel file with test data"
	@echo "Usage: make import-excel FILE=sample.xlsx ORG_ID=1 SITE_ID=5"

.PHONY: test-upload
test-upload: ## Test Excel upload endpoint (usage: make test-upload TK=token SITE=site_id FILE=path.xlsx)
	@if [ -z "$(TK)" ] || [ -z "$(SITE)" ] || [ -z "$(FILE)" ]; then \
		echo "Usage: make test-upload TK=token SITE=site_id FILE=path.xlsx"; \
		echo "Example: make test-upload TK=eyJ... SITE=5 FILE=./testdata/sample.xlsx"; \
		echo "Get token with: make login"; \
		exit 1; \
	fi
	@echo "Testing Excel upload endpoint..."
	curl -s -X POST http://localhost:8080/api/v1/imports/excel \
		-H "Authorization: Bearer $(TK)" \
		-F dry_run=true -F site_id=$(SITE) \
		-F file=@$(FILE) | jq

.PHONY: test-upload-dry-run
test-upload-dry-run: ## Test Excel upload with dry run (usage: make test-upload-dry-run TK=token SITE=site_id FILE=path.xlsx)
	@if [ -z "$(TK)" ] || [ -z "$(SITE)" ] || [ -z "$(FILE)" ]; then \
		echo "Usage: make test-upload-dry-run TK=token SITE=site_id FILE=path.xlsx"; \
		echo "Example: make test-upload-dry-run TK=eyJ... SITE=5 FILE=./testdata/sample.xlsx"; \
		exit 1; \
	fi
	@echo "Testing Excel upload with dry run..."
	curl -s -X POST http://localhost:8080/api/v1/imports/excel \
		-H "Authorization: Bearer $(TK)" \
		-F dry_run=true -F site_id=$(SITE) \
		-F file=@$(FILE) | jq

.PHONY: test-upload-real
test-upload-real: ## Test Excel upload with real import (usage: make test-upload-real TK=token SITE=site_id FILE=path.xlsx)
	@if [ -z "$(TK)" ] || [ -z "$(SITE)" ] || [ -z "$(FILE)" ]; then \
		echo "Usage: make test-upload-real TK=token SITE=site_id FILE=path.xlsx"; \
		echo "Example: make test-upload-real TK=eyJ... SITE=5 FILE=./testdata/sample.xlsx"; \
		echo "WARNING: This will actually import data!"; \
		exit 1; \
	fi
	@echo "Testing Excel upload with real import..."
	@echo "WARNING: This will actually import data into the database!"
	@read -p "Are you sure? (y/N): " confirm && [ "$$confirm" = "y" ]
	curl -s -X POST http://localhost:8080/api/v1/imports/excel \
		-H "Authorization: Bearer $(TK)" \
		-F site_id=$(SITE) \
		-F file=@$(FILE) | jq

.PHONY: login
login: ## Get authentication token (usage: make login EMAIL=email PASSWORD=password)
	@if [ -z "$(EMAIL)" ] || [ -z "$(PASSWORD)" ]; then \
		echo "Usage: make login EMAIL=email PASSWORD=password"; \
		echo "Example: make login EMAIL=admin@example.com PASSWORD=password"; \
		exit 1; \
	fi
	@echo "Getting authentication token..."
	@TOKEN=$$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
		-H "Content-Type: application/json" \
		-d "{\"email\":\"$(EMAIL)\",\"password\":\"$(PASSWORD)\"}" | jq -r '.token'); \
	if [ "$$TOKEN" = "null" ] || [ -z "$$TOKEN" ]; then \
		echo "Login failed. Check credentials and server status."; \
		exit 1; \
	fi; \
	echo "Token: $$TOKEN"; \
	echo "Use with: make test-upload TK=$$TOKEN SITE=5 FILE=./testdata/sample.xlsx"
