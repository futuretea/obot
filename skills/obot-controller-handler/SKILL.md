---
name: obot-controller-handler
description: |
  基于 obot 项目 pkg/controller/handlers/ 的 Controller Reconciliation 设计模式指南。涵盖使用 nah/router 的调谐循环、Handler 结构体、状态更新、路由注册、generationed 模式、资源清理等核心实践。
  Controller reconciliation design patterns for the obot project using nah/router. Covers handler struct, reconcile method signature, status updates, route registration, generationed pattern, and resource cleanup drawn from pkg/controller/handlers/.
---

# Obot Controller Handler 设计模式

## 适用范围

本 Skill 适用于在 `pkg/controller/handlers/` 下新增或修改 Controller Handler 的场景：

- **新增资源控制器**：为新的 Kubernetes CR 类型实现调谐逻辑
- **状态管理**：更新资源 `.Status` 字段
- **资源清理**：实现 finalizer 或 cleanup handler
- **路由注册**：在 `pkg/controller/routes.go` 中注册控制器路由

---

## 核心原则

### 1. 每个资源一个独立包

Controller Handler 按资源类型拆分到独立包，路径规则为：

```
pkg/controller/handlers/<resource-name>/<resource-name>.go
```

示例：
```
pkg/controller/handlers/mcpserver/mcpserver.go
pkg/controller/handlers/agents/agents.go
pkg/controller/handlers/threads/threads.go
```

### 2. Handler 结构体 + 构造函数

```go
package mcpserver

import (
    "github.com/obot-platform/obot/logger"
    // ...
)

// 包级 logger，统一命名
var log = logger.Package()

type Handler struct {
    gptClient         *gptscript.GPTScript
    mcpSessionManager *mcp.SessionManager
    baseURL           string
}

func New(gptClient *gptscript.GPTScript, mcpSessionManager *mcp.SessionManager, baseURL string) *Handler {
    return &Handler{
        gptClient:         gptClient,
        mcpSessionManager: mcpSessionManager,
        baseURL:           baseURL,
    }
}
```

### 3. Reconcile 方法签名

所有调谐方法使用统一签名：

```go
func (h *Handler) Reconcile(req router.Request, resp router.Response) error
```

- `req.Object`：当前被调谐的资源（需要类型断言）
- `req.Client`：Kubernetes client（用于 Get/List/Update）
- `req.Ctx`：请求 context（含 cancellation）
- `resp`：可用于触发重新调谐（`resp.RetryAfter(duration)`）

---

## 操作流程

### 流程 1：获取并类型断言当前资源

```go
func (h *Handler) Reconcile(req router.Request, _ router.Response) error {
    // 类型断言，获取具体类型
    server := req.Object.(*v1.MCPServer)

    // 早期返回：条件不满足时直接跳过
    if server.Spec.MCPServerCatalogEntryName == "" {
        return nil
    }

    // 业务逻辑...
    return nil
}
```

### 流程 2：读取关联资源

```go
func (h *Handler) DetectDrift(req router.Request, _ router.Response) error {
    server := req.Object.(*v1.MCPServer)

    // 获取关联资源
    var entry v1.MCPServerCatalogEntry
    if err := req.Get(&entry, server.Namespace, server.Spec.MCPServerCatalogEntryName); apierrors.IsNotFound(err) {
        // 关联资源不存在时直接返回，不报错
        return nil
    } else if err != nil {
        return err
    }

    // 处理关联资源...
    return nil
}
```

### 流程 3：更新资源状态

```go
func (h *Handler) SyncStatus(req router.Request, _ router.Response) error {
    server := req.Object.(*v1.MCPServer)

    // 计算新状态
    newStatus := computeStatus(server)

    // 只在状态实际变化时更新，避免无限调谐
    if server.Status.NeedsUpdate != newStatus {
        server.Status.NeedsUpdate = newStatus
        // 使用 Status().Update() 更新 status 子资源
        return req.Client.Status().Update(req.Ctx, server)
    }
    return nil
}
```

更新 Spec（主资源）：
```go
return req.Client.Update(req.Ctx, server)
```

### 流程 4：使用 retry 处理并发冲突

```go
import "k8s.io/client-go/util/retry"

func (h *Handler) UpdateWithRetry(req router.Request, _ router.Response) error {
    server := req.Object.(*v1.MCPServer)

    return retry.RetryOnConflict(retry.DefaultRetry, func() error {
        // 重新获取最新版本
        if err := req.Get(server, server.Namespace, server.Name); err != nil {
            return err
        }
        // 修改字段
        server.Spec.SomeField = "new-value"
        return req.Client.Update(req.Ctx, server)
    })
}
```

### 流程 5：generationed 模式（跳过未变更资源）

当资源 Spec 未变化时，可通过 `generationed` 包跳过重复调谐：

```go
import "github.com/obot-platform/obot/pkg/controller/generationed"

func (h *Handler) Reconcile(req router.Request, resp router.Response) error {
    obj := req.Object.(*v1.Agent)

    // 如果 generation 未变化，跳过
    if generationed.IsUpToDate(obj) {
        return nil
    }

    // 执行调谐逻辑...

    // 标记 generation 已处理
    generationed.SetGeneration(obj)
    return req.Client.Status().Update(req.Ctx, obj)
}
```

### 流程 6：使用 untriggered 注册不触发调谐的 Watch

```go
import "github.com/obot-platform/nah/pkg/untriggered"

// 在 routes.go 中：
// Watch MCPServerCatalogEntry 变化，但不以其为主触发源
root.On(mcpserverHandler.DetectDrift, &v1.MCPServer{}).
    WatchingFor(untriggered.Of(&v1.MCPServerCatalogEntry{}))
```

### 流程 7：路由注册（controller/routes.go）

```go
func (c *Controller) setupRoutes() {
    root := c.router  // c.router = services.Router（在 New() 中注入）

    // 1. 构造 Handler
    mcpserver := mcpserver.New(c.services.GPTClient, c.services.MCPLoader, c.services.ServerURL)

    // 2. 注册单资源调谐（主路由）
    root.On(mcpserver.Reconcile, &v1.MCPServer{})

    // 3. 注册多个 Handler 方法到同一资源
    root.On(mcpserver.DetectDrift, &v1.MCPServer{})
    root.On(mcpserver.DetectK8sSettingsDrift, &v1.MCPServer{})

    // 4. 注册清理 Handler（finalizer）
    root.On(handlers.GCOrphans, &v1.MCPServer{})
}
```

### 流程 8：定期重试（RetryAfter）

```go
func (h *Handler) PollStatus(req router.Request, resp router.Response) error {
    server := req.Object.(*v1.MCPServer)

    if !serverIsReady(server) {
        // 5 秒后重新调谐
        resp.RetryAfter(5 * time.Second)
        return nil
    }

    return h.syncStatus(req, server)
}
```

---

## 注意事项

### 关键约束

1. **状态更新只用 `Status().Update()`**
   - Spec 更新用 `client.Update()`
   - Status 更新用 `client.Status().Update()`
   - 两者混用会导致冲突

2. **IsNotFound 不是错误**
   - 关联资源不存在时，`apierrors.IsNotFound(err)` 应 `return nil`
   - 不要把 NotFound 作为错误上报

3. **避免无限调谐循环**
   - 仅在状态实际变化时调用 `Update()`
   - 使用 `generationed` 跳过未变更资源

4. **包级 logger 命名统一**
   - 每个 handler 包都声明 `var log = logger.Package()`
   - 不要使用 `fmt.Println` 或 `log.Printf`

5. **Handler 方法不改变主资源 Spec**
   - 除非有明确业务需求
   - Spec 变更应由用户（API 层）驱动，不由 Controller 自动修改

### 常见陷阱

| 问题 | 影响 | 解决方案 |
|------|------|----------|
| 忘记处理 `IsNotFound` | panic 或错误日志 | 检查并 `return nil` |
| 每次调谐都调用 `Status().Update()` | 无限循环 | 先比较再更新 |
| 在 reconcile 中启动 goroutine 无 context | goroutine 泄漏 | 传递 `req.Ctx` |
| 两个 Handler 方法修改同一字段 | 竞态冲突 | 拆分为不同字段或加锁 |

---

## 反模式（避免）

| 反模式 ❌ | 正确做法 ✅ |
|----------|------------|
| `obj := req.Object.(v1.MCPServer)` 值断言 | `obj := req.Object.(*v1.MCPServer)` 指针断言 |
| 每次都 `Status().Update()` 不做比较 | 先 diff 再决定是否更新 |
| `fmt.Println("debug info")` | `log.Infof("debug info")` |
| `req.Client.Update(ctx, obj)` 更新 Status | `req.Client.Status().Update(ctx, obj)` |
| Handler 跨越多个无关资源类型 | 每个资源一个独立包/Handler |

---

## 相关 Skills

- [obot-service-wiring](../obot-service-wiring/SKILL.md)：服务依赖注入与路由初始化
- [obot-authz](../obot-authz/SKILL.md)：授权机制（API 层，非 Controller 层）
- [review-go-style](../review-go-style/SKILL.md)：Go 代码风格审查
- [improve-code-readability](../improve-code-readability/SKILL.md)：代码可读性提升
