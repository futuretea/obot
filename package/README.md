# Enterprise CA Certificates

This package provides everything you need to build and deploy Obot with enterprise CA certificates.

## Quick Start

### Option A: Using Makefile (Recommended)

```bash
# 1. Place your CA certificate
cp /path/to/your/ca.crt enterprise-ca.crt

# 2. Show available commands
make help

# 3. Build images
make build REGISTRY=your-registry.example.com

# 4. Push to registry
make push

# 5. Deploy to Kubernetes
make deploy NAMESPACE=obot-system

# 6. Verify deployment
make verify
```

### Option B: Using Scripts Directly

### 1. Prepare CA Certificate

Place your enterprise CA certificate in this directory:

```bash
cp /path/to/your/ca.crt enterprise-ca.crt
```

### 2. Build Custom Images

```bash
# Build all images (Obot + MCP servers)
./build.sh -r your-registry.example.com

# Or build only specific images
./build.sh --obot-only
./build.sh --mcp-only
```

### 3. Deploy to Kubernetes

```bash
# Generate Helm values
./deploy.sh generate-values

# Edit values-custom.yaml as needed, then install
./deploy.sh install
```

### 4. Verify Deployment

```bash
# Test CA certificate configuration
./test.sh -u https://your-internal-service.example.com

# Or verify Kubernetes deployment
./deploy.sh verify
```

## Files in This Package

### Dockerfiles
- **`Dockerfile.obot-enterprise`**: Custom Obot server image with CA certificate
- **`Dockerfile.mcp-base`**: Custom MCP base image with CA certificate
- **`Dockerfile.nanobot-shim`**: Custom Nanobot shim image (if exists)
- **`Dockerfile.webhook-converter`**: Custom webhook converter image (if exists)

### Scripts
- **`build.sh`**: Automated build script for all images
- **`deploy.sh`**: Kubernetes deployment helper using Helm
- **`test.sh`**: Validation script to test CA certificate configuration

### Usage

Run any script with `-h` or `--help` for detailed options:

```bash
./build.sh --help
./deploy.sh --help
./test.sh --help
```

When deploying Obot in an enterprise environment, you may need to configure trust for internal services that use certificates signed by a private CA. This includes:

- Internal authentication providers (e.g., Keycloak, Active Directory)
- Internal Git repositories or artifact registries
- Internal APIs accessed by MCP servers
- Internal databases or storage services
- Internal LLM providers (vLLM, Ollama, OpenAI-compatible APIs)

Obot consists of two main components that need CA certificate configuration:

1. **Obot Server**: The main control plane (written in Go)
2. **MCP Servers**: Dynamically launched containers that run tools and agents (Python, Node.js, Go)

### Configuration Approaches Comparison

**Recommended Approach: Build Custom Images**

Build custom Docker images that include your enterprise CA certificate. This is the most reliable and production-ready approach:

- ✅ Clean and simple configuration
- ✅ No runtime overhead
- ✅ Consistent across restarts
- ✅ Works in air-gapped environments
- ✅ Follows container best practices
- ✅ Easy to automate with CI/CD

## Prerequisites

- Your enterprise root CA certificate (PEM format)
- Access to the Obot deployment configuration
- For Docker deployments: access to the Docker host filesystem
- For Kubernetes deployments: ability to create ConfigMaps/Secrets

## Configuring Obot Server

### Create Custom Obot Image

#### Step 1: Create Dockerfile

Create a `Dockerfile.obot-enterprise`:

```dockerfile
FROM ghcr.io/obot-platform/obot:latest

# Switch to root to install CA certificate
USER root

# Copy your enterprise CA certificate
COPY enterprise-ca.crt /usr/local/share/ca-certificates/

# Update system CA bundle
RUN update-ca-certificates

# Switch back to original user if needed
# USER obot
```

#### Step 2: Build and Push Image

```bash
# Build custom image
docker build -t your-registry.example.com/obot:enterprise \
  -f Dockerfile.obot-enterprise .

# Push to your registry
docker push your-registry.example.com/obot:enterprise
```

#### Step 3: Use Custom Image

**For Docker:**

```bash
docker run -d \
  --name obot \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v obot-data:/data \
  -p 8080:8080 \
  your-registry.example.com/obot:enterprise
```

**For Kubernetes (Helm):**

```yaml
# values.yaml
image:
  repository: your-registry.example.com/obot
  tag: enterprise
  pullPolicy: IfNotPresent

# If using private registry
imagePullSecrets:
  - name: registry-credentials
```

```bash
helm install obot obot/obot \
  --namespace obot-system \
  --create-namespace \
  -f values.yaml
```

#### Step 4: Verify Configuration

```bash
# Check if Obot can access internal services
docker exec obot curl -v https://your-internal-service.example.com

# Check if CA certificate is in the system bundle
docker exec obot grep -i "your-ca-name" /etc/ssl/certs/ca-certificates.crt

# Check logs for certificate errors
docker logs obot
```

### For Kubernetes Deployments (Helm)

#### Use Custom Image

Build a custom image as described above and configure Helm to use it:

```yaml
# values.yaml
image:
  repository: your-registry.example.com/obot
  tag: enterprise
  pullPolicy: IfNotPresent

imagePullSecrets:
  - name: registry-credentials
```

#### Install or Upgrade Obot

```bash
# Install Obot with CA certificate configuration
helm install obot obot/obot \
  --namespace obot-system \
  --create-namespace \
  -f values.yaml

# Or upgrade existing installation
helm upgrade obot obot/obot \
  --namespace obot-system \
  -f values.yaml
```

#### Verify Configuration

```bash
# Check if pod is running
kubectl get pods -n obot-system

# Verify CA certificate is in the system bundle
kubectl exec -n obot-system deployment/obot -- \
  grep -i "your-ca-name" /etc/ssl/certs/ca-certificates.crt

# Test connection to internal service
kubectl exec -n obot-system deployment/obot -- \
  curl -v https://your-internal-service.example.com
```

## Configuring MCP Servers

MCP servers are dynamically launched by Obot and need separate CA certificate configuration. The recommended approach is to create custom base images that include your enterprise CA certificates.

### Step 1: Create Custom MCP Base Images

#### For MCP Base Image

Create a `Dockerfile.mcp-base`:

```dockerfile
FROM ghcr.io/obot-platform/mcp-images/phat:main

# Copy your enterprise CA certificate
COPY enterprise-ca.crt /usr/local/share/ca-certificates/

# Update system CA certificates bundle
# This makes the CA trusted by Go, Python, curl, and most system tools
RUN update-ca-certificates

# Only needed for Node.js runtime (adds additional CAs beyond system bundle)
ENV NODE_EXTRA_CA_CERTS=/usr/local/share/ca-certificates/enterprise-ca.crt
```

> **Key Point**: Running `update-ca-certificates` merges your CA into `/etc/ssl/certs/ca-certificates.crt`. This system bundle is automatically trusted by:
> - **Go**: Reads from system certificate pool
> - **Python**: Uses system bundle via default CA paths
> - **curl/wget**: Uses system bundle by default
> - **Node.js**: Requires `NODE_EXTRA_CA_CERTS` for additional CAs

#### For Nanobot Shim Image

Create a `Dockerfile.nanobot-shim`:

```dockerfile
FROM ghcr.io/nanobot-ai/nanobot:v0.0.50

# Copy your enterprise CA certificate
COPY enterprise-ca.crt /usr/local/share/ca-certificates/

# Update system CA certificates
RUN update-ca-certificates

# Only needed for Node.js runtime
ENV NODE_EXTRA_CA_CERTS=/usr/local/share/ca-certificates/enterprise-ca.crt
```

#### For HTTP Webhook Converter Image

Create a `Dockerfile.webhook-converter`:

```dockerfile
FROM ghcr.io/obot-platform/mcp-images/http-webhook-mcp-converter:main

# Copy your enterprise CA certificate
COPY enterprise-ca.crt /usr/local/share/ca-certificates/

# Update system CA certificates
RUN update-ca-certificates

# Only needed for Node.js runtime
ENV NODE_EXTRA_CA_CERTS=/usr/local/share/ca-certificates/enterprise-ca.crt
```

### Step 2: Build and Push Custom Images

```bash
# Build MCP base image
docker build -t your-registry.example.com/obot/mcp-base:latest \
  -f Dockerfile.mcp-base .

# Build Nanobot shim image
docker build -t your-registry.example.com/obot/nanobot-shim:latest \
  -f Dockerfile.nanobot-shim .

# Build webhook converter image
docker build -t your-registry.example.com/obot/webhook-converter:latest \
  -f Dockerfile.webhook-converter .

# Push images to your registry
docker push your-registry.example.com/obot/mcp-base:latest
docker push your-registry.example.com/obot/nanobot-shim:latest
docker push your-registry.example.com/obot/webhook-converter:latest
```

### Step 3: Configure Obot to Use Custom Images

#### For Docker Deployments

```bash
docker run -d \
  --name obot \
  -e OBOT_SERVER_MCPBASE_IMAGE=your-registry.example.com/obot/mcp-base:latest \
  -e OBOT_SERVER_MCPREMOTE_SHIM_BASE_IMAGE=your-registry.example.com/obot/nanobot-shim:latest \
  -e OBOT_SERVER_MCPHTTPWEBHOOK_BASE_IMAGE=your-registry.example.com/obot/webhook-converter:latest \
  -e OBOT_SERVER_MCPRUNTIME_BACKEND=docker \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v obot-data:/data \
  -p 8080:8080 \
  ghcr.io/obot-platform/obot:latest
```

#### For Kubernetes Deployments (Helm)

Update your `values.yaml`:

```yaml
# values.yaml

config:
  # Use custom MCP base images with enterprise CA
  OBOT_SERVER_MCPBASE_IMAGE: "your-registry.example.com/obot/mcp-base:latest"
  OBOT_SERVER_MCPREMOTE_SHIM_BASE_IMAGE: "your-registry.example.com/obot/nanobot-shim:latest"
  OBOT_SERVER_MCPHTTPWEBHOOK_BASE_IMAGE: "your-registry.example.com/obot/webhook-converter:latest"
  
  # Ensure Docker runtime backend
  OBOT_SERVER_MCPRUNTIME_BACKEND: "docker"

# If using private registry, configure image pull secrets
imagePullSecrets:
  - name: registry-credentials

# Configure MCP image pull secrets if needed
mcpImagePullSecrets:
  - name: mcp-registry-secret
    registry: your-registry.example.com
    username: your-username
    password: your-password
    email: your-email@example.com
```

Apply the configuration:

```bash
helm upgrade obot obot/obot \
  --namespace obot-system \
  -f values.yaml
```

### Step 4: Verify MCP Server Configuration

#### Test MCP Server Creation

1. Create a test agent or workflow in Obot that uses an MCP server
2. Verify the MCP server container is created with the custom image:

```bash
# List MCP server containers
docker ps --filter "label=mcp.server.id"

# Inspect MCP server container
docker inspect <container-id> | grep Image

# Verify CA certificate in MCP server
docker exec <container-id> ls -l /usr/local/share/ca-certificates/

# Test connection from MCP server to internal service
docker exec <container-id> curl -v https://your-internal-service.example.com
```

## Common Use Cases

### Connecting to Internal LLM Providers

If you're using an internal or self-hosted LLM provider (e.g., internal OpenAI-compatible API, vLLM, Ollama) with a private CA:

#### For Obot Server

The Obot server needs to trust the LLM provider's certificate when making model inference requests:

```yaml
# values.yaml for Kubernetes deployment
extraVolumes:
  - name: enterprise-ca
    configMap:
      name: enterprise-ca

extraVolumeMounts:
  - name: enterprise-ca
    mountPath: /usr/local/share/ca-certificates/enterprise-ca.crt
    subPath: ca.crt
    readOnly: true

config:
  # Configure your internal LLM provider
  OPENAI_API_KEY: "your-api-key"
  # Or use model provider configuration in the UI

# Optional: If you need to run update-ca-certificates at startup
# Add an init container in the deployment
```

For Docker deployment:

```bash
docker run -d \
  --name obot \
  -v /opt/obot/certs/enterprise-ca.crt:/usr/local/share/ca-certificates/enterprise-ca.crt:ro \
  -e OPENAI_API_KEY=your-api-key \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v obot-data:/data \
  -p 8080:8080 \
  --entrypoint /bin/sh \
  ghcr.io/obot-platform/obot:latest \
  -c "update-ca-certificates && exec run.sh"
```

#### For MCP Servers

MCP servers may also need to call LLM providers directly (e.g., when using agents with tool capabilities). Since MCP servers can run Python, Node.js, or Go code, ensure your custom MCP base images include the CA certificate and environment variables for all runtimes:

```dockerfile
# Dockerfile.mcp-base
FROM ghcr.io/obot-platform/mcp-images/phat:main

COPY enterprise-ca.crt /usr/local/share/ca-certificates/
RUN update-ca-certificates

# For Go binaries (reads from system bundle)
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
# For Python (requests, httpx, aiohttp, urllib3)
ENV REQUESTS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
# For curl, wget, and other CLI tools
ENV CURL_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
# For Node.js (adds additional CA beyond system bundle)
ENV NODE_EXTRA_CA_CERTS=/usr/local/share/ca-certificates/enterprise-ca.crt
```

#### Supported LLM Provider Scenarios

1. **Self-hosted OpenAI-compatible APIs** (vLLM, LocalAI, etc.)
   - Configure via Model Providers in Obot UI
   - Set base URL to your internal endpoint: `https://llm.internal.company.com/v1`

2. **Internal Ollama instances**
   - Use Ollama model provider with internal URL
   - Example: `https://ollama.internal.company.com`

3. **Enterprise OpenAI/Anthropic proxies**
   - Configure proxy endpoint with private CA
   - MCP servers will inherit CA trust from base image

4. **Internal embedding services**
   - Used for knowledge base and RAG functionality
   - Both Obot server and MCP servers need CA trust

#### Verification

```bash
# Test LLM provider connection from Obot server
kubectl exec -n obot-system deployment/obot -- \
  curl -v https://llm.internal.company.com/v1/models

# Test from MCP server container
docker exec <mcp-container-id> \
  curl -v https://llm.internal.company.com/v1/models
```

### Connecting to Internal Keycloak

If you're using an internal Keycloak instance with a private CA:

```yaml
# values.yaml for Obot server (Kubernetes)
extraVolumes:
  - name: enterprise-ca
    configMap:
      name: enterprise-ca

extraVolumeMounts:
  - name: enterprise-ca
    mountPath: /usr/local/share/ca-certificates/enterprise-ca.crt
    subPath: ca.crt
    readOnly: true

config:
  OBOT_SERVER_ENABLE_AUTHENTICATION: "true"
  # Keycloak OIDC discovery will now trust your private CA
```

For Docker:

```bash
docker run -d \
  --name obot \
  -v /opt/obot/certs/enterprise-ca.crt:/usr/local/share/ca-certificates/enterprise-ca.crt:ro \
  -e OBOT_SERVER_ENABLE_AUTHENTICATION=true \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v obot-data:/data \
  -p 8080:8080 \
  --entrypoint /bin/sh \
  ghcr.io/obot-platform/obot:latest \
  -c "update-ca-certificates && exec run.sh"
```

### Accessing Internal Git Repositories

For MCP servers that need to access internal Git repositories:

```dockerfile
# Dockerfile.mcp-base
FROM ghcr.io/obot-platform/mcp-images/phat:main

COPY enterprise-ca.crt /usr/local/share/ca-certificates/
RUN update-ca-certificates

# Configure Git to use system CA bundle
RUN git config --system http.sslCAInfo /etc/ssl/certs/ca-certificates.crt

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
```

### Using Internal NPM/PyPI Registries

If your MCP servers need to access internal package registries:

```dockerfile
# Dockerfile.mcp-base
FROM ghcr.io/obot-platform/mcp-images/phat:main

COPY enterprise-ca.crt /usr/local/share/ca-certificates/
RUN update-ca-certificates

# Configure npm to use custom CA
RUN npm config set cafile /etc/ssl/certs/ca-certificates.crt

# Configure pip to use custom CA
RUN pip config set global.cert /etc/ssl/certs/ca-certificates.crt

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV REQUESTS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
```

## Troubleshooting

### Certificate Verification Errors

If you see errors like:

```
x509: certificate signed by unknown authority
SSL certificate problem: unable to get local issuer certificate
Error: unable to verify the first certificate
failed to connect to LLM provider: certificate verify failed
```

**Diagnosis:**

```bash
# For Obot server
kubectl exec -n obot-system deployment/obot -- \
  openssl s_client -connect your-service.example.com:443 -CApath /etc/ssl/certs

# For MCP server
docker exec <mcp-container-id> \
  openssl s_client -connect your-service.example.com:443 -CApath /etc/ssl/certs
```

**Solutions:**

1. Verify CA certificate is properly mounted
2. Check file permissions (should be readable)
3. Ensure CA certificate is in PEM format
4. Verify environment variables are set correctly

### MCP Servers Using Old Images

If MCP servers are not using your custom images:

**Diagnosis:**

```bash
# Check current MCP server images
docker ps --filter "label=mcp.server.id" --format "{{.Image}}"

# Check Obot configuration
docker exec obot env | grep MCPBASE_IMAGE
```

**Solutions:**

1. Verify configuration environment variables are set
2. Restart Obot server to pick up new configuration
3. Remove old MCP server containers and let Obot recreate them

### CA Certificate Not Found in Container

**Diagnosis:**

```bash
# Check if CA certificate exists
docker exec <container-id> ls -l /usr/local/share/ca-certificates/
docker exec <container-id> cat /etc/ssl/certs/ca-certificates.crt | grep "Your CA Name"
```

**Solutions:**

1. Rebuild custom images ensuring COPY command succeeds
2. Run `update-ca-certificates` in the Dockerfile
3. Verify the source CA certificate file exists during build

### Multiple CA Certificates

If you need to trust multiple CAs:

```dockerfile
FROM ghcr.io/obot-platform/mcp-images/phat:main

# Copy all CA certificates
COPY ca-certificates/*.crt /usr/local/share/ca-certificates/

# Update CA bundle (merges all CAs into system bundle)
RUN update-ca-certificates

# Only needed for Node.js
ENV NODE_EXTRA_CA_CERTS=/usr/local/share/ca-certificates/
```

## Security Considerations

1. **CA Certificate Distribution**: Store CA certificates securely in your CI/CD pipeline or secret management system
2. **Image Registry Security**: Use private registries for custom images and configure appropriate authentication
3. **Regular Updates**: Rebuild custom images when CA certificates are rotated or updated
4. **Least Privilege**: Only include necessary CA certificates, not the entire certificate chain unless required
5. **Audit Trail**: Keep records of which CA certificates are trusted and why

## Technical Background: How Different Runtimes Handle CA Certificates

Understanding how each runtime locates and trusts CA certificates helps explain why the configuration differs:

### Go Runtime (Obot Server)

- **Obot server is written in Go** and uses Go's `crypto/tls` package
- Go reads CA certificates from the **system certificate pool**
- After running `update-ca-certificates`, Go automatically trusts your CA
- Location: `/etc/ssl/certs/ca-certificates.crt`
- **No environment variables needed** (system pool is used by default)

### Python Runtime (MCP Servers)

- Many MCP servers use Python with libraries like `requests`, `httpx`, `urllib3`
- These libraries **automatically use the system certificate bundle** by default
- After running `update-ca-certificates`, Python trusts your CA
- **No environment variables needed** in most cases
- Exception: Only set `REQUESTS_CA_BUNDLE` or `SSL_CERT_FILE` if using non-standard Python installations

### Node.js Runtime (MCP Servers, UI)

- Node.js has its **own certificate store** separate from the system
- `NODE_EXTRA_CA_CERTS` tells Node.js to load additional CA certificates
- Points to `/usr/local/share/ca-certificates/enterprise-ca.crt` (or directory)
- **This is the only environment variable you need to set**
- Node.js will combine its built-in CAs with your enterprise CA

### System Tools (curl, wget, git)

- Use the system certificate bundle by default
- After running `update-ca-certificates`, they automatically trust your CA
- **No environment variables needed**

### Summary

| Runtime | Trusts System Bundle After `update-ca-certificates`? | Additional ENV Required? |
|---------|------------------------------------------------------|-------------------------|
| **Go (Obot)** | ✅ Yes (automatically) | ❌ No |
| **Python** | ✅ Yes (automatically) | ❌ No (usually) |
| **Node.js** | ❌ No (separate store) | ✅ Yes (`NODE_EXTRA_CA_CERTS`) |
| **System tools** | ✅ Yes (automatically) | ❌ No |

**Key Takeaway**: After running `update-ca-certificates`, only Node.js requires the `NODE_EXTRA_CA_CERTS` environment variable. All other runtimes automatically trust the updated system bundle.

## Automation with CI/CD

### Automated Custom Image Builds

#### GitHub Actions Example

```yaml
name: Build Obot Enterprise Image

on:
  push:
    branches: [main]
  schedule:
    # Rebuild weekly for base image updates
    - cron: '0 0 * * 0'
  workflow_dispatch:

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Fetch CA Certificate
        run: |
          echo "${{ secrets.ENTERPRISE_CA_CERT }}" > enterprise-ca.crt
      
      - name: Build Obot Image
        run: |
          docker build -t your-registry.example.com/obot:enterprise \
            -f Dockerfile.obot-enterprise .
      
      - name: Login to Registry
        run: |
          echo "${{ secrets.REGISTRY_PASSWORD }}" | \
            docker login your-registry.example.com \
            -u "${{ secrets.REGISTRY_USERNAME }}" --password-stdin
      
      - name: Push Image
        run: |
          docker push your-registry.example.com/obot:enterprise

  build-mcp-images:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        image:
          - name: mcp-base
            dockerfile: Dockerfile.mcp-base
          - name: nanobot-shim
            dockerfile: Dockerfile.nanobot-shim
          - name: webhook-converter
            dockerfile: Dockerfile.webhook-converter
    steps:
      - uses: actions/checkout@v3
      
      - name: Fetch CA Certificate
        run: |
          echo "${{ secrets.ENTERPRISE_CA_CERT }}" > enterprise-ca.crt
      
      - name: Build ${{ matrix.image.name }}
        run: |
          docker build \
            -t your-registry.example.com/obot/${{ matrix.image.name }}:latest \
            -f ${{ matrix.image.dockerfile }} .
      
      - name: Login and Push
        run: |
          echo "${{ secrets.REGISTRY_PASSWORD }}" | \
            docker login your-registry.example.com \
            -u "${{ secrets.REGISTRY_USERNAME }}" --password-stdin
          docker push your-registry.example.com/obot/${{ matrix.image.name }}:latest
```

#### GitLab CI Example

```yaml
# .gitlab-ci.yml
variables:
  REGISTRY: your-registry.example.com
  IMAGE_NAME: obot

stages:
  - build
  - push

build:obot:
  stage: build
  image: docker:latest
  services:
    - docker:dind
  script:
    - echo "$ENTERPRISE_CA_CERT" > enterprise-ca.crt
    - docker build -t $REGISTRY/$IMAGE_NAME:enterprise -f Dockerfile.obot-enterprise .
    - echo "$REGISTRY_PASSWORD" | docker login $REGISTRY -u "$REGISTRY_USERNAME" --password-stdin
    - docker push $REGISTRY/$IMAGE_NAME:enterprise
  only:
    - main
    - schedules
```

### Automated Deployment Updates

After building new images, automatically update your deployments:

```yaml
# Add to GitHub Actions workflow
- name: Update Kubernetes Deployment
  run: |
    kubectl set image deployment/obot \
      obot=$REGISTRY/obot:enterprise-${{ github.sha }} \
      -n obot-system
    
    # Or trigger Helm upgrade
    helm upgrade obot obot/obot \
      --namespace obot-system \
      --set image.tag=enterprise-${{ github.sha }} \
      --reuse-values
```

## References

- [Server Configuration](./server-configuration.md)
- [MCP Deployments in Kubernetes](./mcp-deployments-in-kubernetes.md)
- [Auth Providers](./auth-providers.md)
