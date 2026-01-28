#!/bin/bash
set -euo pipefail

# Test Script for Enterprise CA Certificate Setup
# Validates that CA certificates are correctly configured in Obot and MCP servers

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_URL="${TEST_URL:-https://your-internal-service.example.com}"
NAMESPACE="${NAMESPACE:-obot-system}"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_success() { echo -e "${GREEN}[✓]${NC} $1"; }
log_fail() { echo -e "${RED}[✗]${NC} $1"; }

usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Test enterprise CA certificate configuration in Obot deployment.

OPTIONS:
    -u, --url URL           Test URL for certificate validation
    -n, --namespace NS      Kubernetes namespace (default: obot-system)
    --docker                Test Docker deployment instead of Kubernetes
    -h, --help              Show this help message

EXAMPLES:
    # Test Kubernetes deployment
    $0 -u https://internal.company.com

    # Test Docker deployment
    $0 --docker -u https://internal.company.com

EOF
}

test_kubernetes_obot() {
    log_info "Testing Obot server in Kubernetes..."
    
    local pod=$(kubectl get pod -n "$NAMESPACE" -l app.kubernetes.io/name=obot -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    
    if [ -z "$pod" ]; then
        log_fail "Obot pod not found in namespace $NAMESPACE"
        return 1
    fi
    
    log_info "Found pod: $pod"
    
    # Test 1: Check if CA certificate file exists
    log_info "Test 1: Checking CA certificate in system bundle..."
    if kubectl exec -n "$NAMESPACE" "$pod" -- test -f /etc/ssl/certs/ca-certificates.crt; then
        log_success "CA bundle exists"
    else
        log_fail "CA bundle not found"
        return 1
    fi
    
    # Test 2: Try to connect to test URL
    if [ "$TEST_URL" != "https://your-internal-service.example.com" ]; then
        log_info "Test 2: Testing connection to $TEST_URL..."
        if kubectl exec -n "$NAMESPACE" "$pod" -- curl -s -o /dev/null -w "%{http_code}" "$TEST_URL" | grep -qE "^(200|301|302|401|403)"; then
            log_success "Successfully connected to $TEST_URL"
        else
            log_warn "Could not connect to $TEST_URL (may require authentication)"
        fi
    else
        log_warn "Test 2: Skipped (please provide --url with your internal service)"
    fi
    
    # Test 3: Check Obot logs for certificate errors
    log_info "Test 3: Checking logs for certificate errors..."
    if kubectl logs -n "$NAMESPACE" "$pod" --tail=100 | grep -qi "certificate\|x509\|tls"; then
        log_warn "Found TLS/certificate mentions in logs (review recommended)"
        kubectl logs -n "$NAMESPACE" "$pod" --tail=20 | grep -i "certificate\|x509\|tls" || true
    else
        log_success "No certificate errors in recent logs"
    fi
}

test_kubernetes_mcp() {
    log_info "Testing MCP servers in Kubernetes..."
    
    local mcp_pods=$(kubectl get pods --all-namespaces -l mcp.server.id --no-headers 2>/dev/null | awk '{print $1 " " $2}')
    
    if [ -z "$mcp_pods" ]; then
        log_warn "No MCP server pods found (they may not be running yet)"
        return 0
    fi
    
    echo "$mcp_pods" | while read -r ns pod; do
        log_info "Testing MCP pod: $pod in namespace $ns"
        
        # Check CA certificate
        if kubectl exec -n "$ns" "$pod" -- test -f /etc/ssl/certs/ca-certificates.crt 2>/dev/null; then
            log_success "  CA bundle exists in $pod"
        else
            log_fail "  CA bundle not found in $pod"
        fi
        
        # Check image
        local image=$(kubectl get pod -n "$ns" "$pod" -o jsonpath='{.spec.containers[0].image}')
        log_info "  Image: $image"
    done
}

test_docker_obot() {
    log_info "Testing Obot server in Docker..."
    
    local container=$(docker ps --filter "name=obot" --format "{{.Names}}" | head -n 1)
    
    if [ -z "$container" ]; then
        log_fail "Obot container not found"
        return 1
    fi
    
    log_info "Found container: $container"
    
    # Test 1: Check CA certificate
    log_info "Test 1: Checking CA certificate..."
    if docker exec "$container" test -f /etc/ssl/certs/ca-certificates.crt; then
        log_success "CA bundle exists"
    else
        log_fail "CA bundle not found"
        return 1
    fi
    
    # Test 2: Test connection
    if [ "$TEST_URL" != "https://your-internal-service.example.com" ]; then
        log_info "Test 2: Testing connection to $TEST_URL..."
        if docker exec "$container" curl -s -o /dev/null -w "%{http_code}" "$TEST_URL" | grep -qE "^(200|301|302|401|403)"; then
            log_success "Successfully connected to $TEST_URL"
        else
            log_warn "Could not connect to $TEST_URL"
        fi
    fi
    
    # Test 3: Check logs
    log_info "Test 3: Checking logs for certificate errors..."
    if docker logs "$container" --tail=100 2>&1 | grep -qi "certificate\|x509\|tls"; then
        log_warn "Found TLS/certificate mentions in logs"
    else
        log_success "No certificate errors in recent logs"
    fi
}

test_docker_mcp() {
    log_info "Testing MCP servers in Docker..."
    
    local mcp_containers=$(docker ps --filter "label=mcp.server.id" --format "{{.Names}}")
    
    if [ -z "$mcp_containers" ]; then
        log_warn "No MCP server containers found"
        return 0
    fi
    
    echo "$mcp_containers" | while read -r container; do
        log_info "Testing MCP container: $container"
        
        if docker exec "$container" test -f /etc/ssl/certs/ca-certificates.crt 2>/dev/null; then
            log_success "  CA bundle exists in $container"
        else
            log_fail "  CA bundle not found in $container"
        fi
        
        local image=$(docker inspect "$container" --format='{{.Config.Image}}')
        log_info "  Image: $image"
    done
}

# Parse arguments
DEPLOYMENT_TYPE="kubernetes"

while [[ $# -gt 0 ]]; do
    case $1 in
        -u|--url)
            TEST_URL="$2"
            shift 2
            ;;
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --docker)
            DEPLOYMENT_TYPE="docker"
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

main() {
    log_info "Starting CA certificate validation tests..."
    log_info "Deployment type: $DEPLOYMENT_TYPE"
    log_info "Test URL: $TEST_URL"
    echo
    
    if [ "$DEPLOYMENT_TYPE" = "kubernetes" ]; then
        test_kubernetes_obot
        echo
        test_kubernetes_mcp
    else
        test_docker_obot
        echo
        test_docker_mcp
    fi
    
    echo
    log_info "Testing completed!"
}

main
