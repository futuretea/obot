---
name: obot-mcp-runtime
description: |
  基于 obot 项目 pkg/mcp/ 的 MCP 运行时实现，涵盖 Docker/Kubernetes 双后端部署、容器生命周期管理、K8s 网络与服务发现、审计日志集成、Gateway 到反向代理的架构演进等核心实践。
  MCP runtime implementation patterns covering Docker/Kubernetes dual-backend deployment, container lifecycle management, K8s networking and service discovery, audit log integration, and the gateway-to-reverse-proxy architectural evolution.
---

# Obot MCP 运行时实现模式

## 适用范围

本 Skill 适用于在 `pkg/mcp/` 下新增或修改 MCP 服务器部署逻辑的场景：

- **Docker 后端**：`pkg/mcp/docker.go` - 容器创建、启动、等待、删除
- **Kubernetes 后端**：`pkg/mcp/kubernetes.go` - Deployment、Service、Secret 管理
- **后端接口**：`pkg/mcp/backend.go` - 双后端抽象与健康检查
- **配置加载**：`pkg/mcp/loader.go` - Options struct 与后端选择
- **类型定义**：`pkg/mcp/types.go` - ServerConfig 与运行时类型
- **审计日志**：`pkg/api/handlers/mcpgateway/auditlog.go` - Token 认证端点
- **服务配置**：`pkg/services/config.go` - 依赖注入与后端初始化

---

## 核心原则

### 1. 双后端接口抽象

Docker 和 Kubernetes 后端实现统一的 `backend` 接口，新功能必须并行添加到两个后端：

```go
// pkg/mcp/backend.go:23-33
type backend interface {
    ensureServerDeployment(ctx context.Context, serverConfig ServerConfig, webhooks []Webhook) (ServerConfig, error)
    deployServer(ctx context.Context, server ServerConfig, webhooks []Webhook) error
    transformConfig(ctx context.Context, serverConfig ServerConfig) (*ServerConfig, error)
    streamServerLogs(ctx context.Context, id string) (io.ReadCloser, error)
    getServerDetails(ctx context.Context, id string) (types.MCPServerDetails, error)
    restartServer(ctx context.Context, id string) error
    shutdownServer(ctx context.Context, id string) error
}
```

### 2. Options Struct 配置模式

使用 Options struct 替代位置参数，支持环境变量注入和默认值：

```go
// pkg/mcp/loader.go:28-66
type Options struct {
    MCPBaseImage            string   `usage:"The base image to use for MCP containers" default:"ghcr.io/obot-platform/mcp-images/phat:main"`
    MCPRemoteShimBaseImage  string   `usage:"The base image to use for MCP remote shim containers" default:"ghcr.io/nanobot-ai/nanobot:v0.0.52"`
    MCPNamespace            string   `usage:"The namespace to use for MCP containers" default:"obot-mcp"`
    MCPClusterDomain        string   `usage:"The cluster domain to use for MCP containers" default:"cluster.local"`
    MCPRuntimeBackend       string   `usage:"The runtime backend: docker, kubernetes, or local" default:"docker"`
    MCPImagePullSecrets     []string `usage:"Image pull secret names for pulling MCP images"`

    // Kubernetes pod scheduling
    MCPK8sSettingsAffinity         string `env:"OBOT_SERVER_MCPK8S_SETTINGS_AFFINITY"`
    MCPK8sSettingsTolerations      string `env:"OBOT_SERVER_MCPK8S_SETTINGS_TOLERATIONS"`
    MCPK8sSettingsResources        string `env:"OBOT_SERVER_MCPK8S_SETTINGS_RESOURCES"`
    MCPK8sSettingsRuntimeClassName string `env:"OBOT_SERVER_MCPK8S_SETTINGS_RUNTIMECLASSNAME"`

    // Obot service FQDN construction
    ServiceName      string `env:"OBOT_SERVER_SERVICE_NAME"`
    ServiceNamespace string `env:"OBOT_SERVER_SERVICE_NAMESPACE"`

    // Audit log
    MCPAuditLogPersistIntervalSeconds int `default:"5"`
    MCPAuditLogsPersistBatchSize      int `default:"1000"`
}
```

### 3. 容器竞争条件处理

使用随机化命名而非复杂锁机制消除竞争：

```go
// Docker 后端：等待容器删除而非报错
if err := d.client.ContainerRemove(ctx, existing.ID, container.RemoveOptions{Force: true}); cerrdefs.IsConflict(err) {
    statusCh, errCh := d.client.ContainerWait(ctx, existing.ID, container.WaitConditionRemoved)
    select {
    case err := <-errCh:
        if err != nil && !cerrdefs.IsNotFound(err) {
            return fmt.Errorf("error waiting for stopped container: %w", err)
        }
    case <-statusCh:
    }
}

// 随机化 init 容器名称避免冲突
fmt.Sprintf("%s-init-%s", containerName, strings.ToLower(rand.Text()))
```

### 4. K8s 安全上下文（Defense-in-Depth）

Pod 和 Container 级别双重安全加固：

```go
// pkg/mcp/kubernetes.go - Pod 级别
SecurityContext: &corev1.PodSecurityContext{
    RunAsNonRoot: &[]bool{true}[0],
    RunAsUser:    &[]int64{1000}[0],
    RunAsGroup:   &[]int64{1000}[0],
    FSGroup:      &[]int64{1000}[0],
    SeccompProfile: &corev1.SeccompProfile{
        Type: corev1.SeccompProfileTypeRuntimeDefault,
    },
},

// Container 级别
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

### 5. K8s 网络与 URL 转换

在 K8s 环境中将 localhost URL 转换为 Service FQDN，所有 scheme 统一为 http（集群内部通信）：

```go
// pkg/mcp/kubernetes.go:268
func (k *kubernetesBackend) replaceHostWithServiceFQDN(urlStr string) string {
    idx := strings.Index(urlStr, "://")
    if idx == -1 { return urlStr }
    rest := urlStr[idx+3:]
    pathIdx := strings.Index(rest, "/")
    var path string
    if pathIdx != -1 { path = rest[pathIdx:] }
    return fmt.Sprintf("http://%s%s", k.serviceFQDN, path)
}
```

### 6. 类型化错误处理与 Pod 状态分析

将基础设施错误映射为语义化错误类型，区分瞬态与永久性故障：

```go
// 错误类型定义
var (
    ErrHealthCheckFailed    = errors.New("health check failed")
    ErrHealthCheckTimeout   = errors.New("health check timeout")
    ErrNotSupportedByBackend = errors.New("not supported by backend")
    ErrPodCrashLoopBackOff  = errors.New("pod crash loop backoff")
    ErrImagePullFailed      = errors.New("image pull failed")
    ErrPodSchedulingFailed  = errors.New("pod scheduling failed")
)

// Pod 状态分析：区分可重试与不可重试错误
// pkg/mcp/kubernetes.go:864-932
func analyzePodStatus(pod *corev1.Pod) (retryable bool, err error) {
    switch pod.Status.Phase {
    case corev1.PodFailed:
        return false, fmt.Errorf("%w: pod Failed: %s", ErrHealthCheckTimeout, pod.Status.Message)
    case corev1.PodSucceeded:
        return false, fmt.Errorf("%w: pod exited", ErrHealthCheckTimeout)
    }

    for _, cs := range pod.Status.ContainerStatuses {
        if cs.State.Waiting != nil {
            switch cs.State.Waiting.Reason {
            case "ContainerCreating", "PodInitializing":
                return true, fmt.Errorf("container %s is %s", cs.Name, cs.State.Waiting.Reason)
            case "ImagePullBackOff", "ErrImagePull":
                return true, fmt.Errorf("%w: %s: %s", ErrImagePullFailed, cs.Name, cs.State.Waiting.Message)
            case "CrashLoopBackOff":
                return false, fmt.Errorf("%w: %s: %s", ErrPodCrashLoopBackOff, cs.Name, cs.State.Waiting.Message)
            }
        }
    }
    return true, fmt.Errorf("pod phase %s, waiting", pod.Status.Phase)
}

// API Handler 层映射为 HTTP 错误
if errors.Is(err, mcp.ErrHealthCheckFailed) || errors.Is(err, mcp.ErrHealthCheckTimeout) {
    return types.NewErrHTTP(http.StatusServiceUnavailable,
        fmt.Sprintf("MCP server for agent %s is not healthy", agent.Name))
}
if nse := (*mcp.ErrNotSupportedByBackend)(nil); errors.As(err, &nse) {
    return types.NewErrHTTP(http.StatusBadRequest, nse.Error())
}
```

### 7. 健康检查模式

区分 nanobot 和 containerized 服务器的健康检查策略：

```go
// pkg/mcp/backend.go:53-158
func ensureServerReady(ctx context.Context, url string, server ServerConfig) error {
    ctx, cancel := context.WithTimeout(ctx, time.Minute)
    defer cancel()
    client := &http.Client{Timeout: time.Second}

    // nanobot 服务器：轮询 /healthz 端点
    if server.Runtime != types.RuntimeContainerized {
        url = fmt.Sprintf("%s/healthz", url)
        for {
            resp, err := client.Get(url)
            if err == nil {
                resp.Body.Close()
                switch resp.StatusCode {
                case http.StatusOK:
                    return nil
                case http.StatusServiceUnavailable:
                    return ErrHealthCheckFailed
                }
            }
            select {
            case <-ctx.Done():
                return ErrHealthCheckTimeout
            case <-time.After(100 * time.Millisecond):
            }
        }
    }
    // containerized 服务器：尝试 POST with SSE or JSON
    // ...
}
```

### 8. Gateway 反向代理架构

MCP 协议处理下沉到 nanobot shim 容器，Gateway 仅做薄代理 + 认证：

```go
func (h *Handler) Proxy(req api.Context) error {
    if req.User.GetUID() == "anonymous" {
        return apierrors.NewUnauthorized("user is not authenticated")
    }
    mcpURL, err := h.ensureServerIsDeployed(req)
    if err != nil {
        return err
    }
    return h.mcpSessionManager.LaunchServer(req.Context(), mcpServerConfig)
}
```

---

## 操作流程

### 流程 1：K8s 资源生成

`k8sObjects()` 方法生成完整的 K8s 资源集：

1. **3 个 Secret**：files（文件挂载）、config（环境变量）、webhook-secrets（Webhook 密钥）
2. **1 个 Deployment**：包含安全上下文、调度策略、资源限制
3. **1 个 Service**：ClusterIP 类型，端口 80 → http

```go
// Secret 命名规范
name.SafeConcatName(server.MCPServerName, "files")
name.SafeConcatName(server.MCPServerName, "config")
name.SafeConcatName(server.MCPServerName, "webhook", "secrets")

// Deployment 标签
Labels: map[string]string{
    "app":         server.MCPServerName,
    "mcp-user-id": podLabelUserID,
}
```

### 流程 2：审计日志 Token 认证

MCP 服务器通过 Token 提交审计日志，使用 hash 索引查找：

```go
// pkg/api/handlers/mcpgateway/auditlog.go:85-135
func (h *AuditLogHandler) SubmitAuditLogs(req api.Context) error {
    token := strings.TrimPrefix(req.Request.Header.Get("Authorization"), "Bearer ")
    if token == "" {
        return types.NewErrHTTP(http.StatusUnauthorized, "no token provided")
    }

    // 通过 hash 索引查找 MCP Server（不存储明文 token）
    var mcpServers v1.MCPServerList
    if err := req.List(&mcpServers, &kclient.ListOptions{
        FieldSelector: fields.OneTermEqualSelector("auditLogTokenHash", hash.Digest(token)),
    }); err != nil {
        return err
    }
    if len(mcpServers.Items) != 1 {
        return types.NewErrHTTP(http.StatusUnauthorized, "invalid token")
    }

    // 验证每条日志归属正确的 MCP Server
    for _, auditLog := range auditLogs {
        if auditLog.MCPID != mcpServerName {
            return types.NewErrForbidden("audit log does not belong to MCP server %q", mcpServerName)
        }
    }
    return nil
}
```

### 流程 3：K8s Settings Hash 变更检测

通过配置 hash 检测 K8s 调度设置变更，触发 Deployment 更新：

```go
// pkg/mcp/kubernetes.go:1173-1205
func ComputeK8sSettingsHash(settings v1.K8sSettingsSpec) string {
    var buf bytes.Buffer
    if settings.Affinity != nil {
        affinityJSON, _ := json.Marshal(settings.Affinity)
        buf.Write(affinityJSON)
    }
    if len(settings.Tolerations) > 0 {
        tolerationsJSON, _ := json.Marshal(settings.Tolerations)
        buf.Write(tolerationsJSON)
    }
    if settings.Resources != nil {
        resourcesJSON, _ := json.Marshal(settings.Resources)
        buf.Write(resourcesJSON)
    }
    if buf.Len() == 0 {
        return "none"
    }
    return hash.Digest(buf.String())
}
```

### 流程 4：新增 MCP 服务器类型

1. 在 `pkg/mcp/types.go` 的 `ServerConfig` 添加新字段
2. 在 `docker.go` 实现 `transformConfig` 转换逻辑
3. 在 `kubernetes.go` 实现对应逻辑（Secret、Deployment 环境变量）
4. 在 `loader.go` 的 `Options` 添加配置项
5. 编写表驱动测试覆盖边界条件

### 流程 5：添加运行时新功能

1. 定义功能开关环境变量（`Options` struct）
2. 在 Docker 后端添加对应容器配置
3. 在 Kubernetes 后端添加 Secret/环境变量
4. 在 Gateway 层添加认证/验证端点
5. 端到端测试验证双后端行为一致

---

## 注意事项

### 关键约束

1. **随机化命名**：容器/Init 容器名称必须包含随机字符串避免冲突
2. **后端一致性**：任何新功能必须同时添加到 Docker 和 K8s 后端
3. **错误映射**：基础设施错误转换为 HTTP 错误时不泄露内部实现细节
4. **安全上下文**：所有容器必须设置 `AllowPrivilegeEscalation: false`、`Drop: ALL`
5. **审计日志 Token**：使用 `hash.Digest(token)` 索引，不存储明文
6. **URL 转换**：K8s 内部通信统一使用 `http://` scheme
7. **指针字面量**：Go K8s API 使用 `&[]bool{true}[0]` 模式创建指针

### 常见陷阱

| 问题 | 影响 | 解决方案 |
|------|------|----------|
| Init 容器名称冲突 | 部署失败 | 使用 `rand.Text()` 随机后缀 |
| 单后端实现新功能 | 环境不一致 | 双后端并行实现 |
| 健康检查无超时 | 请求挂起 | `context.WithTimeout(ctx, time.Minute)` |
| Pod CrashLoopBackOff 重试 | 无限等待 | `analyzePodStatus` 区分可重试/不可重试 |
| K8s Settings 变更未检测 | Deployment 不更新 | `ComputeK8sSettingsHash` 注解比对 |

---

## 反模式

| 反模式 | 正确做法 |
|--------|----------|
| 在 Gateway 层实现 MCP 协议逻辑 | 下沉到 nanobot shim 容器 |
| 使用锁解决容器竞争 | 随机命名 + ContainerWait |
| 单后端实现新功能 | 双后端并行实现 |
| 错误直接返回给客户端 | 映射为 `types.NewErrHTTP` |
| 存储明文 audit token | 使用 `hash.Digest(token)` 索引 |
| 位置参数传递后端配置 | Options struct |
| 忽略 Pod 状态分析 | `analyzePodStatus` 区分故障类型 |

---

## 相关 Skills

- [obot-gateway-security](../obot-gateway-security/SKILL.md)：API Key 认证与网关安全
- [obot-k8s-devops](../obot-k8s-devops/SKILL.md)：K8s 安全加固与 Helm 部署
- [obot-fullstack-mcp](../obot-fullstack-mcp/SKILL.md)：Composite MCP 服务器与策略评估
- [obot-service-wiring](../obot-service-wiring/SKILL.md)：后端依赖注入与初始化
