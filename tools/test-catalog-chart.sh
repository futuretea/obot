#!/usr/bin/env bash
set -euo pipefail

for key in OBOT_SERVER_DEFAULT_MCPCATALOG_PATH OBOT_SERVER_DEFAULT_SYSTEM_MCPCATALOG_PATH; do
    rendered=$(helm template obot chart --set dev.useEmbeddedDb=true --show-only templates/internal-configmap.yaml)
    [[ "$rendered" != *"$key:"* ]]

    rendered=$(helm template obot chart --set dev.useEmbeddedDb=true --set-string "config.$key=" --show-only templates/internal-configmap.yaml)
    [[ "$rendered" == *"$key: \"\""* ]]

    rendered=$(helm template obot chart --set dev.useEmbeddedDb=true --set-string "config.$key=https://catalog.example/catalog" --show-only templates/internal-configmap.yaml)
    [[ "$rendered" == *"$key: \"https://catalog.example/catalog\""* ]]

    rendered=$(helm template obot chart --set dev.useEmbeddedDb=true --set "config.$key=null" --show-only templates/internal-configmap.yaml)
    [[ "$rendered" != *"$key:"* ]]
done

values=$(python3 - <<'PY'
from pathlib import Path
print(Path('packaging/enterprise-ca/README.md').read_text().split('```yaml\n', 1)[1].split('```', 1)[0])
PY
)
rendered=$(helm template obot chart --set dev.useEmbeddedDb=true --show-only templates/internal-configmap.yaml -f /dev/stdin <<< "$values")
[[ "$rendered" == *'OBOT_SERVER_DEFAULT_MCPCATALOG_PATH: ""'* ]]
[[ "$rendered" == *'OBOT_SERVER_DEFAULT_SYSTEM_MCPCATALOG_PATH: ""'* ]]
