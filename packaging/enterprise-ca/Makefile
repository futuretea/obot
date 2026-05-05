.PHONY: help check-ca build build-obot build-mcp build-all-mcp push push-all

REGISTRY ?= harbor.futuretea.me
OBOT_REPO ?= obot
MCP_REPO ?= obot
PACKAGE_VERSION ?= latest
MCP_TAG ?=
OBOT_TAG ?=
CA_CERT ?= enterprise-ca.crt
CA_WRAP_TOOL ?=
PLATFORM ?=

CA_WRAP_TOOL_ARG = $(if $(CA_WRAP_TOOL),--ca-wrap-tool $(CA_WRAP_TOOL),)
PLATFORM_ARG = $(if $(PLATFORM),--platform $(PLATFORM),)
MCP_TAG_ARG = $(if $(MCP_TAG),--mcp-tag $(MCP_TAG),)
COMMON_ARGS = -c $(CA_CERT) $(CA_WRAP_TOOL_ARG) -r $(REGISTRY) --obot-repo $(OBOT_REPO) --mcp-repo $(MCP_REPO) --package-version $(PACKAGE_VERSION) $(MCP_TAG_ARG) $(PLATFORM_ARG)
OBOT_TAG_ARG = $(if $(OBOT_TAG),--tag $(OBOT_TAG),)

help: ## Show this help message
	@echo "Obot enterprise CA image packaging"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-24s %s\n", $$1, $$2}'
	@echo ""
	@echo "Configuration:"
	@echo "  REGISTRY          Container registry (default: $(REGISTRY))"
	@echo "  OBOT_REPO         Obot image repository namespace (default: $(OBOT_REPO))"
	@echo "  MCP_REPO          MCP image repository namespace (default: $(MCP_REPO))"
	@echo "  PACKAGE_VERSION   Obot base image tag (default: $(PACKAGE_VERSION))"
	@echo "  MCP_TAG           MCP image tag override (default: <source-tag>-ca)"
	@echo "  OBOT_TAG          Obot image tag override (default: <source-tag>-ca)"
	@echo "  CA_CERT           CA certificate path (default: $(CA_CERT))"
	@echo "  CA_WRAP_TOOL      Generic CA wrapper path (default: ./image-ca-wrap/image-ca-wrap.sh)"
	@echo "  PLATFORM          Target platform, for example linux/amd64"

check-ca: ## Verify that the enterprise CA certificate exists
	@./build.sh -c $(CA_CERT) --check-only

build: ## Build OSS Obot and core runtime images
	@./build.sh $(COMMON_ARGS) $(OBOT_TAG_ARG)

build-obot: ## Build only the OSS Obot image
	@./build.sh $(COMMON_ARGS) $(OBOT_TAG_ARG) --obot-only

build-mcp: ## Build only core runtime images
	@./build.sh $(COMMON_ARGS) --mcp-only

build-all-mcp: ## Build core runtime images and optional catalog tool images
	@./build.sh $(COMMON_ARGS) --mcp-only --catalog-tools

push: ## Build and push OSS Obot and core runtime images
	@./build.sh $(COMMON_ARGS) $(OBOT_TAG_ARG) --push

push-all: ## Build and push OSS Obot, core runtime images, and optional catalog tool images
	@./build.sh $(COMMON_ARGS) $(OBOT_TAG_ARG) --catalog-tools --push
