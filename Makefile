# Makefile for Go project

default: build

DOCKER_IMAGE ?= obot:local
DOCKER_REGISTRY ?= ghcr.io
DOCKER_NAMESPACE ?= futuretea
DOCKER_IMAGE_NAME ?= obot
DOCKER_BASE_IMAGE ?= $(DOCKER_REGISTRY)/$(DOCKER_NAMESPACE)/$(DOCKER_IMAGE_NAME)/base:latest
DOCKER_RUNTIME_BASE_IMAGE ?= $(DOCKER_REGISTRY)/$(DOCKER_NAMESPACE)/$(DOCKER_IMAGE_NAME)-runtime-base:latest
DOCKER_LOCAL_RUNTIME_BASE_IMAGE ?= obot/runtime-base:local
DOCKER_PLATFORM ?=
DOCKER_BUILD := docker build$(if $(DOCKER_PLATFORM), --platform $(DOCKER_PLATFORM),)

# All target
all: ui
	$(MAKE) build

ui: ui-user-install
	cd ui/user && \
	pnpm run build && \
	BUILD=node pnpm run build

ui-user-install:
	cd ui/user && \
	pnpm install

ui-user: ui-user-install
	cd ui/user && \
	pnpm run build

ui-user-node: ui-user-install
	cd ui/user && \
	BUILD=node pnpm run build

clean:
	rm -rf ui/admin/build
	rm -rf ui/user/build

serve-docs:
	cd docs && \
	npm install && \
	npm run start

# Build the project

GIT_TAG := $(shell git describe --tags --exact-match 2>/dev/null | xargs -I {} echo -X 'github.com/obot-platform/obot/pkg/version.Tag={}')
GO_LD_FLAGS := "-s -w $(GIT_TAG)"
GO_BUILD_MOD_FLAG :=
ifneq ($(wildcard vendor/modules.txt),)
GO_BUILD_MOD_FLAG := -mod=vendor
endif
build:
	go build $(GO_BUILD_MOD_FLAG) -ldflags=$(GO_LD_FLAGS) -o bin/obot .

docker-build:
	$(DOCKER_BUILD) \
		--build-arg BASE_IMAGE=$(DOCKER_BASE_IMAGE) \
		--build-arg RUNTIME_BASE_IMAGE=$(DOCKER_RUNTIME_BASE_IMAGE) \
		-t $(DOCKER_IMAGE) .

docker-build-fast:
	$(DOCKER_BUILD) \
		--build-arg BASE_IMAGE=$(DOCKER_BASE_IMAGE) \
		--build-arg RUNTIME_BASE_IMAGE=$(DOCKER_RUNTIME_BASE_IMAGE) \
		-t $(DOCKER_IMAGE) .

docker-build-full:
	$(DOCKER_BUILD) --build-arg RUNTIME_BASE_IMAGE=runtime-base -t $(DOCKER_IMAGE) .

docker-build-runtime-base:
	$(DOCKER_BUILD) --target runtime-base -t $(DOCKER_LOCAL_RUNTIME_BASE_IMAGE) .

docker-build-local-runtime-base: docker-build-runtime-base
	$(MAKE) docker-build DOCKER_RUNTIME_BASE_IMAGE=$(DOCKER_LOCAL_RUNTIME_BASE_IMAGE) DOCKER_PLATFORM=$(DOCKER_PLATFORM)

dev:
	./tools/dev.sh $(ARGS)

dev-open: ARGS=--open-uis
dev-open: dev

otel-jaeger-up:
	docker compose -f tools/jaeger-compose.yaml up -d

otel-jaeger-down:
	docker compose -f tools/jaeger-compose.yaml down

otel-jaeger-logs:
	docker compose -f tools/jaeger-compose.yaml logs -f

# Lint the project
lint: lint-go

tidy:
	go mod tidy

GOLANGCI_LINT_VERSION ?= v2.11.4
setup-env:
	if ! command -v golangci-lint &> /dev/null; then \
  		echo "Could not find golangci-lint, installing version $(GOLANGCI_LINT_VERSION)."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION); \
	fi

lint-go: setup-env
	golangci-lint run

generate:
	go generate

test:
	go test -v -cover ./...

# Runs Go linters and validates that all generated code is committed.
validate-go-code: tidy generate lint-go no-changes

no-changes:
	@if [ -n "$$(git status --porcelain)" ]; then \
		git status --porcelain; \
		git --no-pager diff; \
		echo "Encountered dirty repo!"; \
		exit 1; \
	fi

#cut a new version for release with items in docs/docs
gen-docs-release:
	if [ -z ${version} ]; then \
  			echo "version not set (version=x.x)"; \
    		exit 1 \
    	;fi
	docker run --rm --workdir=/docs -v $${PWD}/docs:/docs node:24-bookworm yarn docusaurus docs:version ${version}

# Completely remove doc version from docs site
remove-docs-version:
	if [ -z ${version} ]; then \
  			echo "version not set (version=x.x)"; \
    		exit 1 \
    	;fi
	echo "removing ${version} from documentation completely"
	-rm  "./docs/versioned_sidebars/version-${version}-sidebars.json"
	-rm  -r ./docs/versioned_docs/version-${version}
	jq 'del(.[] | select(. == "${version}"))' ./docs/versions.json > tmp.json && mv tmp.json ./docs/versions.json
	grep -v '"${version}": {label: "${version}", banner: "none", path: "${version}"},' ./docs/docusaurus.config.ts  > tmp.config.ts && mv tmp.config.ts ./docs/docusaurus.config.ts

.PHONY: ui ui-user-install ui-user ui-user-node build docker-build docker-build-fast docker-build-full docker-build-runtime-base docker-build-local-runtime-base all clean dev dev-open otel-jaeger-up otel-jaeger-down otel-jaeger-logs lint lint-admin lint-api no-changes fmt tidy gen-docs-release deprecate-docs-release remove-docs-version
