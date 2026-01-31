# Enterprise CA Certificates

Build Obot images with enterprise CA certificates for internal service trust.

## Quick Start

### Using Makefile (Recommended)

```bash
# 1. Place your CA certificate
cp /path/to/your/ca.crt enterprise-ca.crt

# 2. Show available commands
make help

# 3. Build images
make build-oss REGISTRY=your-registry.example.com

# 4. Build and push
make push-oss REGISTRY=your-registry.example.com

# 5. Cross-compile (arm64 → amd64)
make build-oss REGISTRY=your-registry.example.com PLATFORM=linux/amd64
```

### Using build.sh Directly

```bash
# Build OSS edition (default)
./build.sh -r your-registry.example.com --oss

# Build Enterprise edition
./build.sh -r your-registry.example.com --enterprise

# Cross-compile
./build.sh -r your-registry.example.com --platform linux/amd64

# Multi-platform (requires --push)
./build.sh -r your-registry.example.com --platform linux/amd64,linux/arm64 --push

# Build only Obot or MCP images
./build.sh --obot-only --oss
./build.sh --mcp-only
```

## Files

### Dockerfiles

| File | Description |
|------|-------------|
| `Dockerfile.obot-oss` | OSS Obot image |
| `Dockerfile.obot-oss-dev` | OSS Obot with local keycloak build |
| `Dockerfile.obot-enterprise` | Enterprise Obot image |
| `Dockerfile.obot-enterprise-dev` | Enterprise Obot with local keycloak build |
| `Dockerfile.mcp-base` | MCP base image |
| `Dockerfile.nanobot-shim` | Nanobot shim image |
| `Dockerfile.webhook-converter` | Webhook converter image |

### Scripts

- **`build.sh`** - Build script (`./build.sh --help` for options)

## Deploy

After building, run with Docker:

```bash
docker run -itd \
  --name obot \
  -e OBOT_SERVER_MCPBASE_IMAGE=your-registry.example.com/obot/mcp-base:latest \
  -e OBOT_SERVER_MCPREMOTE_SHIM_BASE_IMAGE=your-registry.example.com/obot/nanobot-shim:latest \
  -e OBOT_SERVER_MCPHTTPWEBHOOK_BASE_IMAGE=your-registry.example.com/obot/webhook-converter:latest \
  -e OBOT_SERVER_MCPRUNTIME_BACKEND=docker \
  -e OBOT_SERVER_ENABLE_AUTHENTICATION=true \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v obot-data:/data \
  -p 8080:8080 \
  your-registry.example.com/obot/obot:oss
```
