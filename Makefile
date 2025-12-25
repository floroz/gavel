.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: up
up: ## Start infrastructure services
	docker-compose up -d

.PHONY: down
down: ## Stop infrastructure services
	docker-compose down

.PHONY: migrate-up
migrate-up: ## Run database migrations
	goose -dir migrations postgres "host=localhost port=5432 user=user password=password dbname=auction_db sslmode=disable" up

.PHONY: migrate-down
migrate-down: ## Rollback last database migration
	goose -dir migrations postgres "host=localhost port=5432 user=user password=password dbname=auction_db sslmode=disable" down

.PHONY: migrate-status
migrate-status: ## Check migration status
	goose -dir migrations postgres "host=localhost port=5432 user=user password=password dbname=auction_db sslmode=disable" status

.PHONY: migrate-create
migrate-create: ## Create a new migration file (usage: make migrate-create NAME=add_users_table)
	goose -dir migrations create $(NAME) sql

.PHONY: run-bid-service
run-bid-service: ## Run the Bid Service (Producer)
	go run cmd/bid-service/main.go

.PHONY: run-bid-worker
run-bid-worker: ## Run the Bid Worker (Outbox Relay)
	go run cmd/bid-worker/main.go

.PHONY: run-stats-service
run-stats-service: ## Run the User Stats Service (Consumer)
	go run cmd/user-stats-service/main.go

.PHONY: run-all
run-all: ## Run all services concurrently
	@echo "Starting all services... Press Ctrl+C to stop."
	@trap 'kill 0' INT; \
	go run cmd/bid-service/main.go 2>&1 | sed "s/^/[BID-API] /" & \
	go run cmd/bid-worker/main.go 2>&1 | sed "s/^/[WORKER]  /" & \
	go run cmd/user-stats-service/main.go 2>&1 | sed "s/^/[STATS]   /" & \
	wait

.PHONY: test
test: ## Run tests
	go test -v ./...

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
	@if ! command -v protoc-gen-go-grpc >/dev/null 2>&1; then \
		echo "Installing protoc-gen-go-grpc..."; \
		go install google.golang.org/protobuf/cmd/protoc-gen-go-grpc@latest || \
		(echo "Note: protoc-gen-go-grpc not installed (optional for gRPC)"; \
		 echo "  Install with: go install google.golang.org/protobuf/cmd/protoc-gen-go-grpc@latest"); \
	else \
		echo "protoc-gen-go-grpc already installed"; \
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
	@mkdir -p internal/pb
	tools/protoc \
		--go_out=. \
		--go_opt=module=github.com/floroz/auction-system \
		--proto_path=api/proto \
		--proto_path=tools/include \
		api/proto/events.proto
	@echo "Protobuf code generated in internal/pb/"

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
	goimports -w -local github.com/floroz/auction-system .
