# Obot Enterprise CA Images

Build wrapper images that trust a local enterprise CA certificate. The wrapper keeps the source image contents and adds
the CA to the image trust store.

Put the real PEM certificate at `enterprise-ca.crt`, or pass `CA_CERT=/path/to/ca.crt`. Do not commit the real
certificate.

## Build

```bash
cd packaging/enterprise-ca
cp enterprise-ca.crt.example enterprise-ca.crt
# Replace enterprise-ca.crt with the real PEM certificate.

make build
```

This wraps the Obot image and the core MCP runtime images. To push multi-platform images:

```bash
make push PLATFORM=linux/amd64,linux/arm64
```

Useful targets:

```bash
make build        # Obot + core MCP runtime images
make build-obot   # Obot image only
make build-mcp    # Core MCP runtime images only
make build-all-mcp # Core + optional catalog tool images
make push         # Build and push Obot + core MCP runtime images
make push-all     # Build and push Obot + all MCP images
```

## Tags

Target tags are derived from source image tags:

```text
ghcr.io/futuretea/obot:latest
  -> harbor.futuretea.me/obot/obot:latest-ca

ghcr.io/obot-platform/mcp-images/stdio-wrapper:v0.24.2
  -> harbor.futuretea.me/obot/stdio-wrapper:v0.24.2-ca

ghcr.io/obot-platform/nanobot-agent:v0.0.91
  -> harbor.futuretea.me/obot/nanobot-agent:v0.0.91-ca
```

Override the Obot source image tag with `PACKAGE_VERSION`:

```bash
make build PACKAGE_VERSION=v0.25.4
```

Override source images directly:

```bash
OBOT_BASE_IMAGE=obot:local \
STDIO_WRAPPER_BASE_IMAGE=ghcr.io/obot-platform/mcp-images/stdio-wrapper:v0.24.2 \
make build
```

Override target tags with `OBOT_TAG` or `MCP_TAG` only when you need fixed tags.

## Helm Values

Point the chart at the wrapped images:

```yaml
image:
  repository: harbor.futuretea.me/obot/obot
  tag: "latest-ca"
config:
  OBOT_SERVER_MCPBASE_IMAGE: "harbor.futuretea.me/obot/stdio-wrapper:v0.24.2-ca"
  OBOT_SERVER_NANOBOT_AGENT_IMAGE: "harbor.futuretea.me/obot/nanobot-agent:v0.0.91-ca"
extraEnv:
  OBOT_SERVER_DEFAULT_MCPCATALOG_PATH: ""
  OBOT_SERVER_DEFAULT_SYSTEM_MCPCATALOG_PATH: ""
```

Use `extraEnv` for empty catalog path overrides. Empty `config` values are omitted from the generated Secret.

## Generic Wrapper

Use the reusable wrapper directly for one-off images:

```bash
./image-ca-wrap/image-ca-wrap.sh \
  --ca-cert enterprise-ca.crt \
  --image ghcr.io/example/app:v1=registry.example.com/team/app:v1-ca \
  --push
```
