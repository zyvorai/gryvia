# KubeFabric Makefile
# Automates building, testing, and deploying all components

.PHONY: all build test clean docker-build docker-push deploy install help

# Variables
REGISTRY ?= ghcr.io/ssahani
VERSION ?= $(shell git describe --tags --always --dirty)
OPERATORS := gpu-operator ai-operator storage-operator network-operator quota-operator

# Colors for output
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
RESET  := $(shell tput -Txterm sgr0)

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  ${GREEN}%-20s${RESET} %s\n", $$1, $$2}' $(MAKEFILE_LIST)

all: build test ## Build and test all components

## Build targets
build: build-operators build-cli build-web-ui build-api-gateway ## Build all components

build-operators: ## Build all operators
	@echo "${GREEN}Building operators...${RESET}"
	@for op in $(OPERATORS); do \
		echo "${YELLOW}Building $$op...${RESET}"; \
		(cd operators/$$op && go build -o bin/manager main.go); \
	done

build-cli: ## Build CLI tool
	@echo "${GREEN}Building CLI...${RESET}"
	cd cli && cargo build --release

build-web-ui: ## Build Web UI
	@echo "${GREEN}Building Web UI...${RESET}"
	cd web-ui && npm ci && npm run build

build-api-gateway: ## Build API Gateway
	@echo "${GREEN}Building API Gateway...${RESET}"
	cd services/api-gateway && pip install -r requirements.txt

## Test targets
test: test-operators test-cli test-api-gateway ## Run all tests

test-operators: ## Test all operators
	@echo "${GREEN}Testing operators...${RESET}"
	@for op in $(OPERATORS); do \
		echo "${YELLOW}Testing $$op...${RESET}"; \
		(cd operators/$$op && go test -v ./...); \
	done

test-cli: ## Test CLI
	@echo "${GREEN}Testing CLI...${RESET}"
	cd cli && cargo test

test-api-gateway: ## Test API Gateway
	@echo "${GREEN}Testing API Gateway...${RESET}"
	cd services/api-gateway && pytest

## Docker targets
docker-build: docker-build-operators docker-build-cli docker-build-web-ui docker-build-api-gateway ## Build all Docker images

docker-build-operators: ## Build operator Docker images
	@echo "${GREEN}Building operator images...${RESET}"
	@for op in $(OPERATORS); do \
		echo "${YELLOW}Building $$op image...${RESET}"; \
		docker build -t $(REGISTRY)/kubefabric-$$op:$(VERSION) operators/$$op; \
		docker tag $(REGISTRY)/kubefabric-$$op:$(VERSION) $(REGISTRY)/kubefabric-$$op:latest; \
	done

docker-build-cli: ## Build CLI Docker image
	@echo "${GREEN}Building CLI image...${RESET}"
	docker build -t $(REGISTRY)/kubefabric-cli:$(VERSION) cli
	docker tag $(REGISTRY)/kubefabric-cli:$(VERSION) $(REGISTRY)/kubefabric-cli:latest

docker-build-web-ui: ## Build Web UI Docker image
	@echo "${GREEN}Building Web UI image...${RESET}"
	docker build -t $(REGISTRY)/kubefabric-ui:$(VERSION) -f docker/Dockerfile.ui .
	docker tag $(REGISTRY)/kubefabric-ui:$(VERSION) $(REGISTRY)/kubefabric-ui:latest

docker-build-api-gateway: ## Build API Gateway Docker image
	@echo "${GREEN}Building API Gateway image...${RESET}"
	docker build -t $(REGISTRY)/kubefabric-api-gateway:$(VERSION) services/api-gateway
	docker tag $(REGISTRY)/kubefabric-api-gateway:$(VERSION) $(REGISTRY)/kubefabric-api-gateway:latest

docker-push: ## Push all Docker images
	@echo "${GREEN}Pushing Docker images...${RESET}"
	@for op in $(OPERATORS); do \
		docker push $(REGISTRY)/kubefabric-$$op:$(VERSION); \
		docker push $(REGISTRY)/kubefabric-$$op:latest; \
	done
	docker push $(REGISTRY)/kubefabric-cli:$(VERSION)
	docker push $(REGISTRY)/kubefabric-cli:latest
	docker push $(REGISTRY)/kubefabric-ui:$(VERSION)
	docker push $(REGISTRY)/kubefabric-ui:latest
	docker push $(REGISTRY)/kubefabric-api-gateway:$(VERSION)
	docker push $(REGISTRY)/kubefabric-api-gateway:latest

## Deployment targets
deploy: deploy-crds deploy-operators deploy-web-ui deploy-api-gateway ## Deploy all components to Kubernetes

deploy-crds: ## Deploy CRDs
	@echo "${GREEN}Deploying CRDs...${RESET}"
	kubectl apply -f crds/

deploy-operators: ## Deploy operators
	@echo "${GREEN}Deploying operators...${RESET}"
	@for op in $(OPERATORS); do \
		kubectl apply -f operators/$$op/config/deployment.yaml; \
	done

deploy-web-ui: ## Deploy Web UI
	@echo "${GREEN}Deploying Web UI...${RESET}"
	kubectl apply -f manifests/deploy/ui-deployment.yaml

deploy-api-gateway: ## Deploy API Gateway
	@echo "${GREEN}Deploying API Gateway...${RESET}"
	kubectl apply -f manifests/deploy/api-gateway-deployment.yaml

deploy-monitoring: ## Deploy monitoring stack
	@echo "${GREEN}Deploying monitoring...${RESET}"
	kubectl apply -f monitoring/

install: ## Install using Helm
	@echo "${GREEN}Installing KubeFabric with Helm...${RESET}"
	helm install kubefabric helm/kubefabric-core -n kubefabric --create-namespace

uninstall: ## Uninstall using Helm
	@echo "${GREEN}Uninstalling KubeFabric...${RESET}"
	helm uninstall kubefabric -n kubefabric

## Development targets
dev-setup: ## Set up development environment
	@echo "${GREEN}Setting up development environment...${RESET}"
	@echo "Installing Go dependencies..."
	@for op in $(OPERATORS); do \
		(cd operators/$$op && go mod download); \
	done
	@echo "Installing Rust toolchain..."
	rustup update stable
	@echo "Installing Node.js dependencies..."
	cd web-ui && npm install
	@echo "Installing Python dependencies..."
	cd services/api-gateway && pip install -r requirements.txt
	@echo "${GREEN}Development environment ready!${RESET}"

fmt: ## Format all code
	@echo "${GREEN}Formatting code...${RESET}"
	@for op in $(OPERATORS); do \
		(cd operators/$$op && go fmt ./...); \
	done
	cd cli && cargo fmt
	cd web-ui && npm run format || true
	cd services/api-gateway && black main.py

lint: ## Lint all code
	@echo "${GREEN}Linting code...${RESET}"
	@for op in $(OPERATORS); do \
		(cd operators/$$op && golangci-lint run); \
	done
	cd cli && cargo clippy
	cd web-ui && npm run lint || true
	cd services/api-gateway && flake8 main.py

clean: ## Clean build artifacts
	@echo "${GREEN}Cleaning build artifacts...${RESET}"
	@for op in $(OPERATORS); do \
		rm -rf operators/$$op/bin; \
	done
	cd cli && cargo clean
	rm -rf web-ui/dist web-ui/node_modules
	find . -type d -name __pycache__ -exec rm -rf {} +

## Documentation targets
docs: ## Generate documentation
	@echo "${GREEN}Generating documentation...${RESET}"
	@for op in $(OPERATORS); do \
		(cd operators/$$op && go doc -all > REFERENCE.md); \
	done

## Release targets
release: test docker-build docker-push ## Create a release (test, build, push)
	@echo "${GREEN}Release $(VERSION) complete!${RESET}"

version: ## Show current version
	@echo "Version: $(VERSION)"

.DEFAULT_GOAL := help
