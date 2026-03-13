---
name: obot-fullstack-mcp
description: |
  基于 obot 项目 pkg/mcp/composite.go 和 pkg/modelaccesspolicy/ 的全栈 MCP 代码模式，涵盖 Composite MCP 服务器编排、Model Access Policy 策略评估、Tool Approval 确认流程、HA 共享存储、Informer 索引优化、分层功能开关等全栈 MCP 核心实践。
  Full-stack MCP patterns from the obot project. Covers composite MCP server orchestration, model access policy evaluation, tool approval confirmation flow, HA shared storage, informer cache indexing, layered feature toggles, and LLM proxy permission enforcement.
---

# Obot 全栈 MCP 设计模式

## 适用范围

本 Skill 适用于以下场景：

- **Composite MCP 服务器**：实现扇出编排、工具命名空间隔离、组件去重校验
- **Model Access Policy**：基于 Subject（用户/组/选择器）的模型访问策略评估
- **Tool Approval**：将工具调用确认从内存迁移到 K8s 共享存储，支持多副本 HA
- **Informer 索引**：为策略评估添加自定义索引，实现 O(1) 查找
- **分层功能开关**：全局 flag -> 用户设置 -> 资源级覆盖的优先级链
- **LLM 代理层权限**：在代理层（而非仅 API Handler）执行模型访问检查

---

## 核心原则

### 1. 类型化校验错误——领域专用错误类型

校验错误使用 `types.RuntimeValidationError` 结构体，而非裸字符串。每个错误携带 Runtime、Field、Message 三要素，调用方可程序化区分错误类型：

```go
return types.RuntimeValidationError{
    Runtime: types.RuntimeComposite,
    Field:   "compositeConfig.componentServers",
    Message: fmt.Sprintf("duplicate component server: %s", component.CatalogEntryID),
}
```

**为什么不用 `errors.New`**：裸字符串错误只能做字符串匹配，无法按字段、Runtime 类型做程序化分支。结构化错误让上层可以精确定位哪个 Runtime 的哪个字段出了什么问题。

### 2. 边界校验——类型自带 Validate() 方法

Manifest 结构体自身携带校验逻辑，在 API 边界（Handler 层）调用，而非在存储层或业务逻辑深处：

```go
func (m ModelAccessPolicyManifest) Validate() error {
    if len(m.Subjects) == 0 {
        return fmt.Errorf("at least one subject is required")
    }

    subjects := make(map[Subject]struct{}, len(m.Subjects))
    for _, subject := range m.Subjects {
        if _, ok := subjects[subject]; ok {
            return fmt.Errorf("duplicate subject: %s/%s", subject.Type, subject.ID)
        }
        subjects[subject] = struct{}{}
    }

    return nil
}
```

**关键点**：
- 校验是值接收器方法（Manifest 是值类型，无状态）
- 去重检测使用 `map[Subject]struct{}`，不用 `map[Subject]bool`
- 在 Handler 层调用 `manifest.Validate()`，校验失败立即返回 400，不让脏数据进入存储

### 3. map[T]struct{} 用于集合语义

在 obot 项目中，集合一律使用 `map[T]struct{}`，绝不使用 `map[T]bool`。`bool` map 仅在值本身有语义时使用（如 `map[string]bool` 表示"启用/禁用"）：

```go
// 集合语义：只关心"是否存在"
seen := make(map[string]struct)
for _, item := range items {
    if _, ok := seen[item.ID]; ok {
        return fmt.Errorf("duplicate: %s", item.ID)
    }
    seen[item.ID] = struct{}{}
}

// 布尔语义：值本身有含义（启用/禁用）
featureFlags := map[string]bool{
    "toolApproval": true,
    "legacyMode":   false,
}
```

### 4. HA 共享存储——JSON Patch 替代内存状态

工具确认（Tool Approval）从本地 in-memory map 迁移到 K8s Run 资源的 Status 字段，使用 JSON Patch 实现原子追加，支持多副本部署：

```go
func addRequestedCallDecision(ctx context.Context, c kclient.SubResourceWriter, run *v1.Run, callID string) error {
    patchPath := "/status/requestedCallDecisions"

    var patchValue any
    if len(run.Status.RequestedCallDecisions) > 0 {
        patchValue = callID
        patchPath += "/-"  // JSON Patch: 追加到数组末尾
    } else {
        patchValue = []string{callID}  // 初始化数组
    }

    patchBytes, err := json.Marshal([]map[string]any{{
        "op":    "add",
        "path":  patchPath,
        "value": patchValue,
    }})
    if err != nil {
        return err
    }

    return c.Patch(ctx, run, kclient.RawPatch(ktypes.JSONPatchType, patchBytes))
}
```

**为什么用 JSON Patch 而非 Update**：
- `Update` 需要先 Get 再 Update，存在竞态窗口
- JSON Patch 是原子操作，多副本并发追加不会互相覆盖
- `/-` 语法表示追加到数组末尾，无需知道当前数组长度

### 5. Informer 自定义索引——O(1) 策略查找

为 Model Access Policy 的 Informer 缓存添加自定义索引，按 Subject 类型（User/Group/Selector）建立索引，策略评估时 O(1) 查找而非全量遍历：

```go
if err := mapInformer.AddIndexers(gocache.Indexers{
    mapUserIndex:     mapSubjectIndexFunc(types.SubjectTypeUser),
    mapGroupIndex:    mapSubjectIndexFunc(types.SubjectTypeGroup),
    mapSelectorIndex: mapSubjectIndexFunc(types.SubjectTypeSelector),
}); err != nil {
    return nil, err
}
```

索引函数按 Subject 类型提取键：

```go
func mapSubjectIndexFunc(subjectType types.SubjectType) gocache.IndexFunc {
    return func(obj any) ([]string, error) {
        policy, ok := obj.(*v1.ModelAccessPolicy)
        if !ok {
            return nil, nil
        }

        var keys []string
        for _, subject := range policy.Spec.Manifest.Subjects {
            if subject.Type == subjectType {
                keys = append(keys, subject.ID)
            }
        }
        return keys, nil
    }
}
```

**查询时**：
```go
// O(1) 查找：获取所有匹配当前用户的策略
policies, err := indexer.ByIndex(mapUserIndex, userID)
```

### 6. 分层功能开关——三级优先级链

功能开关遵循 `全局 flag -> 用户设置 -> 资源级覆盖` 的优先级链：

```
全局 flag（Config / 环境变量）
    ↓ 可被覆盖
用户级设置（UserSetting CR）
    ↓ 可被覆盖
资源级覆盖（MCPServer.Spec / Project.Spec 字段）
```

```go
func resolveToolApprovalEnabled(globalConfig Config, userSetting *v1.UserSetting, server *v1.MCPServer) bool {
    // 资源级覆盖优先
    if server.Spec.ToolApprovalOverride != nil {
        return *server.Spec.ToolApprovalOverride
    }

    // 用户级设置次之
    if userSetting != nil && userSetting.Spec.ToolApprovalEnabled != nil {
        return *userSetting.Spec.ToolApprovalEnabled
    }

    // 全局 flag 兜底
    return globalConfig.ToolApprovalEnabled
}
```

**关键点**：使用 `*bool`（指针）区分"未设置"和"显式设为 false"。

---

## 操作流程

### 流程 1：实现 Composite MCP 服务器编排

Composite MCP 服务器扇出到多个组件服务器，合并结果，工具名带命名空间前缀：

```go
// 1. 校验组件列表——去重检测
func validateCompositeConfig(config types.CompositeConfig) error {
    seen := make(map[string]struct{}, len(config.ComponentServers))
    var errs []error

    for _, component := range config.ComponentServers {
        if _, ok := seen[component.CatalogEntryID]; ok {
            errs = append(errs, types.RuntimeValidationError{
                Runtime: types.RuntimeComposite,
                Field:   "compositeConfig.componentServers",
                Message: fmt.Sprintf("duplicate component server: %s", component.CatalogEntryID),
            })
        }
        seen[component.CatalogEntryID] = struct{}{}
    }

    return errors.Join(errs...)
}

// 2. 扇出调用——并发获取工具列表
func (c *CompositeClient) ListTools(ctx context.Context) ([]types.Tool, error) {
    var (
        mu       sync.Mutex
        allTools []types.Tool
        g        errgroup.Group
    )

    for _, component := range c.components {
        component := component  // capture loop variable
        g.Go(func() error {
            tools, err := component.Client.ListTools(ctx)
            if err != nil {
                return fmt.Errorf("component %s: %w", component.Name, err)
            }

            mu.Lock()
            defer mu.Unlock()
            for _, tool := range tools {
                // 工具名加命名空间前缀，避免跨组件冲突
                tool.Name = component.Name + "/" + tool.Name
                allTools = append(allTools, tool)
            }
            return nil
        })
    }

    if err := g.Wait(); err != nil {
        return nil, err
    }

    return allTools, nil
}
```

### 流程 2：Model Access Policy 评估

策略评估通过 Informer 索引实现高效查找，按 User -> Group -> Selector 顺序匹配：

```go
func (h *Helper) UserHasModelAccess(ctx context.Context, user user.Info, modelID string) (bool, error) {
    // 1. 按用户 ID 查找策略（O(1)）
    policies, err := h.indexer.ByIndex(mapUserIndex, user.GetUID())
    if err != nil {
        return false, err
    }

    // 2. 按用户组查找策略
    for _, group := range user.GetGroups() {
        groupPolicies, err := h.indexer.ByIndex(mapGroupIndex, group)
        if err != nil {
            return false, err
        }
        policies = append(policies, groupPolicies...)
    }

    // 3. 评估所有匹配的策略
    for _, obj := range policies {
        policy := obj.(*v1.ModelAccessPolicy)
        if policy.Spec.Manifest.AllowsModel(modelID) {
            return true, nil
        }
    }

    return false, nil
}
```

### 流程 3：LLM 代理层权限执行

模型访问检查在代理层（proxy）执行，返回 403，而非仅在 API Handler 层检查。这确保即使绕过 API 直接访问代理也无法越权：

```go
// pkg/proxy/proxy.go
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    userInfo := authn.GetUser(r.Context())
    modelID := extractModelID(r)

    // 在代理层检查模型访问权限——不依赖上游 API Handler
    hasAccess, err := p.modelAccessHelper.UserHasModelAccess(r.Context(), userInfo, modelID)
    if err != nil {
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    if !hasAccess {
        http.Error(w, "model access denied", http.StatusForbidden)
        return
    }

    // 权限通过，转发请求
    p.upstream.ServeHTTP(w, r)
}
```

### 流程 4：forceTick 机制——避免轮询延迟

自定义 tickEvery 循环支持 forceTick channel，当工具确认到达时立即触发处理，而非等待下一个 tick 周期：

```go
type ConfirmationHandler struct {
    forceTick chan struct{}
    // ...
}

func (h *ConfirmationHandler) NotifyConfirmation() {
    // 非阻塞发送，避免发送方阻塞
    select {
    case h.forceTick <- struct{}{}:
    default:
    }
}

func (h *ConfirmationHandler) Run(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            h.processConfirmations(ctx)
        case <-h.forceTick:
            h.processConfirmations(ctx)
        }
    }
}
```

**为什么需要 forceTick**：
- 默认 ticker 间隔（如 5 秒）会导致用户确认后最多等待 5 秒才被处理
- forceTick 让确认到达时立即触发，用户体验从"最多 5 秒"变为"近乎即时"
- `select + default` 保证非阻塞，不会因 channel 满而卡住发送方

### 流程 5：Table-Driven 测试（testify 风格）

使用 `assert` 和 `require` 区分"可继续"和"必须中断"的断言：

```go
func TestValidateModelAccessPolicy(t *testing.T) {
    tests := []struct {
        name    string
        input   types.ModelAccessPolicyManifest
        wantErr string
    }{
        {
            name: "valid policy with single user subject",
            input: types.ModelAccessPolicyManifest{
                Subjects: []types.Subject{
                    {Type: types.SubjectTypeUser, ID: "user-1"},
                },
                Models: []string{"gpt-4"},
            },
            wantErr: "",
        },
        {
            name: "empty subjects returns error",
            input: types.ModelAccessPolicyManifest{
                Subjects: []types.Subject{},
                Models:   []string{"gpt-4"},
            },
            wantErr: "at least one subject is required",
        },
        {
            name: "duplicate subject returns error",
            input: types.ModelAccessPolicyManifest{
                Subjects: []types.Subject{
                    {Type: types.SubjectTypeUser, ID: "user-1"},
                    {Type: types.SubjectTypeUser, ID: "user-1"},
                },
                Models: []string{"gpt-4"},
            },
            wantErr: "duplicate subject: user/user-1",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.input.Validate()
            if tt.wantErr == "" {
                require.NoError(t, err)  // 必须无错误，否则中断
            } else {
                require.Error(t, err)    // 必须有错误，否则中断
                assert.Contains(t, err.Error(), tt.wantErr)  // 可继续
            }
        })
    }
}
```

**`require` vs `assert`**：
- `require`：断言失败立即中断当前测试（`t.FailNow()`），用于前置条件
- `assert`：断言失败记录错误但继续执行（`t.Fail()`），用于验证细节

---

## 注意事项

### 关键约束

1. **JSON Patch 的数组初始化 vs 追加**
   - 空数组时用 `"op": "add", "path": "/status/field", "value": ["item"]` 初始化
   - 非空数组时用 `"op": "add", "path": "/status/field/-", "value": "item"` 追加
   - 混淆两者会导致 patch 失败或覆盖已有数据

2. **Informer 索引必须在 Start 之前注册**
   - `AddIndexers()` 必须在 Informer 启动（`Start()`）之前调用
   - 启动后调用会返回错误
   - 索引函数必须是幂等的纯函数

3. **`*bool` 区分"未设置"和"false"**
   - 功能开关字段使用 `*bool` 而非 `bool`
   - `nil` 表示"未设置，使用上层默认值"
   - `ptr(false)` 表示"显式禁用"

4. **forceTick channel 必须带 buffer 或用 select+default**
   - 无缓冲 channel + 阻塞发送会导致发送方卡住
   - 推荐 `select { case ch <- struct{}{}: default: }` 非阻塞发送

5. **代理层权限检查不可省略**
   - 即使 API Handler 已做权限检查，代理层仍需独立检查
   - 代理层可能被内部服务直接调用，绕过 API Handler
   - 返回 403 而非静默降级

6. **Composite 工具名必须带命名空间前缀**
   - 格式：`componentName/toolName`
   - 避免不同组件的同名工具冲突
   - 调用时需要解析前缀路由到正确的组件客户端

### 常见陷阱

| 问题 | 影响 | 解决方案 |
|------|------|----------|
| JSON Patch 空数组用 `/-` 追加 | patch 失败（路径不存在） | 先判断数组是否为空，空则初始化 |
| Informer 启动后添加索引 | 返回错误，索引不生效 | 在 `Start()` 之前调用 `AddIndexers()` |
| 功能开关用 `bool` 而非 `*bool` | 无法区分"未设置"和"false" | 使用 `*bool`，`nil` 表示未设置 |
| 集合用 `map[T]bool` | 语义不清，`false` 值有歧义 | 使用 `map[T]struct{}` |
| 只在 API Handler 检查模型权限 | 代理层被绕过时无权限保护 | 代理层独立检查，返回 403 |
| forceTick 用阻塞发送 | 发送方 goroutine 卡住 | `select + default` 非阻塞发送 |

---

## 反模式（避免）

| 反模式 | 正确做法 |
|--------|----------|
| `return errors.New("duplicate server")` 裸字符串 | `return types.RuntimeValidationError{Runtime: ..., Field: ..., Message: ...}` |
| `map[string]bool` 做集合去重 | `map[string]struct{}` |
| `client.Update()` 做并发追加 | JSON Patch `"op": "add", "path": "/-"` 原子追加 |
| 全量遍历策略列表做权限检查 | Informer 自定义索引 + `ByIndex()` O(1) 查找 |
| 功能开关 `bool` 字段 | `*bool` 字段，`nil` 表示"继承上层默认" |
| 只在 API Handler 层检查模型权限 | 代理层独立执行 403 检查 |
| `time.Sleep` 轮询等待确认 | `ticker + forceTick channel` 即时响应 |
| `require` 用于所有断言 | 前置条件用 `require`，细节验证用 `assert` |
| Composite 工具名不加前缀 | `componentName/toolName` 命名空间隔离 |
| 校验逻辑散落在存储层 | Manifest 自带 `Validate()` 方法，Handler 层调用 |

---

## 相关 Skills

- [obot-validation](../obot-validation/SKILL.md)：RuntimeValidator 接口模式与结构化校验错误
- [obot-controller-handler](../obot-controller-handler/SKILL.md)：Controller 调谐循环与状态更新
- [obot-authz](../obot-authz/SKILL.md)：授权机制与 ACR 集成
- [obot-api-handler](../obot-api-handler/SKILL.md)：API Handler 依赖注入与错误处理
- [obot-service-wiring](../obot-service-wiring/SKILL.md)：服务初始化与后台 goroutine 管理
- [review-go-style](../review-go-style/SKILL.md)：Go 代码风格审查
