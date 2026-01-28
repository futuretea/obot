#!/bin/bash
set -euo pipefail

# Enterprise CA Certificate Build Script
# This script builds custom Obot and MCP images with enterprise CA certificates

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CA_CERT="${CA_CERT:-enterprise-ca.crt}"
REGISTRY="${REGISTRY:-your-registry.example.com}"
OBOT_REPO="${OBOT_REPO:-obot}"
MCP_REPO="${MCP_REPO:-obot}"
OBOT_TAG="${OBOT_TAG:-enterprise}"
MCP_TAG="${MCP_TAG:-latest}"

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
    -t, --tag TAG           Tag for Obot image (default: enterprise)
    -m, --mcp-tag TAG       Tag for MCP images (default: latest)
    --obot-only             Build only Obot image
    --mcp-only              Build only MCP images
    --push                  Push images after building
    -h, --help              Show this help message

ENVIRONMENT VARIABLES:
    CA_CERT                 Path to CA certificate file
    REGISTRY                Container registry URL
    OBOT_REPO               Obot repository name
    MCP_REPO                MCP repository name
    OBOT_TAG                Tag for Obot image
    MCP_TAG                 Tag for MCP images
    REGISTRY_USERNAME       Registry username (for push)
    REGISTRY_PASSWORD       Registry password (for push)

EXAMPLES:
    # Build all images with custom CA certificate
    $0 -c /path/to/ca.crt -r myregistry.com

    # Build and push only Obot image
    $0 --obot-only --push

    # Build MCP images with specific tag
    $0 --mcp-only -m v1.0.0

EOF
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        exit 1
    fi
    
    if [ ! -f "$SCRIPT_DIR/$CA_CERT" ]; then
        log_error "CA certificate not found: $SCRIPT_DIR/$CA_CERT"
        log_info "Please place your enterprise CA certificate in $SCRIPT_DIR/"
        exit 1
    fi
    
    log_info "Prerequisites check passed"
}

build_obot_image() {
    log_info "Building Obot enterprise image..."
    
    docker build \
        -t "$REGISTRY/$OBOT_REPO:$OBOT_TAG" \
        -f "$SCRIPT_DIR/Dockerfile.obot-enterprise" \
        "$SCRIPT_DIR"
    
    log_info "Successfully built $REGISTRY/$OBOT_REPO:$OBOT_TAG"
}

build_mcp_images() {
    log_info "Building MCP images..."
    
    local images=(
        "mcp-base:Dockerfile.mcp-base"
        "nanobot-shim:Dockerfile.nanobot-shim"
        "webhook-converter:Dockerfile.webhook-converter"
    )
    
    for image_def in "${images[@]}"; do
        IFS=':' read -r image_name dockerfile <<< "$image_def"
        
        if [ -f "$SCRIPT_DIR/$dockerfile" ]; then
            log_info "Building $image_name..."
            docker build \
                -t "$REGISTRY/$MCP_REPO/$image_name:$MCP_TAG" \
                -f "$SCRIPT_DIR/$dockerfile" \
                "$SCRIPT_DIR"
            log_info "Successfully built $REGISTRY/$MCP_REPO/$image_name:$MCP_TAG"
        else
            log_warn "Dockerfile not found: $dockerfile (skipping)"
        fi
    done
}

push_images() {
    log_info "Pushing images to registry..."
    
    # Login if credentials provided
    if [ -n "${REGISTRY_USERNAME:-}" ] && [ -n "${REGISTRY_PASSWORD:-}" ]; then
        log_info "Logging in to registry..."
        echo "$REGISTRY_PASSWORD" | docker login "$REGISTRY" -u "$REGISTRY_USERNAME" --password-stdin
    fi
    
    if [ "$BUILD_OBOT" = true ]; then
        log_info "Pushing $REGISTRY/$OBOT_REPO:$OBOT_TAG"
        docker push "$REGISTRY/$OBOT_REPO:$OBOT_TAG"
    fi
    
    if [ "$BUILD_MCP" = true ]; then
        local images=("mcp-base" "nanobot-shim" "webhook-converter")
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
PUSH_IMAGES=false

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
        --obot-only)
            BUILD_MCP=false
            shift
            ;;
        --mcp-only)
            BUILD_OBOT=false
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
    log_info "Starting build process..."
    log_info "CA Certificate: $CA_CERT"
    log_info "Registry: $REGISTRY"
    log_info "Obot Tag: $OBOT_TAG"
    log_info "MCP Tag: $MCP_TAG"
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
        echo "    $REGISTRY/$OBOT_REPO:$OBOT_TAG"
    fi
    
    echo
    log_info "Deploy with Kubernetes:"
    log_info "  ./deploy.sh install"
    log_info "  or"
    log_info "  make deploy"
}

main
