.PHONY: help build build-oss build-enterprise push clean

# Configuration
REGISTRY ?= your-registry.example.com
OBOT_REPO ?= obot
MCP_REPO ?= obot
MCP_TAG ?= latest
CA_CERT ?= enterprise-ca.crt
PLATFORM ?=

help: ## Show this help message
	@echo "Obot Enterprise CA Certificate Package"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'
	@echo ""
	@echo "Configuration (via environment variables):"
	@echo "  REGISTRY    Container registry (default: $(REGISTRY))"
	@echo "  OBOT_REPO   Obot repository name (default: $(OBOT_REPO))"
	@echo "  MCP_REPO    MCP repository name (default: $(MCP_REPO))"
	@echo "  MCP_TAG     MCP images tag (default: $(MCP_TAG))"
	@echo "  CA_CERT     CA certificate file (default: $(CA_CERT))"
	@echo "  PLATFORM    Target platform (e.g., linux/amd64, linux/arm64)"
	@echo ""
	@echo "Examples:"
	@echo "  make build-oss REGISTRY=myregistry.com"
	@echo "  make build-enterprise REGISTRY=myregistry.com"
	@echo "  make build-oss PLATFORM=linux/amd64"

check-ca: ## Check if CA certificate exists
	@if [ ! -f "$(CA_CERT)" ]; then \
		echo "Error: CA certificate not found: $(CA_CERT)"; \
		echo "Please place your enterprise CA certificate in this directory"; \
		exit 1; \
	fi
	@echo "✓ CA certificate found: $(CA_CERT)"

build: build-oss ## Build all images (OSS edition by default)

build-oss: check-ca ## Build OSS edition images
	@./build.sh -c $(CA_CERT) -r $(REGISTRY) --obot-repo $(OBOT_REPO) --mcp-repo $(MCP_REPO) -m $(MCP_TAG) --oss $(if $(PLATFORM),--platform $(PLATFORM))

build-enterprise: check-ca ## Build Enterprise edition images
	@./build.sh -c $(CA_CERT) -r $(REGISTRY) --obot-repo $(OBOT_REPO) --mcp-repo $(MCP_REPO) -m $(MCP_TAG) --enterprise $(if $(PLATFORM),--platform $(PLATFORM))

build-obot-oss: check-ca ## Build only OSS Obot image
	@./build.sh -c $(CA_CERT) -r $(REGISTRY) --obot-repo $(OBOT_REPO) --oss --obot-only $(if $(PLATFORM),--platform $(PLATFORM))

build-obot-enterprise: check-ca ## Build only Enterprise Obot image
	@./build.sh -c $(CA_CERT) -r $(REGISTRY) --obot-repo $(OBOT_REPO) --enterprise --obot-only $(if $(PLATFORM),--platform $(PLATFORM))

build-mcp: check-ca ## Build only MCP images
	@./build.sh -c $(CA_CERT) -r $(REGISTRY) --mcp-repo $(MCP_REPO) -m $(MCP_TAG) --mcp-only $(if $(PLATFORM),--platform $(PLATFORM))

push-oss: check-ca ## Build and push OSS edition images
	@./build.sh -c $(CA_CERT) -r $(REGISTRY) --obot-repo $(OBOT_REPO) --mcp-repo $(MCP_REPO) -m $(MCP_TAG) --oss --push $(if $(PLATFORM),--platform $(PLATFORM))

push-enterprise: check-ca ## Build and push Enterprise edition images
	@./build.sh -c $(CA_CERT) -r $(REGISTRY) --obot-repo $(OBOT_REPO) --mcp-repo $(MCP_REPO) -m $(MCP_TAG) --enterprise --push $(if $(PLATFORM),--platform $(PLATFORM))

clean: ## Clean generated files
	@rm -f values-custom.yaml values-*.yaml *.log
	@rm -rf keycloak-auth-provider auth-providers-common
	@echo "Cleaned generated files"
