# Image CA Wrap

`image-ca-wrap.sh` builds new image tags from existing images after adding an enterprise CA certificate to the image trust store.

It is intentionally project-agnostic: it does not know about Obot, Helm values, MCP images, or runtime-specific environment variables.

## Usage

```bash
./image-ca-wrap.sh \
  --ca-cert enterprise-ca.crt \
  --image ghcr.io/example/app:v1=registry.example.com/team/app:v1-ca \
  --image ghcr.io/example/worker:v2=registry.example.com/team/worker:v2-ca
```

Push the resulting tags:

```bash
./image-ca-wrap.sh \
  --ca-cert enterprise-ca.crt \
  --image ghcr.io/example/app:v1=registry.example.com/team/app:v1-ca \
  --push
```

Build and push a multi-platform tag:

```bash
./image-ca-wrap.sh \
  --ca-cert enterprise-ca.crt \
  --image ghcr.io/example/app:v1=registry.example.com/team/app:v1-ca \
  --platform linux/amd64,linux/arm64 \
  --push
```

## Notes

- `--image` is `source=target`; the source image is used as `BASE_IMAGE`, and the target is the new tag.
- Multi-platform builds must use `--push` because Docker cannot load a multi-platform image into the local image store.
- The generated image defaults to `USER root`, matching the previous Obot packaging behavior. Use `--final-user` if a wrapped image must end as a different user.
- The source image needs enough shell and certificate tooling for the generic Containerfile to append the CA bundle.
