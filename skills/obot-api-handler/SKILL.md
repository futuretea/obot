---
name: obot-api-handler
description: |
  基于 obot 项目 pkg/api/handlers/ 的 API Handler 设计模式指南。涵盖 Handler 结构体的依赖注入、api.Context 的使用、SSE 流式响应、存储 CRUD 操作、用户权限检查等核心实践。
  API handler design patterns for the obot project. Covers handler struct dependency injection, api.Context usage, SSE streaming, storage CRUD, user role checks, and error conventions drawn from pkg/api/handlers/.
---

# Obot API Handler 设计模式

## 适用范围

本 Skill 适用于在 `pkg/api/handlers/` 下新增或修改 API Handler 的场景：

- **新增资源接口**：为新的 Kubernetes CR 类型添加 CRUD API
- **流式响应**：实现 SSE（Server-Sent Events）推送
- **Handler 重构**：统一依赖注入与错误处理风格
- **路由注册**：在 `pkg/api/router/router.go` 中注册新端点

---

## 核心原则

### 1. Handler 结构体 + 构造函数

每个资源的 Handler 是一个独立的结构体，所有依赖通过构造函数注入，**不使用全局变量**：

```go
type AgentHandler struct {
    mcpSessionManager *mcp.SessionManager
    invoker           *invoke.Invoker
    dispatcher        *dispatcher.Dispatcher
    serverURL         string
}

func NewAgentHandler(dispatcher *dispatcher.Dispatcher, mcpSessionManager *mcp.SessionManager, invoker *invoke.Invoker, serverURL, internalServerURL string) *AgentHandler {
    return &AgentHandler{
        serverURL:         serverURL,
        invoker:           invoker,
        dispatcher:        dispatcher,
        mcpSessionManager: mcpSessionManager,
    }
}
```

### 2. api.Context 是统一的请求上下文

所有 Handler 方法接收 `api.Context`，它组合了 HTTP 读写、存储、网关客户端和用户信息：

```go
type Context struct {
    http.ResponseWriter
    *http.Request
    GPTClient     *gptscript.GPTScript  // 具体类型，非接口
    Storage       storage.Client
    GatewayClient *gclient.Client
    User          user.Info             // k8s.io/apiserver/pkg/authentication/user.Info
    APIBaseURL    string
}
```

方法签名统一为：
```go
func (h *FooHandler) Get(req api.Context) error
func (h *FooHandler) List(req api.Context) error
func (h *FooHandler) Create(req api.Context) error
func (h *FooHandler) Update(req api.Context) error
func (h *FooHandler) Delete(req api.Context) error
```

### 3. 错误处理：始终包装上下文

返回错误时使用 `fmt.Errorf("context: %w", err)`，**绝不静默吞掉错误**：

```go
// ✅ 正确：包装错误上下文
if err := req.Get(&agent, id); err != nil {
    return fmt.Errorf("failed to get agent %s: %w", id, err)
}

// ❌ 错误：丢弃错误
req.Get(&agent, id)

// ❌ 错误：既打日志又返回
log.Error(err)
return err
```

---

## 操作流程

### 流程 1：读取请求数据

```go
// 读取 Path 参数
id := req.PathValue("id")

// 读取请求 Body（反序列化为结构体）
var manifest types.AgentManifest
if err := req.Read(&manifest); err != nil {
    return err
}

// 读取原始 Body bytes（带大小限制）
data, err := req.Body()
// 或指定大小限制
data, err = req.Body(api.BodyOptions{MaxBytes: 1 * 1024 * 1024})
```

### 流程 2：存储 CRUD 操作

`api.Context` 内置了常用存储方法，自动绑定 namespace：

```go
// 获取单个资源
var agent v1.Agent
if err := req.Get(&agent, id); err != nil {
    return err
}

// 列出资源（可附加过滤条件）
var agentList v1.AgentList
if err := req.List(&agentList); err != nil {
    return err
}
// 带字段过滤
if err := req.List(&agentList, &kclient.ListOptions{
    FieldSelector: fields.OneTermEqualSelector("spec.userID", userID),
}); err != nil {
    return err
}

// 创建资源
if err := req.Create(&agent); err != nil {
    return err
}

// 更新资源
if err := req.Update(&agent); err != nil {
    return err
}

// 删除资源（404 自动忽略）
if err := req.Delete(&agent); err != nil {
    return err
}
```

### 流程 3：写入响应

```go
// 200 OK，返回 JSON
return req.Write(obj)

// 201 Created
return req.WriteCreated(obj)

// 自定义状态码
return req.WriteCode(obj, http.StatusAccepted)
```

### 流程 4：SSE 流式响应

```go
func (h *AgentHandler) Run(req api.Context) error {
    // 启动异步任务，拿到事件 channel
    events, err := h.invoker.Start(req.Context(), ...)
    if err != nil {
        return err
    }

    // WriteEvents 自动检测 Accept: text/event-stream
    // - SSE 请求：逐条推送 event
    // - JSON 请求：收集后一次性返回 {"items": [...]}
    // - 其他：直接写入 Content 字段
    return req.WriteEvents(events)
}
```

若需要手动控制 SSE：
```go
if req.IsStreamRequested() {
    req.ResponseWriter.Header().Set("Content-Type", "text/event-stream")
    _ = req.WriteDataEvent(myEvent)
    req.Flush()
}
```

### 流程 5：用户权限检查

```go
// 在 authz 路径规则已通过后，handler 内的角色检查仅用于“同一路径下差异化行为”
// 例如：admin 展示额外字段，普通用户不展示
if req.UserIsAdmin() {
    return req.Write(fullAdminView(agent))
}
return req.Write(publicView(agent))
```

> 角色检查不替代 authz 路径规则注册。若某路径仅允许 admin 访问，应在 `adminAndOwnerRules` 中注册，不应将限制逻辑全部塞进 handler。

可用角色检查方法：

```go
req.UserIsAdmin()         // types.GroupAdmin
req.UserIsOwner()         // types.GroupOwner
req.UserIsPowerUser()     // types.GroupPowerUser
req.UserIsAuthenticated() // types.GroupAuthenticated
req.UserIsAuditor()       // types.GroupAuditor

// 获取用户 ID
userID := req.UserID()       // uint
userUID := req.User.GetUID() // string（Kubernetes user UID）
```

### 流程 6：打破导入循环——定义本地接口

当 Handler 需要来自其他包的功能，但直接导入会产生循环时，在 Handler 文件内定义窄接口：

```go
// pkg/api/handlers/mcp.go

// MCPOAuthChecker 定义在 handler 文件内，打破 import cycle
type MCPOAuthChecker interface {
    CheckForMCPAuth(req api.Context, server v1.MCPServer, config mcp.ServerConfig, userID, mcpID, oauthAppAuthRequestID string) (string, error)
}

type MCPHandler struct {
    mcpOAuthChecker MCPOAuthChecker  // 接受任何实现该接口的类型
    // ...
}
```

### 流程 7：路由注册

在 `pkg/api/router/router.go` 中：

```go
func Router(ctx context.Context, services *services.Services) (http.Handler, error) {
    mux := services.APIServer

    // 1. 构造 Handler（注入依赖）
    agents := handlers.NewAgentHandler(services.ProviderDispatcher, services.MCPLoader, services.Invoker, services.ServerURL, services.InternalServerURL)

    // 2. 注册路由（Method + Path 组合）
    mux.HandleFunc("GET /api/agents", agents.List)
    mux.HandleFunc("POST /api/agents", agents.Create)
    mux.HandleFunc("GET /api/agents/{id}", agents.Get)
    mux.HandleFunc("PUT /api/agents/{id}", agents.Update)
    mux.HandleFunc("DELETE /api/agents/{id}", agents.Delete)

    return mux, nil
}
```

---

## 注意事项

### 关键约束

1. **不要在 Handler 中直接调用 `http.Error`**
   - 始终返回 `error`，由中间件统一处理
   - 使用 `types.NewErrHTTP(code, msg)` 返回带 HTTP 状态码的错误

2. **namespace 已内置**
   - `req.Get/List/Create` 自动使用 `system.DefaultNamespace`
   - 不要手动拼装 namespace

3. **Body 有默认大小限制**
   - `req.Body()` 默认最大 8MB
   - 超出时自动返回 `413 Request Entity Too Large`

4. **Handler 文件命名**
   - 一个文件对应一个资源类型：`agent.go`, `mcp.go`, `threads.go`
   - 大文件（如 `mcp.go` 有 3634 行）可接受，保持资源内聚

### 常见陷阱

| 问题 | 影响 | 解决方案 |
|------|------|----------|
| 直接引用 `authz` 包 | import cycle | 定义本地窄接口 |
| 忘记 `req.Body()` 读完后关闭 | 无影响（已自动 discard） | 无需手动关闭 |
| 在 SSE handler 外层再 `Write` | 双重写入 | 先判断 `IsStreamRequested()` |
| 错误只打日志不返回 | 调用者无感知 | 只 return，不打日志 |

---

## 反模式（避免）

| 反模式 ❌ | 正确做法 ✅ |
|----------|------------|
| `var globalClient = ...`（全局变量） | 构造函数注入 |
| `http.Error(w, msg, code)` | `return types.NewErrHTTP(code, msg)` |
| `log.Error(err); return err` | `return fmt.Errorf("context: %w", err)` |
| 直接 `import "pkg/api/authz"` 导致循环 | 定义本地接口 |
| `req.Storage.Get(ctx, key, obj)` 手动拼 namespace | `req.Get(obj, name)` |

---

## 相关 Skills

- [obot-authz](../obot-authz/SKILL.md)：Authorization 授权机制
- [obot-validation](../obot-validation/SKILL.md)：请求参数校验
- [obot-service-wiring](../obot-service-wiring/SKILL.md)：服务依赖注入
- [review-go-style](../review-go-style/SKILL.md)：Go 代码风格审查
