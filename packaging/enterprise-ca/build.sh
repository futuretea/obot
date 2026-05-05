#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CA_WRAP_TOOL="${CA_WRAP_TOOL:-$SCRIPT_DIR/image-ca-wrap/image-ca-wrap.sh}"

PACKAGE_VERSION="${PACKAGE_VERSION:-master}"
CA_CERT="${CA_CERT:-enterprise-ca.crt}"
REGISTRY="${REGISTRY:-harbor.futuretea.me}"
OBOT_REPO="${OBOT_REPO:-obot}"
MCP_REPO="${MCP_REPO:-obot}"
OBOT_TAG="${OBOT_TAG:-}"
MCP_TAG="${MCP_TAG:-}"
PLATFORM="${PLATFORM:-}"

BUILD_OBOT=true
BUILD_MCP=true
BUILD_CATALOG_TOOLS=false
PULL_IMAGES=false
PUSH_IMAGES=false
CHECK_ONLY=false

log_info() { printf '[INFO] %s\n' "$*"; }
die() { printf '[ERROR] %s\n' "$*" >&2; exit 1; }

usage() {
    cat <<'EOF'
Usage: ./build.sh [OPTIONS]

Wrap Obot and MCP images with an enterprise CA certificate.

Options:
  -c, --ca-cert PATH       Enterprise CA certificate path.
      --ca-wrap-tool PATH  Generic image CA wrapper tool path.
  -r, --registry URL       Container registry URL.
      --obot-repo NAME     Obot image repository namespace.
      --mcp-repo NAME      MCP image repository namespace.
      --package-version V  Obot base image tag. Defaults to master.
  -t, --tag TAG            Obot image tag override. Defaults to <source-tag>-ca.
  -m, --mcp-tag TAG        MCP image tag override. Defaults to <source-tag>-ca.
      --obot-only          Build only the Obot image.
      --mcp-only           Build only MCP/runtime images.
      --catalog-tools      Also build optional catalog tool images.
      --platform PLATFORM  Docker platform, for example linux/amd64.
      --pull               Pull base images before building.
      --push               Push images after building.
      --check-only         Validate local inputs and exit.
  -h, --help               Show this help message.
EOF
}

need_value() {
    [[ $# -ge 2 && -n "${2:-}" ]] || die "$1 requires a value"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -c|--ca-cert) need_value "$@"; CA_CERT="$2"; shift 2 ;;
        --ca-wrap-tool) need_value "$@"; CA_WRAP_TOOL="$2"; shift 2 ;;
        -r|--registry) need_value "$@"; REGISTRY="$2"; shift 2 ;;
        --obot-repo) need_value "$@"; OBOT_REPO="$2"; shift 2 ;;
        --mcp-repo) need_value "$@"; MCP_REPO="$2"; shift 2 ;;
        --package-version) need_value "$@"; PACKAGE_VERSION="$2"; shift 2 ;;
        -t|--tag) need_value "$@"; OBOT_TAG="$2"; shift 2 ;;
        -m|--mcp-tag) need_value "$@"; MCP_TAG="$2"; shift 2 ;;
        --obot-only) BUILD_OBOT=true; BUILD_MCP=false; shift ;;
        --mcp-only) BUILD_OBOT=false; BUILD_MCP=true; shift ;;
        --catalog-tools) BUILD_CATALOG_TOOLS=true; shift ;;
        --platform) need_value "$@"; PLATFORM="$2"; shift 2 ;;
        --pull) PULL_IMAGES=true; shift ;;
        --push) PUSH_IMAGES=true; shift ;;
        --check-only) CHECK_ONLY=true; shift ;;
        -h|--help) usage; exit 0 ;;
        *) die "Unknown option: $1" ;;
    esac
done

apply_defaults() {
    OBOT_BASE_IMAGE="${OBOT_BASE_IMAGE:-ghcr.io/futuretea/obot:${PACKAGE_VERSION}}"

    PHAT_BASE_IMAGE="${PHAT_BASE_IMAGE:-ghcr.io/obot-platform/mcp-images/phat:v0.20.3}"
    NANOBOT_BASE_IMAGE="${NANOBOT_BASE_IMAGE:-ghcr.io/nanobot-ai/nanobot:v0.0.76}"
    NANOBOT_AGENT_BASE_IMAGE="${NANOBOT_AGENT_BASE_IMAGE:-ghcr.io/nanobot-ai/nanobot-agent:v0.0.76}"
    WEBHOOK_CONVERTER_BASE_IMAGE="${WEBHOOK_CONVERTER_BASE_IMAGE:-ghcr.io/obot-platform/mcp-images/http-webhook-mcp-converter:v0.20.2}"
    OBOT_MCP_SERVER_SEARCH_BASE_IMAGE="${OBOT_MCP_SERVER_SEARCH_BASE_IMAGE:-ghcr.io/obot-platform/obot-mcp-server:v0.1.1}"

    GITHUB_MCP_BASE_IMAGE="${GITHUB_MCP_BASE_IMAGE:-ghcr.io/obot-platform/mcp-images/github:main}"
    GRAFANA_MCP_BASE_IMAGE="${GRAFANA_MCP_BASE_IMAGE:-ghcr.io/obot-platform/mcp-images/grafana:latest}"
}

core_image_defs() {
    cat <<'EOF'
mcp-phat|PHAT_BASE_IMAGE
nanobot|NANOBOT_BASE_IMAGE
nanobot-agent|NANOBOT_AGENT_BASE_IMAGE
mcp-webhook-converter|WEBHOOK_CONVERTER_BASE_IMAGE
obot-mcp-server-search|OBOT_MCP_SERVER_SEARCH_BASE_IMAGE
EOF
}

catalog_image_defs() {
    cat <<'EOF'
mcp-github|GITHUB_MCP_BASE_IMAGE
mcp-grafana|GRAFANA_MCP_BASE_IMAGE
EOF
}

mcp_image_defs() {
    core_image_defs
    [[ "$BUILD_CATALOG_TOOLS" == true ]] && catalog_image_defs
}

repo_ref() {
    printf '%s/%s/%s\n' "$REGISTRY" "$1" "$2"
}

image_ref() {
    printf '%s:%s\n' "$(repo_ref "$1" "$2")" "$3"
}

source_image_tag() {
    local image_without_digest last_path_part
    image_without_digest="${1%%@*}"
    last_path_part="${image_without_digest##*/}"

    if [[ "$last_path_part" == *:* ]]; then
        printf '%s\n' "${last_path_part##*:}"
    else
        printf 'latest\n'
    fi
}

ca_tag() {
    printf '%s-ca\n' "$(source_image_tag "$1")"
}

obot_target_tag() {
    local source_image="$1"
    if [[ -n "$OBOT_TAG" ]]; then
        printf '%s\n' "$OBOT_TAG"
    else
        ca_tag "$source_image"
    fi
}

mcp_target_tag() {
    local source_image="$1"
    if [[ -n "$MCP_TAG" ]]; then
        printf '%s\n' "$MCP_TAG"
    else
        ca_tag "$source_image"
    fi
}

resolve_ca_cert_path() {
    local candidates=()
    if [[ "$CA_CERT" = /* ]]; then
        candidates+=("$CA_CERT")
    else
        candidates+=("$PWD/$CA_CERT" "$SCRIPT_DIR/$CA_CERT")
    fi

    local candidate
    for candidate in "${candidates[@]}"; do
        [[ -f "$candidate" ]] && { printf '%s\n' "$candidate"; return; }
    done
    printf '%s\n' "${candidates[0]}"
}

check_prerequisites() {
    local ca_cert_path
    ca_cert_path="$(resolve_ca_cert_path)"
    [[ -f "$ca_cert_path" ]] || die "CA certificate not found: $ca_cert_path"
    [[ -x "$CA_WRAP_TOOL" ]] || die "CA wrap tool not executable: $CA_WRAP_TOOL"
}

append_image_arg() {
    local source_image="$1"
    local target_image="$2"
    CA_WRAP_ARGS+=(--image "$source_image=$target_image")
}

collect_image_args() {
    CA_WRAP_ARGS=(-c "$(resolve_ca_cert_path)")
    [[ -n "$PLATFORM" ]] && CA_WRAP_ARGS+=(--platform "$PLATFORM")
    [[ "$PULL_IMAGES" == true ]] && CA_WRAP_ARGS+=(--pull)
    [[ "$PUSH_IMAGES" == true ]] && CA_WRAP_ARGS+=(--push)
    [[ "$CHECK_ONLY" == true ]] && CA_WRAP_ARGS+=(--check-only)

    if [[ "$BUILD_OBOT" == true ]]; then
        local obot_source_image
        obot_source_image="$OBOT_BASE_IMAGE"
        append_image_arg "$obot_source_image" "$(image_ref "$OBOT_REPO" obot "$(obot_target_tag "$obot_source_image")")"
    fi

    if [[ "$BUILD_MCP" == true ]]; then
        local image_name base_var source_image
        while IFS='|' read -r image_name base_var; do
            source_image="${!base_var}"
            append_image_arg "$source_image" "$(image_ref "$MCP_REPO" "$image_name" "$(mcp_target_tag "$source_image")")"
        done < <(mcp_image_defs)
    fi
}

main() {
    apply_defaults

    check_prerequisites
    collect_image_args

    log_info "Obot base image tag: $PACKAGE_VERSION"
    log_info "Registry: $REGISTRY"
    log_info "Obot tag override: ${OBOT_TAG:-<source-tag>-ca}"
    log_info "MCP tag override: ${MCP_TAG:-<source-tag>-ca}"
    log_info "Platform: ${PLATFORM:-native}"
    log_info "Build Obot image: $BUILD_OBOT"
    log_info "Build MCP images: $BUILD_MCP"
    log_info "Build optional catalog tools: $BUILD_CATALOG_TOOLS"

    "$CA_WRAP_TOOL" "${CA_WRAP_ARGS[@]}"
    if [[ "$CHECK_ONLY" == true ]]; then
        log_info "Enterprise CA packaging inputs are valid"
        exit 0
    fi

    log_info "Build completed"
}

main
