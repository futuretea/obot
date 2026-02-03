#!/bin/bash
set -euo pipefail

# Enterprise CA Certificate Build Script
# This script builds custom Obot and MCP images with enterprise CA certificates

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CA_CERT="${CA_CERT:-enterprise-ca.crt}"
REGISTRY="${REGISTRY:-your-registry.example.com}"
OBOT_REPO="${OBOT_REPO:-obot}"
MCP_REPO="${MCP_REPO:-obot}"
OBOT_TAG="${OBOT_TAG:-}"
MCP_TAG="${MCP_TAG:-latest}"
BUILD_KEYCLOAK_LOCAL="${BUILD_KEYCLOAK_LOCAL:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Build custom Obot and MCP images with enterprise CA certificates.

OPTIONS:
    -c, --ca-cert PATH      Path to CA certificate (default: enterprise-ca.crt)
    -r, --registry URL      Container registry URL (default: your-registry.example.com)
    --obot-repo NAME        Obot repository name (default: obot)
    --mcp-repo NAME         MCP repository name (default: obot)
    -t, --tag TAG           Tag for Obot image (default: matches edition - oss/enterprise)
    -m, --mcp-tag TAG       Tag for MCP images (default: latest)
    --platform PLATFORM     Target platform(s): linux/amd64, linux/arm64, or linux/amd64,linux/arm64 (default: native)
    --oss                   Build OSS edition (default)
    --enterprise            Build Enterprise edition
    --obot-only             Build only Obot image
    --mcp-only              Build only MCP images
    --keycloak-local        Build keycloak-auth-provider from local source (for debugging)
    --pull                  Pull latest base images before building
    --push                  Push images after building
    -h, --help              Show this help message

ENVIRONMENT VARIABLES:
    CA_CERT                 Path to CA certificate file
    REGISTRY                Container registry URL
    OBOT_REPO               Obot repository name
    MCP_REPO                MCP repository name
    OBOT_TAG                Tag for Obot image
    MCP_TAG                 Tag for MCP images
    BUILD_KEYCLOAK_LOCAL    Build keycloak-auth-provider from local source (true/false)
    REGISTRY_USERNAME       Registry username (for push)
    REGISTRY_PASSWORD       Registry password (for push)

EXAMPLES:
    # Build all images with custom CA certificate
    $0 -c /path/to/ca.crt -r myregistry.com

    # Build and push only Obot image
    $0 --obot-only --push

    # Build MCP images with specific tag
    $0 --mcp-only -m v1.0.0

    # Build Obot with local keycloak-auth-provider for debugging
    $0 --obot-only --keycloak-local

    # Build with latest base images
    $0 --pull

    # Build for amd64 platform (cross-compile on arm64 machine)
    $0 --platform linux/amd64

    # Build multi-platform images (requires --push)
    $0 --platform linux/amd64,linux/arm64 --push

    # Build OSS edition
    $0 --oss

    # Build Enterprise edition
    $0 --enterprise

EOF
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        exit 1
    fi
    
    # Check buildx if platform is specified
    if [ -n "$PLATFORM" ]; then
        if ! docker buildx version &> /dev/null; then
            log_error "Docker buildx is required for cross-platform builds"
            exit 1
        fi
        
        # Multi-platform builds require push
        if [[ "$PLATFORM" == *","* ]] && [ "$PUSH_IMAGES" = false ]; then
            log_error "Multi-platform builds require --push flag (cannot load multi-arch images locally)"
            exit 1
        fi
    fi
    
    if [ ! -f "$SCRIPT_DIR/$CA_CERT" ]; then
        log_error "CA certificate not found: $SCRIPT_DIR/$CA_CERT"
        log_info "Please place your enterprise CA certificate in $SCRIPT_DIR/"
        exit 1
    fi
    
    log_info "Prerequisites check passed"
}

build_obot_image() {
    log_info "Building Obot ${OBOT_EDITION} image..."
    
    local build_context="$SCRIPT_DIR"
    local dockerfile
    local build_cmd="docker build"
    local build_args=(
        -t "$REGISTRY/$OBOT_REPO/obot:$OBOT_TAG"
    )
    
    # Use buildx for cross-platform builds
    if [ -n "$PLATFORM" ]; then
        build_cmd="docker buildx build"
        build_args+=(--platform "$PLATFORM")
        # For single platform, load to local docker; for multi-platform, must push
        if [[ "$PLATFORM" != *","* ]]; then
            build_args+=(--load)
        fi
    fi
    
    if [ "$BUILD_KEYCLOAK_LOCAL" = "true" ]; then
        log_info "Building with local keycloak-auth-provider source (dev mode)..."
        dockerfile="$SCRIPT_DIR/Dockerfile.obot-${OBOT_EDITION}-dev"
        
        # Check if keycloak-auth-provider source exists
        local keycloak_src="$WORKSPACE_DIR/third-party-projects/obot-tools/keycloak-auth-provider"
        local common_src="$WORKSPACE_DIR/third-party-projects/obot-tools/auth-providers-common"
        
        if [ ! -d "$keycloak_src" ]; then
            log_error "keycloak-auth-provider source not found: $keycloak_src"
            exit 1
        fi
        
        if [ ! -d "$common_src" ]; then
            log_error "auth-providers-common source not found: $common_src"
            exit 1
        fi
        
        # Copy source to build context
        log_info "Copying keycloak-auth-provider source to build context..."
        rm -rf "$SCRIPT_DIR/keycloak-auth-provider" "$SCRIPT_DIR/auth-providers-common"
        cp -r "$keycloak_src" "$SCRIPT_DIR/keycloak-auth-provider"
        cp -r "$common_src" "$SCRIPT_DIR/auth-providers-common"
    else
        log_info "Building with pre-built tools (production mode)..."
        dockerfile="$SCRIPT_DIR/Dockerfile.obot-${OBOT_EDITION}"
    fi
    
    build_args+=(-f "$dockerfile")
    
    if [ "$PULL_IMAGES" = true ]; then
        build_args+=(--pull)
    fi
    
    # For multi-platform builds with push, add --push flag
    if [ -n "$PLATFORM" ] && [[ "$PLATFORM" == *","* ]]; then
        build_args+=(--push)
    fi
    
    $build_cmd "${build_args[@]}" "$build_context"
    
    # Cleanup copied source (if any)
    rm -rf "$SCRIPT_DIR/keycloak-auth-provider" "$SCRIPT_DIR/auth-providers-common"
    
    log_info "Successfully built $REGISTRY/$OBOT_REPO/obot:$OBOT_TAG"
}

build_mcp_images() {
    log_info "Building MCP images..."
    
    local build_cmd="docker build"
    
    # Use buildx for cross-platform builds
    if [ -n "$PLATFORM" ]; then
        build_cmd="docker buildx build"
    fi
    
    local images=(
        "nanobot:Dockerfile.nanobot"
        "mcp-phat:Dockerfile.mcp-phat"
        "mcp-webhook-converter:Dockerfile.mcp-webhook-converter"
        "mcp-github:Dockerfile.mcp-github"
        "mcp-grafana:Dockerfile.mcp-grafana"
        "mcp-elasticsearch:Dockerfile.mcp-elasticsearch"
    )
    
    for image_def in "${images[@]}"; do
        IFS=':' read -r image_name dockerfile <<< "$image_def"
        
        if [ -f "$SCRIPT_DIR/$dockerfile" ]; then
            log_info "Building $image_name..."
            local mcp_build_args=(
                -t "$REGISTRY/$MCP_REPO/$image_name:$MCP_TAG"
                -f "$SCRIPT_DIR/$dockerfile"
            )
            if [ -n "$PLATFORM" ]; then
                mcp_build_args+=(--platform "$PLATFORM")
                if [[ "$PLATFORM" != *","* ]]; then
                    mcp_build_args+=(--load)
                else
                    mcp_build_args+=(--push)
                fi
            fi
            if [ "$PULL_IMAGES" = true ]; then
                mcp_build_args+=(--pull)
            fi
            $build_cmd "${mcp_build_args[@]}" "$SCRIPT_DIR"
            log_info "Successfully built $REGISTRY/$MCP_REPO/$image_name:$MCP_TAG"
        else
            log_warn "Dockerfile not found: $dockerfile (skipping)"
        fi
    done
}

push_images() {
    # Skip if multi-platform build (already pushed via buildx)
    if [ -n "$PLATFORM" ] && [[ "$PLATFORM" == *","* ]]; then
        log_info "Multi-platform images already pushed during build"
        return
    fi
    
    log_info "Pushing images to registry..."
    
    # Login if credentials provided
    if [ -n "${REGISTRY_USERNAME:-}" ] && [ -n "${REGISTRY_PASSWORD:-}" ]; then
        log_info "Logging in to registry..."
        echo "$REGISTRY_PASSWORD" | docker login "$REGISTRY" -u "$REGISTRY_USERNAME" --password-stdin
    fi
    
    if [ "$BUILD_OBOT" = true ]; then
        log_info "Pushing $REGISTRY/$OBOT_REPO/obot:$OBOT_TAG"
        docker push "$REGISTRY/$OBOT_REPO/obot:$OBOT_TAG"
    fi
    
    if [ "$BUILD_MCP" = true ]; then
        local images=("nanobot" "mcp-phat" "mcp-webhook-converter" "mcp-github" "mcp-grafana" "mcp-elasticsearch")
        for image_name in "${images[@]}"; do
            if docker image inspect "$REGISTRY/$MCP_REPO/$image_name:$MCP_TAG" &> /dev/null; then
                log_info "Pushing $REGISTRY/$MCP_REPO/$image_name:$MCP_TAG"
                docker push "$REGISTRY/$MCP_REPO/$image_name:$MCP_TAG"
            fi
        done
    fi
    
    log_info "All images pushed successfully"
}

# Parse arguments
BUILD_OBOT=true
BUILD_MCP=true
PULL_IMAGES=false
PUSH_IMAGES=false
PLATFORM=""
OBOT_EDITION="oss"

while [[ $# -gt 0 ]]; do
    case $1 in
        -c|--ca-cert)
            CA_CERT="$2"
            shift 2
            ;;
        -r|--registry)
            REGISTRY="$2"
            shift 2
            ;;
        --obot-repo)
            OBOT_REPO="$2"
            shift 2
            ;;
        --mcp-repo)
            MCP_REPO="$2"
            shift 2
            ;;
        -t|--tag)
            OBOT_TAG="$2"
            shift 2
            ;;
        -m|--mcp-tag)
            MCP_TAG="$2"
            shift 2
            ;;
        --platform)
            PLATFORM="$2"
            shift 2
            ;;
        --oss)
            OBOT_EDITION="oss"
            shift
            ;;
        --enterprise)
            OBOT_EDITION="enterprise"
            shift
            ;;
        --obot-only)
            BUILD_MCP=false
            shift
            ;;
        --mcp-only)
            BUILD_OBOT=false
            shift
            ;;
        --keycloak-local)
            BUILD_KEYCLOAK_LOCAL=true
            shift
            ;;
        --pull)
            PULL_IMAGES=true
            shift
            ;;
        --push)
            PUSH_IMAGES=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Main execution
main() {
    # Set default tag based on edition if not specified
    if [ -z "$OBOT_TAG" ]; then
        OBOT_TAG="$OBOT_EDITION"
    fi
    
    log_info "Starting build process..."
    log_info "CA Certificate: $CA_CERT"
    log_info "Registry: $REGISTRY"
    log_info "Obot Edition: $OBOT_EDITION"
    log_info "Obot Tag: $OBOT_TAG"
    log_info "MCP Tag: $MCP_TAG"
    log_info "Build keycloak-auth-provider from local: $BUILD_KEYCLOAK_LOCAL"
    log_info "Pull latest base images: $PULL_IMAGES"
    log_info "Target platform: ${PLATFORM:-native}"
    echo
    
    check_prerequisites
    
    if [ "$BUILD_OBOT" = true ]; then
        build_obot_image
    fi
    
    if [ "$BUILD_MCP" = true ]; then
        build_mcp_images
    fi
    
    if [ "$PUSH_IMAGES" = true ]; then
        push_images
    fi
    
    echo
    log_info "Build completed successfully!"
    echo
    log_info "Next steps:"
    
    if [ "$PUSH_IMAGES" = false ]; then
        log_info "  1. Push images: $0 --push"
        echo
    fi
    
    log_info "Deploy with Docker:"
    if [ "$BUILD_OBOT" = true ]; then
        echo
        echo "  docker run -itd \\"
        echo "    --name obot \\"
        echo "    -e OBOT_SERVER_MCPBASE_IMAGE=$REGISTRY/$MCP_REPO/mcp-base:$MCP_TAG \\"
        echo "    -e OBOT_SERVER_MCPREMOTE_SHIM_BASE_IMAGE=$REGISTRY/$MCP_REPO/nanobot-shim:$MCP_TAG \\"
        echo "    -e OBOT_SERVER_MCPHTTPWEBHOOK_BASE_IMAGE=$REGISTRY/$MCP_REPO/webhook-converter:$MCP_TAG \\"
        echo "    -e OBOT_SERVER_MCPRUNTIME_BACKEND=docker \\"
        echo "    -e OBOT_SERVER_ENABLE_AUTHENTICATION=true \\"
        echo "    -v /var/run/docker.sock:/var/run/docker.sock \\"
        echo "    -v obot-data:/data \\"
        echo "    -p 8080:8080 \\"
        echo "    $REGISTRY/$OBOT_REPO/obot:$OBOT_TAG"
    fi
}

main
