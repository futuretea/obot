---
name: obot-k8s-devops
description: |
  基于 obot 项目 chart/ 和 .github/workflows/ 的 Kubernetes 安全加固、Helm chart 设计、CI/CD 流水线与容器安全实践指南。涵盖 Pod Security Admission (PSA) 命名空间标签、容器/Pod 级安全上下文、NetworkPolicy 网络隔离、Helm 条件渲染、多阶段 Dockerfile、CI 版本字符串简化、防御性配置校验、ServiceAccount 云 IAM 集成等核心模式。
  Kubernetes security hardening, Helm chart design, CI/CD pipeline, and container security patterns from the obot project. Covers Pod Security Admission (PSA) namespace labels, container/pod-level security contexts, NetworkPolicy network isolation, Helm conditional rendering, multi-stage Dockerfile, CI version string simplification, defensive configuration validation, ServiceAccount cloud IAM integration, and documentation-driven security practices.
---

# Obot Kubernetes DevOps 安全与部署模式

## 适用范围

本 Skill 适用于 obot 项目中 Kubernetes 部署、安全加固与 CI/CD 相关的场景：

- **Pod 安全加固**：为 MCP server Pod 配置 PSA 标签和安全上下文
- **网络隔离**：设计 NetworkPolicy 限制 Pod 间通信和出站流量
- **Helm chart 扩展**：添加条件渲染的 init container、volume、ServiceAccount
- **容器镜像构建**：多阶段 Dockerfile 优化与层缓存
- **CI/CD 流水线**：版本字符串管理与构建流程简化
- **防御性配置**：启动时校验关键配置项，避免运行时故障

---

## 核心原则

### 1. 纵深防御（Defense in Depth）

安全不依赖单一机制，而是在多个层面叠加防护：

```
Namespace PSA 标签（准入控制）
    ↓
Pod-level SecurityContext（进程隔离）
    ↓
Container-level SecurityContext（能力裁剪）
    ↓
NetworkPolicy（网络隔离）
    ↓
ServiceAccount 最小权限（云 IAM 绑定）
```

每一层独立生效，即使某层被绕过，其余层仍提供保护。

### 2. 安全默认值（Secure by Default）

所有安全配置默认启用，用户需要显式关闭而非显式开启：

```yaml
# Helm values.yaml — 安全选项默认开启
mcpNamespace:
  podSecurity:
    enabled: true          # 默认启用 PSA
    enforce: restricted    # 最严格级别
```

### 3. 文档驱动安全（Documentation-Driven Security）

每个安全特性的代码变更必须伴随对应文档，说明：
- 该特性防御什么威胁
- 如何配置和自定义
- 关闭后的风险

---

## 操作流程

### 流程 1：Pod Security Admission (PSA) 命名空间标签

为 MCP server Pod 所在命名空间配置 PSA 标签，在准入层强制执行安全策略：

```yaml
# Helm values.yaml
mcpNamespace:
  podSecurity:
    enabled: true
    enforce: restricted        # 拒绝不合规 Pod
    enforceVersion: latest     # 跟随集群最新 PSA 版本
    audit: restricted          # 审计日志记录违规
    warn: restricted           # 向用户显示警告
```

对应 Helm 模板中的命名空间标签渲染：

```yaml
# templates/mcp-namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: {{ .Values.mcpNamespace.name }}
  labels:
    {{- if .Values.mcpNamespace.podSecurity.enabled }}
    pod-security.kubernetes.io/enforce: {{ .Values.mcpNamespace.podSecurity.enforce }}
    pod-security.kubernetes.io/enforce-version: {{ .Values.mcpNamespace.podSecurity.enforceVersion }}
    pod-security.kubernetes.io/audit: {{ .Values.mcpNamespace.podSecurity.audit }}
    pod-security.kubernetes.io/warn: {{ .Values.mcpNamespace.podSecurity.warn }}
    {{- end }}
```

**规则**：
- `enforce: restricted` 是生产环境唯一推荐级别
- `audit` 和 `warn` 同步设为 `restricted`，确保违规行为可追溯
- `enforceVersion: latest` 避免锁定旧版本策略

### 流程 2：容器级安全上下文（Container SecurityContext）

每个容器必须配置完整的安全上下文，裁剪所有不必要的能力：

```go
SecurityContext: &corev1.SecurityContext{
    AllowPrivilegeEscalation: &[]bool{false}[0],
    RunAsNonRoot:             &[]bool{true}[0],
    RunAsUser:                &[]int64{1000}[0],
    RunAsGroup:               &[]int64{1000}[0],
    Capabilities: &corev1.Capabilities{
        Drop: []corev1.Capability{"ALL"},
    },
    SeccompProfile: &corev1.SeccompProfile{
        Type: corev1.SeccompProfileTypeRuntimeDefault,
    },
},
```

**逐字段说明**：

| 字段 | 值 | 作用 |
|------|-----|------|
| `AllowPrivilegeEscalation` | `false` | 禁止通过 setuid/setgid 提权 |
| `RunAsNonRoot` | `true` | 强制非 root 运行，kubelet 层面拒绝 root 容器 |
| `RunAsUser` / `RunAsGroup` | `1000` | 指定非特权 UID/GID |
| `Capabilities.Drop` | `ALL` | 丢弃所有 Linux capabilities |
| `SeccompProfile.Type` | `RuntimeDefault` | 启用容器运行时默认 seccomp 配置文件 |

**Go 惯用法**：`&[]bool{false}[0]` 是 Go 中创建 `*bool` 指针的惯用写法（Kubernetes API 中 `*bool` 字段区分"未设置"和"设为 false"）。

### 流程 3：Pod 级安全上下文（PodSecurityContext）

Pod 级安全上下文为所有容器提供统一的安全基线，并通过 `FSGroup` 管理共享卷权限：

```go
SecurityContext: &corev1.PodSecurityContext{
    RunAsNonRoot: &[]bool{true}[0],
    RunAsUser:    &[]int64{1000}[0],
    RunAsGroup:   &[]int64{1000}[0],
    FSGroup:      &[]int64{1000}[0],
    SeccompProfile: &corev1.SeccompProfile{
        Type: corev1.SeccompProfileTypeRuntimeDefault,
    },
},
```

**Pod 级 vs 容器级的关系**：
- Pod 级设置作为所有容器的默认值
- 容器级设置可覆盖 Pod 级（但不应放宽安全策略）
- `FSGroup` 只能在 Pod 级设置，确保挂载卷的文件归属正确的 GID

### 流程 4：NetworkPolicy 网络隔离

为 MCP server Pod 设计精细的网络策略，遵循最小权限原则：

**入站规则（Ingress）**：仅允许来自 obot 主命名空间的流量

```yaml
ingress:
  - from:
      - namespaceSelector:
          matchLabels:
            kubernetes.io/metadata.name: {{ .Values.obotNamespace }}
```

**出站规则（Egress）**：允许 DNS 解析 + obot 服务通信 + 公网访问，阻断私有网段

```yaml
egress:
  # 1. 允许 DNS 解析（UDP 53）
  - to:
      - namespaceSelector: {}
    ports:
      - protocol: UDP
        port: 53

  # 2. 允许访问 obot 服务
  - to:
      - namespaceSelector:
          matchLabels:
            kubernetes.io/metadata.name: {{ .Values.obotNamespace }}

  # 3. 允许公网访问，阻断所有私有 IP 段
  - to:
      - ipBlock:
          cidr: 0.0.0.0/0
          except:
            - 10.0.0.0/8        # RFC 1918 私有地址
            - 172.16.0.0/12     # RFC 1918 私有地址
            - 192.168.0.0/16    # RFC 1918 私有地址
            - 127.0.0.0/8       # 回环地址
            - 169.254.0.0/16    # 链路本地地址
            - 224.0.0.0/4       # 组播地址
            - 240.0.0.0/4       # 保留地址
```

**设计意图**：
- MCP server Pod 可能需要访问外部 API（公网），但不应访问集群内其他服务
- 阻断私有 IP 段防止 SSRF 攻击横向移动到集群内部服务
- DNS 放行确保域名解析正常工作

### 流程 5：Helm 条件渲染——Init Container 与加密配置

使用条件渲染实现可选的加密配置初始化，通过 init container 生成配置文件：

```yaml
{{- if eq .Values.config.OBOT_SERVER_ENCRYPTION_PROVIDER "custom" }}
initContainers:
  - name: encryptionsetup
    image: {{ template "system_default_registry" . }}{{ .Values.image.repository }}:{{ .Values.image.tag }}
    command:
    - /bin/sh
    - -c
    - |
      cat > /config/encryption.yaml <<EOF
      kind: EncryptionConfiguration
      apiVersion: apiserver.config.k8s.io/v1
      resources:
        - resources:
            - secrets
          providers:
            - aescbc:
                keys:
                  - name: key1
                    secret: {{ .Values.config.OBOT_SERVER_ENCRYPTION_KEY | quote }}
            - identity: {}
      EOF
    volumeMounts:
      - name: config
        mountPath: /config
{{- end }}
```

**敏感数据卷使用 `emptyDir` + `Memory` 介质**：

```yaml
volumes:
  - name: config
    emptyDir:
      medium: Memory    # 存储在 tmpfs（内存），不落盘
      sizeLimit: 1Mi
```

**规则**：
- 加密密钥等敏感数据绝不写入持久卷
- `medium: Memory` 确保数据仅存在于内存中，Pod 销毁后自动清除
- `sizeLimit` 防止内存滥用

### 流程 6：多阶段 Dockerfile 构建

使用独立的构建阶段隔离不同组件，通过 `--link` 优化层缓存：

```dockerfile
# 阶段 1：构建工具二进制
FROM golang:1.25 AS tools-build
WORKDIR /src
COPY tools/ ./tools/
RUN cd tools && go build -o /out/tools ./...

# 阶段 2：构建 Provider
FROM golang:1.25 AS providers-build
WORKDIR /src
COPY providers/ ./providers/
RUN cd providers && go build -o /out/providers ./...

# 阶段 3：构建企业工具
FROM golang:1.25 AS enterprise-tools-build
WORKDIR /src
COPY enterprise-tools/ ./enterprise-tools/
RUN cd enterprise-tools && go build -o /out/enterprise-tools ./...

# 阶段 4：最终镜像——使用 --link 优化层缓存
FROM gcr.io/distroless/static:nonroot AS final
COPY --link --from=tools-build /out/tools /usr/local/bin/
COPY --link --from=providers-build /out/providers /usr/local/bin/
COPY --link --from=enterprise-tools-build /out/enterprise-tools /usr/local/bin/
COPY --link --from=0 /obot /usr/local/bin/obot

USER 1000:1000
ENTRYPOINT ["/usr/local/bin/obot"]
```

**`--link` 的作用**：
- 使 `COPY` 层独立于前序层，提升缓存命中率
- 当某个构建阶段的产物未变化时，对应的 `COPY --link` 层可直接复用
- 显著减少 CI 构建时间

### 流程 7：CI 版本字符串简化

将复杂的 YAML 解析替换为简单的逗号分隔 key=value 格式：

```bash
# ❌ 旧方式：依赖 yq 解析 YAML
TOOLS_VERSION=$(yq '.tools.version' versions.yaml)
PROVIDERS_VERSION=$(yq '.providers.version' versions.yaml)

# ✅ 新方式：简单的 key=value 格式
# versions.txt 内容：
# tools=v1.2.3,providers=v2.0.1,enterprise-tools=v1.0.0

# 解析逻辑
IFS=',' read -ra PAIRS <<< "$(cat versions.txt)"
for pair in "${PAIRS[@]}"; do
    key="${pair%%=*}"
    value="${pair#*=}"
    declare "${key^^}_VERSION=$value"
done
```

**优势**：
- 消除对 `yq` 等外部工具的依赖
- 减少 CI 镜像体积和构建步骤
- 格式简单，不易出错

### 流程 8：防御性配置校验

在服务启动时校验关键配置项，快速失败而非运行时异常：

```go
func validateConfig(cfg Config) error {
    // 数据库 DSN 必须配置（除非启用特殊开发模式）
    if cfg.DSN == "" && !cfg.DevMode {
        return fmt.Errorf("OBOT_DSN is required; set OBOT_DEV_MODE=true for embedded database")
    }

    // 加密 Provider 为 custom 时，密钥不能为空
    if cfg.EncryptionProvider == "custom" && cfg.EncryptionKey == "" {
        return fmt.Errorf("OBOT_SERVER_ENCRYPTION_KEY is required when encryption provider is 'custom'")
    }

    return nil
}
```

**规则**：
- 所有必需配置在启动阶段校验，不延迟到首次使用时
- 错误信息明确指出缺失的配置项和修复方式
- 提供合理的开发模式旁路（如 `DevMode`），但生产环境不可绕过

### 流程 9：ServiceAccount 与云 IAM 集成

Helm chart 中 ServiceAccount 默认创建，支持注解绑定云 IAM 角色：

```yaml
# values.yaml
serviceAccount:
  create: true
  name: ""                    # 留空则自动生成
  annotations: {}
  # 示例：AWS IRSA
  # annotations:
  #   eks.amazonaws.com/role-arn: arn:aws:iam::123456789:role/obot-role
  # 示例：GCP Workload Identity
  # annotations:
  #   iam.gke.io/gcp-service-account: obot@project.iam.gserviceaccount.com
```

对应 Helm 模板：

```yaml
# templates/serviceaccount.yaml
{{- if .Values.serviceAccount.create }}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "obot.serviceAccountName" . }}
  labels:
    {{- include "obot.labels" . | nindent 4 }}
  {{- with .Values.serviceAccount.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
{{- end }}
```

**规则**：
- `create: true` 为默认值，确保开箱即用
- 注解支持任意云厂商的 IAM 绑定机制
- ServiceAccount 名称可自定义，便于与已有 IAM 策略对接

### 流程 10：文档与代码同步变更

每个安全特性的 PR 必须包含：

```
PR 结构示例：
├── chart/values.yaml                    # 新增安全配置项
├── chart/templates/networkpolicy.yaml   # NetworkPolicy 模板
├── pkg/mcp/docker.go                    # Go 代码中的安全上下文
├── docs/docs/configuration/             # 配置文档
│   ├── pod-security.md                  # PSA 配置说明
│   └── network-policy.md               # NetworkPolicy 配置说明
└── docs/docs/architecture/              # 架构文档
    └── security-model.md               # 安全模型概述
```

**文档内容要求**：
- 说明该特性防御的威胁模型
- 提供完整的配置示例（含默认值和自定义值）
- 列出关闭该特性的风险和影响
- 包含故障排查指南（如 Pod 因 PSA 被拒绝时的排查步骤）

---

## 注意事项

### 关键约束

1. **PSA `restricted` 级别的兼容性要求**
   - 所有容器必须 `runAsNonRoot: true`
   - 不能使用 `hostNetwork`、`hostPID`、`hostIPC`
   - 不能挂载 `hostPath` 卷
   - 必须 drop `ALL` capabilities
   - 必须设置 seccomp profile
   - 违反任一条件，Pod 将被准入控制器拒绝

2. **NetworkPolicy 的 DNS 放行不可遗漏**
   - 忘记放行 UDP 53 会导致所有域名解析失败
   - Pod 表现为网络不通，但实际是 DNS 问题
   - 排查时优先检查 DNS 连通性

3. **`emptyDir` + `Memory` 的资源限制**
   - `medium: Memory` 占用 Pod 的内存配额
   - 必须设置 `sizeLimit` 防止 OOM
   - 不设置 `sizeLimit` 时，kubelet 不会限制 tmpfs 大小

4. **`--link` 仅在 BuildKit 下生效**
   - 确保 CI 环境启用 BuildKit（`DOCKER_BUILDKIT=1`）
   - 不支持 BuildKit 的环境会忽略 `--link` 标志

5. **ServiceAccount 注解的云厂商差异**
   - AWS IRSA：`eks.amazonaws.com/role-arn`
   - GCP Workload Identity：`iam.gke.io/gcp-service-account`
   - Azure Workload Identity：`azure.workload.identity/client-id`
   - 注解键名不可混用

### 常见陷阱

| 问题 | 影响 | 解决方案 |
|------|------|----------|
| PSA 标签设为 `baseline` 而非 `restricted` | 允许特权容器运行 | 生产环境始终使用 `restricted` |
| 容器只设 `RunAsNonRoot` 不设 `RunAsUser` | 镜像默认 UID 可能为 0 | 显式指定 `RunAsUser: 1000` |
| NetworkPolicy 未阻断 `169.254.0.0/16` | 可通过 metadata API 获取云凭证 | 将链路本地地址加入 except 列表 |
| init container 未设安全上下文 | PSA `restricted` 下 Pod 被拒绝 | init container 同样需要完整安全上下文 |
| Helm 模板缺少 `{{- if }}` 条件守卫 | 空值渲染为无效 YAML | 所有可选配置块用条件包裹 |
| CI 版本字符串含空格或换行 | 解析失败 | 使用 `trim` 或 `tr -d '\n'` 清理 |

---

## 反模式（避免）

| 反模式 | 正确做法 |
|--------|----------|
| 安全上下文只设 Pod 级，不设容器级 | Pod 级和容器级都显式设置，容器级不应放宽 Pod 级策略 |
| `Capabilities.Add: ["NET_BIND_SERVICE"]` 随意添加 | 先 Drop ALL，仅在有明确需求时逐个添加并注释原因 |
| NetworkPolicy 出站规则 `to: []`（允许所有） | 显式列出允许的目标，阻断私有 IP 段 |
| 敏感配置写入 ConfigMap | 使用 Secret + `emptyDir` Memory 介质 |
| Dockerfile 单阶段构建 | 多阶段构建，最终镜像使用 distroless/nonroot |
| CI 中使用 `yq`/`jq` 解析简单配置 | 简单 key=value 格式，减少外部依赖 |
| 启动时不校验配置，运行时 panic | `validateConfig()` 在启动阶段快速失败 |
| ServiceAccount `create: false` 为默认值 | `create: true` 为默认值，开箱即用 |
| 安全特性代码无配套文档 | 代码与文档在同一 PR 中同步提交 |

---

## 相关 Skills

- [obot-service-wiring](../obot-service-wiring/SKILL.md)：服务依赖注入与配置标签规范
- [obot-controller-handler](../obot-controller-handler/SKILL.md)：Controller 调谐逻辑（MCP server Pod 创建）
- [obot-mcp-runtime](../obot-mcp-runtime/SKILL.md)：MCP 运行时实现（Docker/Kubernetes Pod 管理）
- [review-go-style](../review-go-style/SKILL.md)：Go 代码风格审查
