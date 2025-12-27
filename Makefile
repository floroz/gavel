.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: up
up: ## Start infrastructure services
	docker compose up -d

.PHONY: down
down: ## Stop infrastructure services
	docker compose down

.PHONY: migrate-bids-up
migrate-bids-up: ## Run Bid Service migrations
	goose -dir services/bid-service/migrations postgres "host=localhost port=5432 user=user password=password dbname=bid_db sslmode=disable" up

.PHONY: migrate-bids-down
migrate-bids-down: ## Rollback Bid Service migration
	goose -dir services/bid-service/migrations postgres "host=localhost port=5432 user=user password=password dbname=bid_db sslmode=disable" down

.PHONY: migrate-bids-create
migrate-bids-create: ## Create a new Bid Service migration (usage: make migrate-bids-create NAME=add_bids_table)
	goose -dir services/bid-service/migrations create $(NAME) sql

.PHONY: migrate-stats-up
migrate-stats-up: ## Run User Stats Service migrations
	goose -dir services/user-stats-service/migrations postgres "host=localhost port=5433 user=user password=password dbname=stats_db sslmode=disable" up

.PHONY: migrate-stats-down
migrate-stats-down: ## Rollback User Stats Service migration
	goose -dir services/user-stats-service/migrations postgres "host=localhost port=5433 user=user password=password dbname=stats_db sslmode=disable" down

.PHONY: migrate-stats-create
migrate-stats-create: ## Create a new User Stats Service migration
	goose -dir services/user-stats-service/migrations create $(NAME) sql

.PHONY: migrate-up-all
migrate-up-all: migrate-bids-up migrate-stats-up ## Run all migrations

.PHONY: migrate-down-all
migrate-down-all: migrate-bids-down migrate-stats-down ## Rollback all migrations

.PHONY: run-bid-service
run-bid-service: ## Run the Bid Service (Producer)
	go run services/bid-service/cmd/api/main.go

.PHONY: run-bid-worker
run-bid-worker: ## Run the Bid Worker (Outbox Relay)
	go run services/bid-service/cmd/worker/main.go

.PHONY: run-stats-service
run-stats-service: ## Run the User Stats Service (Consumer)
	go run services/user-stats-service/cmd/worker/main.go

.PHONY: run-stats-api
run-stats-api: ## Run the User Stats Service API (Read Side)
	go run services/user-stats-service/cmd/api/main.go

.PHONY: run-all
run-all: ## Run all services concurrently
	@echo "Starting all services... Press Ctrl+C to stop."
	@trap 'kill 0' INT; \
	go run services/bid-service/cmd/api/main.go 2>&1 | sed "s/^/[BID-API] /" & \
	go run services/bid-service/cmd/worker/main.go 2>&1 | sed "s/^/[WORKER]  /" & \
	go run services/user-stats-service/cmd/worker/main.go 2>&1 | sed "s/^/[STATS]   /" & \
	wait

.PHONY: build-bid-service
build-bid-service: ## Build Bid Service Docker image
	docker build -f services/bid-service/Dockerfile -t bid-service .

.PHONY: build-stats-service
build-stats-service: ## Build User Stats Service Docker image
	docker build -f services/user-stats-service/Dockerfile -t user-stats-service .

.PHONY: build-all
build-all: build-bid-service build-stats-service ## Build all Docker images
	@echo "All images built successfully."

.PHONY: test-unit
test-unit: ## Run unit tests
	go test -v ./...

.PHONY: test-integration
test-integration: ## Run integration tests
	go test -v -tags integration ./...

.PHONY: test
test: test-unit test-integration ## Run all tests

.PHONY: tidy
tidy: ## Tidy go modules
	go mod tidy

.PHONY: install-protoc
install-protoc: ## Install protoc compiler locally (project-only)
	@if [ ! -f tools/protoc ]; then \
		echo "Downloading protoc for macOS..."; \
		mkdir -p tools; \
		ARCH=$$(uname -m); \
		if [ "$$ARCH" = "arm64" ]; then \
			PROTOC_ARCH="osx-aarch_64"; \
		else \
			PROTOC_ARCH="osx-x86_64"; \
		fi; \
		curl -L --fail --show-error --cert-status https://github.com/protocolbuffers/protobuf/releases/download/v27.1/protoc-27.1-$$PROTOC_ARCH.zip -o tools/protoc.zip || \
		curl -L --fail --show-error --insecure https://github.com/protocolbuffers/protobuf/releases/download/v27.1/protoc-27.1-$$PROTOC_ARCH.zip -o tools/protoc.zip; \
		unzip -o tools/protoc.zip -d tools; \
		chmod +x tools/bin/protoc; \
		mv tools/bin/protoc tools/protoc; \
		rm -f tools/protoc.zip; \
		echo "protoc installed to tools/protoc"; \
	else \
		echo "protoc already installed"; \
	fi

.PHONY: install-protoc-plugins
install-protoc-plugins: ## Install Go protobuf plugins
	@if ! command -v protoc-gen-go >/dev/null 2>&1; then \
		echo "Installing protoc-gen-go..."; \
		go install google.golang.org/protobuf/cmd/protoc-gen-go@latest || \
		(echo "Error: Failed to install protoc-gen-go. Please run manually:"; \
		 echo "  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; \
		 exit 1); \
	else \
		echo "protoc-gen-go already installed"; \
	fi
	@if ! command -v protoc-gen-connect-go >/dev/null 2>&1; then \
		echo "Installing protoc-gen-connect-go..."; \
		go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest || \
		(echo "Error: Failed to install protoc-gen-connect-go."; \
		 echo "  go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest"; \
		 exit 1); \
	else \
		echo "protoc-gen-connect-go already installed"; \
	fi

.PHONY: proto-gen
proto-gen: install-protoc ## Generate Go code from protobuf files
	@if [ ! -f tools/protoc ]; then \
		echo "Error: protoc not found. Run 'make install-protoc' first."; \
		exit 1; \
		fi
	@if ! command -v protoc-gen-go >/dev/null 2>&1; then \
		echo "Error: protoc-gen-go not found in PATH."; \
		echo "Please install it with: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; \
		echo "Make sure $$HOME/go/bin (or $$GOPATH/bin) is in your PATH."; \
		exit 1; \
	fi
	@if ! command -v protoc-gen-connect-go >/dev/null 2>&1; then \
		echo "Error: protoc-gen-connect-go not found in PATH."; \
		echo "Please install it with: go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest"; \
		exit 1; \
	fi
	@mkdir -p pkg/proto
	tools/protoc \
		--go_out=. \
		--go_opt=module=github.com/floroz/gavel \
		--connect-go_out=. \
		--connect-go_opt=module=github.com/floroz/gavel \
		--proto_path=api/proto \
		--proto_path=tools/include \
		api/proto/events.proto \
		api/proto/bids/v1/bid_service.proto \
		api/proto/userstats/v1/user_stats_service.proto
	@echo "Protobuf code generated in pkg/proto/"

.PHONY: proto-gen-ts
proto-gen-ts: install-protoc ## Generate TypeScript code for frontend
	@mkdir -p frontend/proto
	@cd frontend && npm install
	tools/protoc \
		--plugin=protoc-gen-es=./frontend/node_modules/.bin/protoc-gen-es \
		--plugin=protoc-gen-connect-es=./frontend/node_modules/.bin/protoc-gen-connect-es \
		--es_out=frontend/proto \
		--es_opt=target=ts \
		--connect-es_out=frontend/proto \
		--connect-es_opt=target=ts \
		--proto_path=api/proto \
		--proto_path=tools/include \
		api/proto/bids/v1/bid_service.proto \
		api/proto/userstats/v1/user_stats_service.proto
	@echo "TypeScript code generated in frontend/proto/"

.PHONY: lint
lint: ## Run linter
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	golangci-lint run

.PHONY: fmt
fmt: ## Format code
	go fmt ./...
	@if ! command -v goimports >/dev/null 2>&1; then \
		echo "Installing goimports..."; \
		go install golang.org/x/tools/cmd/goimports@latest; \
	fi
	goimports -w -local github.com/floroz/gavel .
