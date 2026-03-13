---
name: obot-service-wiring
description: |
  基于 obot 项目 pkg/services/config.go 的服务依赖注入与初始化设计指南。涵盖 services.Services 中心结构体、配置标签规范、两阶段初始化（PreStart/PostStart）、nah Router 集成、循环依赖规避、后台 goroutine 管理等核心实践。
  Service dependency injection and initialization patterns for the obot project. Covers the services.Services central struct, config tag conventions, two-phase init (PreStart/PostStart), nah Router integration, circular import avoidance, and background goroutine management drawn from pkg/services/config.go.
---

# Obot 服务依赖注入设计模式

## 适用范围

本 Skill 适用于在 `pkg/services/` 下扩展或修改服务初始化的场景：

- **新增服务组件**：向 `services.Services` 添加新的依赖字段
- **配置扩展**：通过标签注解添加新的 CLI flag 或环境变量
- **初始化顺序**：理解 `New()` / `PreStart()` / `PostStart()` 三阶段生命周期
- **循环依赖规避**：通过本地接口打破 import cycle

---

## 核心原则

### 1. services.Services 是唯一的依赖容器

所有运行时组件都挂在 `services.Services` 结构体上，通过指针传递：

```go
type Services struct {
    // 存储与数据库
    StorageClient   storage.Client
    GatewayClient   *gclient.Client
    GPTClient       *gptscript.GPTScript

    // HTTP 服务
    APIServer       *server.Server
    Router          *router.Router

    // 业务组件
    Invoker         *invoke.Invoker
    MCPLoader       *mcp.SessionManager
    Events          *events.Emitter

    // 辅助工具
    AccessControlRuleHelper *accesscontrolrule.Helper
    ModelAccessPolicyHelper *modelaccesspolicy.Helper

    // 配置值（非指针，值类型）
    ServerURL           string
    InternalServerURL   string
    MCPRuntimeBackend   string
    // ...
}
```

**规则**：
- 所有 Handler 和 Controller 只接受 `*services.Services` 或从中提取的具体依赖
- 不直接在组件间互相持有引用（通过 Services 中转）

### 2. 配置通过标签注解

配置结构体使用标签声明 CLI flag、环境变量、默认值和说明：

```go
type Config struct {
    // name: CLI flag 名称
    // env: 环境变量（优先级高于 flag）
    // usage: 帮助文本
    // default: 默认值
    ServerURL   string `name:"server-url" env:"OBOT_SERVER_URL" usage:"Public server URL" default:"http://localhost:8080"`
    AuthEnabled bool   `name:"auth-enabled" env:"OBOT_AUTH_ENABLED" usage:"Enable authentication" default:"true"`
    MaxWorkers  int    `name:"max-workers" usage:"Maximum number of workers" default:"10"`
}
```

子系统配置通过类型别名内嵌：

```go
// 类型别名——复用其他包的 Options 结构，避免重复定义
type (
    GatewayConfig     gserver.Options      // 来自 pkg/gateway/server
    AuditConfig       audit.Options        // 来自 pkg/api/server/audit
    RateLimiterConfig ratelimiter.Options
    EncryptionConfig  encryption.Options
    MCPConfig         mcp.Options
)

// Services 配置结构（组合所有子配置）
type Config struct {
    GatewayConfig
    AuditConfig
    MCPConfig
    // ...
}
```

---

## 操作流程

### 流程 1：三阶段初始化生命周期

```
New(ctx, cfg)          → 构造 Services，初始化所有组件，不做 I/O
    ↓
PreStart(ctx)          → 执行 Kubernetes 引导操作（创建默认资源、迁移数据）
    ↓
PostStart(ctx)         → 启动后台 goroutine（监控、同步、清理任务）
```

**New()：纯构造，无副作用**

```go
func New(ctx context.Context, cfg Config) (*Services, error) {
    // 按依赖顺序初始化各组件
    db, err := db.New(cfg.DSN)
    if err != nil {
        return nil, fmt.Errorf("failed to init database: %w", err)
    }

    gatewayClient := gclient.New(db)

    storageClient, err := storage.New(ctx, cfg.StorageConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to init storage: %w", err)
    }

    return &Services{
        StorageClient: storageClient,
        GatewayClient: gatewayClient,
        ServerURL:     cfg.ServerURL,
        // ...
    }, nil
}
```

**PreStart()：Kubernetes 引导操作**

```go
// pkg/controller/controller.go
func (c *Controller) PreStart(ctx context.Context) error {
    // 创建默认数据（如默认 Agent、默认模型别名）
    if err := data.Data(ctx, c.services.StorageClient, c.services.AgentsDir); err != nil {
        return fmt.Errorf("failed to apply data: %w", err)
    }

    // 确保默认配置存在
    if err := ensureDefaultUserRoleSetting(ctx, c.services.StorageClient); err != nil {
        return fmt.Errorf("failed to ensure default user role setting: %w", err)
    }

    // 执行数据库迁移
    if err := c.migrate(ctx); err != nil {
        return fmt.Errorf("migration failed: %w", err)
    }

    return nil
}
```

**PostStart()：后台 goroutine（Controller 中的实际签名）**

```go
// pkg/controller/controller.go
// 注意：实际的 PostStart 接收两个参数（controller 特定）
func (c *Controller) PostStart(ctx context.Context, client kclient.Client) {
    // 启动后台定期任务，传入 ctx 用于 cancellation
    go c.toolRefHandler.PollRegistries(ctx, client)
}
```

### 流程 2：Router 注册（nah 框架）

`services.Router` 是 `*router.Router`（来自 `github.com/obot-platform/nah`），用于 Controller 订阅 Kubernetes 资源变更：

```go
// Controller 通过 services.Router 注册路由
func (c *Controller) setupRoutes() {
    root := c.router  // = c.services.Router

    // 订阅 v1.MCPServer 资源变更，调用 handler.Reconcile
    root.On(mcpserverHandler.Reconcile, &v1.MCPServer{})
}

// PostStart 启动路由监听
func (c *Controller) PostStart(ctx context.Context) error {
    return c.router.Start(ctx)
}
```

### 流程 3：打破 import cycle——本地接口

当 A 包需要 B 包的功能，但 B 包已经 import 了 A 包时，在 A 包中定义窄接口：

```go
// pkg/api/handlers/mcp.go
// 问题：handlers 不能直接 import mcpgateway/oauth（会循环）
// 解决：在使用处定义接口，依赖注入时传入实现

// 在 handlers 包内定义接口（而非 import 具体实现）
type MCPOAuthChecker interface {
    CheckForMCPAuth(
        req api.Context,
        server v1.MCPServer,
        config mcp.ServerConfig,
        userID, mcpID, oauthAppAuthRequestID string,
    ) (string, error)
}

type MCPHandler struct {
    mcpOAuthChecker MCPOAuthChecker  // 接受接口，不依赖具体类型
}

// pkg/api/router/router.go
// 在不会产生循环的地方，注入具体实现
oauthChecker := oauth.NewMCPOAuthHandlerFactory(...)  // 具体类型
mcp := handlers.NewMCPHandler(..., oauthChecker, ...)  // 赋值给接口字段
```

### 流程 4：import 组织规范

```go
import (
    // 1. 标准库
    "context"
    "fmt"
    "os"

    // 2. 第三方库（模块路径不以 github.com/obot-platform/obot/ 开头）
    "github.com/adrg/xdg"
    "github.com/gptscript-ai/go-gptscript"

    // 3. 本项目内部包（模块路径以 github.com/obot-platform/obot/ 开头）
    "github.com/obot-platform/obot/pkg/api/authn"
    "github.com/obot-platform/obot/pkg/services"

    // 4. 副作用 import（空白标识符，放最后）
    _ "github.com/obot-platform/nah/pkg/logrus" // Setup nah logging
)
```

### 流程 5：后台 goroutine 管理

后台任务必须绑定 context，实现优雅退出：

```go
// ✅ 正确：使用 ctx 控制生命周期
func (s *Server) PostStart(ctx context.Context) error {
    go s.watchLoop(ctx)
    return nil
}

func (s *Server) watchLoop(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return  // 优雅退出
        case <-ticker.C:
            s.sync(ctx)
        }
    }
}

// ❌ 错误：goroutine 无退出机制
go func() {
    for {
        time.Sleep(30 * time.Second)
        s.sync(context.Background())  // context 无法取消
    }
}()
```

### 流程 6：Options 结构体用于可选配置

对外暴露的服务 Options，使用标签支持环境变量：

```go
// pkg/gateway/server/server.go
type Options struct {
    Hostname     string
    UIHostname   string `name:"ui-hostname" env:"OBOT_SERVER_UI_HOSTNAME"`
    GatewayDebug bool

    DailyUserPromptTokenLimit     int  `usage:"Max daily prompt tokens" default:"10000000"`
    DailyUserCompletionTokenLimit int  `usage:"Max daily completion tokens" default:"100000"`
    NanobotIntegration            bool `usage:"Enable Nanobot integration" default:"false"`
    LocalAuthEnabled              bool `usage:"Enable local auth" env:"OBOT_LOCAL_AUTH_ENABLED" default:"false"`
}

func New(ctx context.Context, db *db.DB, ..., opts Options) (*Server, error) {
    s := &Server{
        baseURL:              opts.Hostname,
        nanobotIntegration:   opts.NanobotIntegration,
        localAuthEnabled:     opts.LocalAuthEnabled,
    }
    // 基于 opts 初始化可选功能
    if opts.LocalAuthEnabled {
        s.loginLimiter = newLoginRateLimiter(ctx, 5*time.Minute, 20)
        // NOTE: goroutine 理想应在 Start(ctx) 中启动（待重构）
        go s.disableInactiveLocalUsersLoop(ctx)
    }
    return s, nil
}
```

---

## 注意事项

### 关键约束

1. **构造函数（`New`）不做 I/O**
   - 数据库连接、文件读取等放在 `PreStart`
   - 保证 `New` 可以在无副作用的情况下测试

2. **后台 goroutine 必须接受 context**
   - 所有 goroutine 必须能通过 context 取消
   - 服务关闭时所有 goroutine 必须退出

3. **Services 字段按初始化顺序排列**
   - 被依赖的组件先初始化
   - 注释说明关键依赖关系

4. **副作用 import 放最后，加注释**
   - `_ "github.com/.../logrus"` 需要注释说明用途
   - 例如 `// Setup nah logging`

5. **Config 标签使用 `name` 而非 `flag`**
   - 项目使用自定义 CLI 框架，标签是 `name:` 而非标准 `flag:`

### 常见陷阱

| 问题 | 影响 | 解决方案 |
|------|------|----------|
| 在 `New()` 中启动 goroutine | 测试难以控制 | goroutine 移到 `PostStart` |
| 两个包相互 import | 编译失败 | 在其中一个包定义接口 |
| 配置不加 `env:` 标签 | 容器化部署无法通过环境变量配置 | 所有关键配置加 `env:` |
| goroutine 内用 `context.Background()` | 无法优雅退出 | 传入上层 `ctx` |

---

## 反模式（避免）

| 反模式 ❌ | 正确做法 ✅ |
|----------|------------|
| 全局变量存储 Services | 通过构造函数注入 `*services.Services` |
| `New()` 内部建立数据库连接 | 连接逻辑放 `PreStart()` |
| 直接 import 导致循环 | 定义本地接口，依赖注入 |
| goroutine 无 context | `go s.loop(ctx)` |
| import 分组混乱 | stdlib → 第三方 → 内部 → 副作用 |

---

## 相关 Skills

- [obot-api-handler](../obot-api-handler/SKILL.md)：Handler 如何从 Services 获取依赖
- [obot-controller-handler](../obot-controller-handler/SKILL.md)：Controller 如何使用 nah Router
- [obot-authz](../obot-authz/SKILL.md)：Authorizer 的注入与使用
- [review-go-style](../review-go-style/SKILL.md)：Go 代码风格审查
