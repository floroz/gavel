.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: cluster
cluster: ## Create/Update local Kind cluster + Registry (ctlptl)
	ctlptl apply -f deploy/ctlptl.yaml

.PHONY: cluster-delete
cluster-delete: ## Destroy local Kind cluster + Registry
	ctlptl delete -f deploy/ctlptl.yaml

.PHONY: dev
dev: ## Start full development environment (Kubernetes + Tilt)
	tilt up

.PHONY: clean
clean: ## Tear down development environment (Kubernetes + Tilt)
	tilt down

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
		api/proto/userstats/v1/user_stats_service.proto \
		api/proto/auth/v1/auth_service.proto
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
