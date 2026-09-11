#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKERFILE="${DOCKERFILE:-$SCRIPT_DIR/Containerfile}"
CA_CERT="${CA_CERT:-enterprise-ca.crt}"
PLATFORM="${PLATFORM:-}"
FINAL_USER="${FINAL_USER:-root}"
PULL_IMAGE=false
PUSH_IMAGE=false
CHECK_ONLY=false
IMAGE_SPECS=()
BUILD_CONTEXT_DIR=""

log_info() { printf '[INFO] %s\n' "$*"; }
die() { printf '[ERROR] %s\n' "$*" >&2; exit 1; }

usage() {
    cat <<'EOF'
Usage: image-ca-wrap.sh [OPTIONS]

Build new image tags from existing images after adding an enterprise CA certificate.

Options:
  -c, --ca-cert PATH       Enterprise CA certificate path.
      --image SRC=DST      Source image and target image tag. May be repeated.
      --platform PLATFORM  Docker platform, for example linux/amd64.
      --final-user USER    Final image USER. Defaults to root.
      --pull               Pull source images before building.
      --push               Push target image tags after building.
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
        --image) need_value "$@"; IMAGE_SPECS+=("$2"); shift 2 ;;
        --platform) need_value "$@"; PLATFORM="$2"; shift 2 ;;
        --final-user) need_value "$@"; FINAL_USER="$2"; shift 2 ;;
        --pull) PULL_IMAGE=true; shift ;;
        --push) PUSH_IMAGE=true; shift ;;
        --check-only) CHECK_ONLY=true; shift ;;
        -h|--help) usage; exit 0 ;;
        *) die "Unknown option: $1" ;;
    esac
done

resolve_ca_cert_path() {
    if [[ "$CA_CERT" = /* ]]; then
        printf '%s\n' "$CA_CERT"
    else
        printf '%s\n' "$PWD/$CA_CERT"
    fi
}

validate_image_spec() {
    local spec="$1"
    [[ "$spec" == *=* ]] || die "Image spec must be source=target: $spec"
    [[ -n "${spec%%=*}" && -n "${spec#*=}" ]] || die "Image spec must be source=target: $spec"
}

validate_ca_cert() {
    local ca_cert_path="$1"

    if command -v openssl >/dev/null 2>&1; then
        openssl x509 -in "$ca_cert_path" -noout >/dev/null 2>&1 || die "CA certificate is not a valid PEM certificate: $ca_cert_path"
        return
    fi

    grep -q -- "-----BEGIN CERTIFICATE-----" "$ca_cert_path" || die "CA certificate does not look like a PEM certificate: $ca_cert_path"
}

check_prerequisites() {
    local ca_cert_path
    ca_cert_path="$(resolve_ca_cert_path)"

    [[ -f "$ca_cert_path" ]] || die "CA certificate not found: $ca_cert_path"
    [[ -f "$DOCKERFILE" ]] || die "Containerfile not found: $DOCKERFILE"
    [[ "${#IMAGE_SPECS[@]}" -gt 0 ]] || die "At least one --image source=target is required"
    validate_ca_cert "$ca_cert_path"

    local spec
    for spec in "${IMAGE_SPECS[@]}"; do
        validate_image_spec "$spec"
    done

    [[ "$CHECK_ONLY" == true ]] && return

    command -v docker >/dev/null 2>&1 || die "Docker is not installed or not on PATH"
    if [[ -n "$PLATFORM" ]]; then
        docker buildx version >/dev/null 2>&1 || die "Docker buildx is required when --platform is set"
        [[ "$PLATFORM" != *,* || "$PUSH_IMAGE" == true ]] || die "Multi-platform builds require --push"
    fi
}

prepare_context() {
    local ca_cert_path="$1"
    local context_dir
    context_dir="$(mktemp -d)"
    mkdir -p "$context_dir/.build"
    cp "$ca_cert_path" "$context_dir/.build/enterprise-ca.crt"
    chmod 0644 "$context_dir/.build/enterprise-ca.crt"
    printf '%s\n' "$context_dir"
}

docker_build() {
    local source_image="$1"
    local target_image="$2"
    local context_dir="$3"

    local args=(-f "$DOCKERFILE" -t "$target_image" --build-arg "BASE_IMAGE=$source_image" --build-arg "FINAL_USER=$FINAL_USER")
    [[ "$PULL_IMAGE" == true ]] && args+=(--pull)

    log_info "Wrapping $source_image as $target_image"
    if [[ -n "$PLATFORM" ]]; then
        local buildx_args=(buildx build --platform "$PLATFORM")
        if [[ "$PUSH_IMAGE" == true ]]; then
            buildx_args+=(--push)
        else
            buildx_args+=(--load)
        fi
        docker "${buildx_args[@]}" "${args[@]}" "$context_dir"
    else
        docker build "${args[@]}" "$context_dir"
        if [[ "$PUSH_IMAGE" == true ]]; then
            log_info "Pushing $target_image"
            docker push "$target_image"
        fi
    fi
}

cleanup() {
    [[ -z "${BUILD_CONTEXT_DIR:-}" ]] || rm -rf "$BUILD_CONTEXT_DIR"
}

main() {
    check_prerequisites

    if [[ "$CHECK_ONLY" == true ]]; then
        log_info "Image CA wrap inputs are valid"
        exit 0
    fi

    local ca_cert_path
    ca_cert_path="$(resolve_ca_cert_path)"
    BUILD_CONTEXT_DIR="$(prepare_context "$ca_cert_path")"
    trap cleanup EXIT

    local spec source_image target_image
    for spec in "${IMAGE_SPECS[@]}"; do
        source_image="${spec%%=*}"
        target_image="${spec#*=}"
        docker_build "$source_image" "$target_image" "$BUILD_CONTEXT_DIR"
    done
}

main
