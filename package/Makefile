.PHONY: help build push deploy test verify clean

# Configuration
REGISTRY ?= your-registry.example.com
OBOT_REPO ?= obot
MCP_REPO ?= obot
OBOT_TAG ?= enterprise
MCP_TAG ?= latest
NAMESPACE ?= obot-system
CA_CERT ?= enterprise-ca.crt

help: ## Show this help message
	@echo "Obot Enterprise CA Certificate Package"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'
	@echo ""
	@echo "Configuration (via environment variables):"
	@echo "  REGISTRY    Container registry (default: $(REGISTRY))"
	@echo "  OBOT_REPO   Obot repository name (default: $(OBOT_REPO))"
	@echo "  MCP_REPO    MCP repository name (default: $(MCP_REPO))"
	@echo "  OBOT_TAG    Obot image tag (default: $(OBOT_TAG))"
	@echo "  MCP_TAG     MCP images tag (default: $(MCP_TAG))"
	@echo "  CA_CERT     CA certificate file (default: $(CA_CERT))"
	@echo ""
	@echo "Examples:"
	@echo "  make build REGISTRY=myregistry.com"
	@echo "  make deploy NAMESPACE=prod"

check-ca: ## Check if CA certificate exists
	@if [ ! -f "$(CA_CERT)" ]; then \
		echo "Error: CA certificate not found: $(CA_CERT)"; \
		echo "Please place your enterprise CA certificate in this directory"; \
		exit 1; \
	fi
	@echo "✓ CA certificate found: $(CA_CERT)"

build: check-ca ## Build all images (Obot + MCP servers)
	@./build.sh -c $(CA_CERT) -r $(REGISTRY) --obot-repo $(OBOT_REPO) --mcp-repo $(MCP_REPO) -t $(OBOT_TAG) -m $(MCP_TAG)

build-obot: check-ca ## Build only Obot image
	@./build.sh -c $(CA_CERT) -r $(REGISTRY) --obot-repo $(OBOT_REPO) -t $(OBOT_TAG) --obot-only

build-mcp: check-ca ## Build only MCP images
	@./build.sh -c $(CA_CERT) -r $(REGISTRY) --mcp-repo $(MCP_REPO) -m $(MCP_TAG) --mcp-only

push: ## Push all images to registry
	@./build.sh -c $(CA_CERT) -r $(REGISTRY) --obot-repo $(OBOT_REPO) --mcp-repo $(MCP_REPO) -t $(OBOT_TAG) -m $(MCP_TAG) --push

values: ## Generate Helm values file
	@./deploy.sh --registry $(REGISTRY) --obot-repo $(OBOT_REPO) --mcp-repo $(MCP_REPO) --obot-tag $(OBOT_TAG) --mcp-tag $(MCP_TAG) generate-values

deploy: values ## Deploy Obot to Kubernetes
	@./deploy.sh -n $(NAMESPACE) install

upgrade: ## Upgrade existing Obot deployment
	@./deploy.sh -n $(NAMESPACE) upgrade

verify: ## Verify Obot deployment
	@./deploy.sh -n $(NAMESPACE) verify

test: ## Run CA certificate validation tests
	@./test.sh -n $(NAMESPACE)

test-docker: ## Run CA certificate validation tests (Docker)
	@./test.sh --docker

clean: ## Clean generated files
	@rm -f values-custom.yaml values-*.yaml *.log
	@echo "Cleaned generated files"

all: build push deploy verify ## Build, push, deploy, and verify
