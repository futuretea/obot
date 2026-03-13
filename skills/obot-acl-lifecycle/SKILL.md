---
name: obot-acl-lifecycle
description: |
  基于 obot 项目 pkg/authz/ 的 ACL、pkg/controller/handlers/ 的 MCP 生命周期、知识系统实现，涵盖 kclient.IgnoreNotFound 链式模式、Status 驱动的状态机、确定性资源命名、能力驱动的任务生命周期、Role 感知的 ACL 生成、存储提供者策略模式、集成测试框架等核心实践。
  ACL, MCP lifecycle, and knowledge system patterns covering kclient.IgnoreNotFound chaining, status-driven state machines, deterministic resource naming, capability-driven task lifecycle, role-aware ACL generation, storage provider strategy pattern, and integration test framework.
---

# Obot ACL 与资源生命周期模式

## 适用范围

本 Skill 适用于以下场景：

- **ACL 权限管理**：`pkg/controller/handlers/accesscontrolrule/` - AccessControlRule 维护与资源裁剪
- **Power User 工作区**：`pkg/controller/handlers/poweruserworkspace/` - 角色变更、ACL 生成、降级清理
- **线程升级/复制**：`pkg/controller/handlers/threads/template.go` - 配置修订、升级检测、任务复制
- **知识系统**：`pkg/controller/handlers/knowledgeset/` - KnowledgeSet 工作区与文件管理
- **审计日志导出**：`pkg/controller/handlers/auditlogexport/` - 流式导出与定时调度
- **类型定义**：`pkg/storage/apis/obot.obot.ai/v1/` - CRD 类型与 Finalizer
- **角色体系**：`apiclient/types/user.go` - 位标志角色层级

---

## 核心原则

### 1. 四分支协调模式（Reconcile Pattern）

PowerUserWorkspace 控制器使用四分支逻辑处理角色变更：

```go
// pkg/controller/handlers/poweruserworkspace/poweruserworkspace.go:83-118
func (h *Handler) reconcileWorkspace(ctx context.Context, c kclient.Client, userID string, effectiveRole types.Role) error {
    shouldHave := isPrivilegedRole(effectiveRole)
    existing := &v1.PowerUserWorkspace{}
    err := c.Get(ctx, router.Key(namespace, system.GetPowerUserWorkspaceID(userID)), existing)
    hasWorkspace := err == nil

    switch {
    case shouldHave && !hasWorkspace:
        // Case 1: 应有但不存在 → 创建
        return c.Create(ctx, &v1.PowerUserWorkspace{...})
    case !shouldHave && hasWorkspace:
        // Case 2: 不应有但存在 → 删除
        return c.Delete(ctx, existing)
    case shouldHave && hasWorkspace:
        // Case 3: 应有且存在 → 协调角色
        return h.reconcileWorkspaceRole(ctx, c, existing, effectiveRole)
    default:
        // Case 4: 不应有且不存在 → 无操作
        return nil
    }
}
```

### 2. Status 驱动的幂等性 Guard

使用 Status 字段防止重复执行：

```go
// pkg/controller/handlers/poweruserworkspace/poweruserworkspace.go:239-301
func (h *Handler) createDefaultAccessControlRule(ctx context.Context, c kclient.Client, workspace *v1.PowerUserWorkspace) error {
    if workspace.Status.DefaultAccessControlRuleGenerated {
        return nil  // 幂等：已生成则跳过
    }

    // 仅 PowerUserPlus 及以上角色生成 ACL
    if !workspace.Spec.Role.HasRole(types.RolePowerUserPlus) {
        return nil
    }

    // 生成通配符选择器规则
    acr := &v1.AccessControlRule{
        ObjectMeta: metav1.ObjectMeta{
            GenerateName: system.AccessControlRulePrefix,
            Namespace:    workspace.Namespace,
        },
        Spec: v1.AccessControlRuleSpec{
            PowerUserWorkspaceID: workspace.Name,
            Generated:            true,
            Manifest: types.AccessControlRuleManifest{
                Subjects: []types.Subject{{
                    Type: types.SubjectTypeSelector,
                    ID:   "*",  // 通配符：所有用户
                }},
                Resources: []types.Resource{{
                    Type: types.ResourceTypeSelector,
                    ID:   "*",  // 通配符：所有资源
                }},
            },
        },
    }
    if err := c.Create(ctx, acr); err != nil {
        return err
    }

    workspace.Status.DefaultAccessControlRuleGenerated = true
    return c.Status().Update(ctx, workspace)
}
```

### 3. 确定性资源命名

使用 `name.SafeHashConcatName` 生成确定性名称，确保幂等性：

```go
import "github.com/obot-platform/nah/pkg/name"

// 确定性命名：相同输入产生相同名称
workflowName := name.SafeHashConcatName(system.WorkflowPrefix, "slack", thread.Name)
mcpServerName := name.SafeHashConcatName("mcp", catalogEntryID, userID)
copiedTaskName := name.SafeHashConcatName(sourceTask.Name, thread.Name)
copiedMCPName := name.SafeHashConcatName(sourceMCPID, thread.Name)

// 系统前缀常量（pkg/system/ids.go）
PowerUserWorkspacePrefix       = "puw1"
AccessControlRulePrefix        = "acr1"
KnowledgeSetPrefix             = "kst1"
WorkspacePrefix                = "wksp1"
AuditLogExportPrefix           = "ael1"
ScheduledAuditLogExportPrefix  = "sael1"
```

### 4. 角色层级与位标志

角色使用位标志实现层级包含关系：

```go
// apiclient/types/user.go:19-51
const (
    RoleBasic         Role = 4    // 基础认证用户
    RoleOwner         Role = 8    // 系统所有者
    RoleAdmin         Role = 16   // 管理员
    RoleAuditor       Role = 32   // 审计员（正交角色）
    RolePowerUserPlus Role = 64   // 增强 Power User（可管理 ACL）
    RolePowerUser     Role = 128  // 标准 Power User（有工作区）
)

// 角色层级映射：高角色包含低角色
var roleMap = map[Role]Role{
    RoleOwner:         RoleAdmin | RolePowerUserPlus | RolePowerUser | RoleBasic,
    RoleAdmin:         RolePowerUserPlus | RolePowerUser | RoleBasic,
    RolePowerUserPlus: RolePowerUser | RoleBasic,
    RolePowerUser:     RoleBasic,
}

func (r Role) HasRole(target Role) bool {
    return r&target != 0 || roleMap[r]&target != 0
}
```

### 5. 降级清理（Cascading Cleanup）

角色降级时级联删除关联资源：

```go
// pkg/controller/handlers/poweruserworkspace/poweruserworkspace.go:170-217
func (h *Handler) cleanupWorkspaceResources(ctx context.Context, c kclient.Client, workspace *v1.PowerUserWorkspace) error {
    // 删除 AccessControlRules
    var acrs v1.AccessControlRuleList
    if err := c.List(ctx, &acrs, kclient.InNamespace(workspace.Namespace),
        kclient.MatchingFields{"spec.powerUserWorkspaceID": workspace.Name}); err != nil {
        return err
    }
    for _, acr := range acrs.Items {
        if err := c.Delete(ctx, &acr); err != nil && !apierrors.IsNotFound(err) {
            return err
        }
    }

    // 删除 MCPServers
    var mcpServers v1.MCPServerList
    if err := c.List(ctx, &mcpServers, kclient.InNamespace(workspace.Namespace),
        kclient.MatchingFields{"spec.powerUserWorkspaceID": workspace.Name}); err != nil {
        return err
    }
    for _, mcp := range mcpServers.Items {
        if err := c.Delete(ctx, &mcp); err != nil && !apierrors.IsNotFound(err) {
            return err
        }
    }

    // 重置状态标志
    workspace.Status.DefaultAccessControlRuleGenerated = false
    return c.Status().Update(ctx, workspace)
}
```

### 6. 配置修订与升级检测

通过 hash 计算配置修订，检测线程与源线程的差异：

```go
// pkg/controller/handlers/threads/template.go:191-223
func (t *Handler) EnsureLatestConfigRevision(req router.Request, _ router.Response) error {
    thread := req.Object.(*v1.Thread)
    if !thread.Status.Created || !thread.Spec.Project || thread.Spec.ParentThreadName != "" {
        return nil
    }

    // 获取所有关联资源
    tasks, knowledgeFiles, projectMCPServers, mcpServers, mcpServerInstances, err :=
        t.fetchThreadResources(req.Ctx, req.Client, thread)

    // 计算配置修订
    config := newProjectThreadConfig(thread.Spec.Manifest, tasks, knowledgeFiles, ...)
    if changed := thread.SetLatestConfigRevision(config.Revision()); !changed {
        return nil
    }

    return req.Client.Status().Update(req.Ctx, thread)
}

// 升级检测：比较源线程与当前线程的修订
func (t *Handler) EnsureUpgradeAvailable(req router.Request, _ router.Response) error {
    thread := req.Object.(*v1.Thread)
    // 模板源：检查修订是否在源的历史中
    found, latest := source.HasRevision(thread.GetLatestConfigRevision())
    upgradeAvailable = found && !latest
}
```

### 7. 任务复制与语义相等检测

复制任务时生成新别名，通过语义比较检测变更：

```go
// pkg/controller/handlers/threads/threads.go:414-501
func (t *Handler) CopyTasksFromSource(req router.Request, _ router.Response) error {
    thread := req.Object.(*v1.Thread)

    for _, sourceTask := range sourceTasks.Items {
        copiedName := name.SafeHashConcatName(sourceTask.Name, thread.Name)

        // 检查是否已存在
        existing := &v1.Workflow{}
        err := req.Client.Get(ctx, router.Key(thread.Namespace, copiedName), existing)

        if apierrors.IsNotFound(err) {
            // 创建新任务，生成新别名
            newManifest := sourceTask.Spec.Manifest
            newManifest.Alias, _ = randomtoken.Generate()
            wf := &v1.Workflow{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      copiedName,
                    Namespace: thread.Namespace,
                },
                Spec: v1.WorkflowSpec{
                    ThreadName: thread.Name,
                    Manifest:   newManifest,
                },
            }
            if err := req.Client.Create(ctx, wf); err != nil {
                return err
            }
        } else if err == nil {
            // 已存在：清除 Alias 后比较语义相等性
            sourceManifest := sourceTask.Spec.Manifest
            sourceManifest.Alias = ""
            existingManifest := existing.Spec.Manifest
            existingManifest.Alias = ""
            if !reflect.DeepEqual(sourceManifest, existingManifest) {
                existing.Spec.Manifest = sourceTask.Spec.Manifest
                existing.Spec.Manifest.Alias = existing.Spec.Manifest.Alias // 保留原别名
                return req.Client.Update(ctx, existing)
            }
        }
    }

    thread.Status.CopiedTasks = true
    return req.Client.Status().Update(ctx, thread)
}
```

### 8. ACR 资源裁剪

定期清理引用已删除资源的 ACR 条目：

```go
// pkg/controller/handlers/accesscontrolrule/accesscontrolrule.go:24-90
func (h *Handler) PruneDeletedResources(req router.Request, _ router.Response) error {
    acr := req.Object.(*v1.AccessControlRule)
    var prunedResources []types.Resource

    for _, resource := range acr.Spec.Manifest.Resources {
        // 验证资源仍然存在
        exists, err := h.resourceExists(req.Ctx, req.Client, acr, resource)
        if err != nil {
            return err
        }
        if exists {
            prunedResources = append(prunedResources, resource)
        }
    }

    if len(prunedResources) != len(acr.Spec.Manifest.Resources) {
        acr.Spec.Manifest.Resources = prunedResources
        return req.Client.Update(req.Ctx, acr)
    }
    return nil
}
```

### 9. 审计日志流式导出

使用 `io.Pipe` 实现流式导出到存储提供者：

```go
// pkg/controller/handlers/auditlogexport/auditlogexport.go:114-150
func (h *Handler) streamingExport(ctx context.Context, export *v1.AuditLogExport, provider StorageProvider) error {
    pr, pw := io.Pipe()

    go func() {
        defer pw.Close()
        encoder := json.NewEncoder(pw)
        var offset int
        for {
            logs, err := h.gatewayClient.ListAuditLogs(ctx, export.Spec.Filters, offset, batchSize)
            if err != nil || len(logs) == 0 {
                return
            }
            for _, log := range logs {
                encoder.Encode(log)
            }
            offset += len(logs)
        }
    }()

    return provider.Upload(ctx, export.Status.ExportPath, pr)
}

// 存储提供者接口
type StorageProvider interface {
    Test() error
    Upload(ctx context.Context, path string, data io.Reader) error
}

// 工厂函数
func NewStorageProvider(providerType string, config StorageConfig) (StorageProvider, error) {
    switch providerType {
    case "s3":      return newS3Provider(config)
    case "gcs":     return newGCSProvider(config)
    case "azure":   return newAzureBlobProvider(config)
    case "customs3": return newCustomS3Provider(config)
    default:        return nil, fmt.Errorf("unsupported provider: %s", providerType)
    }
}
```

### 10. 定时审计日志导出

使用 cron 表达式调度导出任务：

```go
// pkg/controller/handlers/scheduledauditlogexport/scheduledauditlogexport.go:21-80
func (h *Handler) ScheduleExports(req router.Request, resp router.Response) error {
    schedule := req.Object.(*v1.ScheduledAuditLogExport)

    // 计算下次运行时间
    nextRun, err := h.calculateNextRun(schedule)
    if err != nil {
        return err
    }

    // 使用 RetryAfter 调度
    resp.RetryAfter(time.Until(nextRun))

    // 创建导出任务
    export := &v1.AuditLogExport{
        ObjectMeta: metav1.ObjectMeta{
            GenerateName: fmt.Sprintf("%s-", schedule.Name),
            Namespace:    schedule.Namespace,
        },
        Spec: v1.AuditLogExportSpec{
            StartTime: metav1.NewTime(startTime),
            EndTime:   metav1.NewTime(time.Now()),
            Filters:   schedule.Spec.Filters,
            Bucket:    schedule.Spec.Bucket,
        },
    }

    schedule.Status.TotalExportsCreated++
    return req.Client.Create(req.Ctx, export)
}
```

---

## 操作流程

### 流程 1：新增需要权限控制的资源

1. 在 `pkg/storage/apis/obot.obot.ai/v1/` 定义 CRD 类型（含 Finalizer）
2. 在 `apiclient/types/` 定义 Manifest 和 Subject/Resource 类型
3. 在 `pkg/controller/handlers/accesscontrolrule/` 添加资源裁剪逻辑
4. 在 Handler 中使用 Status 字段确保幂等性
5. 编写集成测试验证权限边界

### 流程 2：新增基于能力的资源

1. 在 ThreadSpec 的 Capabilities 中添加布尔字段
2. 在控制器中检查 Capabilities 决定创建/删除
3. 使用 `SafeHashConcatName` 确定性命名
4. 更新 Status 字段标记完成

### 流程 3：角色变更处理

1. `HandleRoleChange` 接收角色变更事件
2. `reconcileWorkspace` 四分支协调
3. 升级时 `createDefaultAccessControlRule`
4. 降级时 `cleanupWorkspaceResources` 级联删除

### 流程 4：线程升级

1. `EnsureLatestConfigRevision` 计算配置 hash
2. `EnsureUpgradeAvailable` 检测源线程变更
3. 用户批准后 `UpgradeThread` 执行升级
4. 清除派生状态触发下游控制器
5. `CopyTasksFromSource` / `CopyToolsFromSource` 重新复制

---

## 注意事项

### 关键约束

1. **幂等性**：所有 create/update 操作必须检查 Status 字段
2. **确定性命名**：使用 `SafeHashConcatName` 而非 `GenerateName`（需要幂等时）
3. **降级清理**：角色降级必须级联删除 ACR 和 MCPServer
4. **语义比较**：任务复制时清除 Alias 再比较（Alias 是随机生成的）
5. **Field Selector**：使用 `kclient.MatchingFields` 而非 List + Filter
6. **Finalizer**：CRD 类型必须定义 Finalizer 确保清理

### 常见陷阱

| 问题 | 影响 | 解决方案 |
|------|------|----------|
| 缺少 Status guard | 重复创建资源 | 检查 Status 字段再执行 |
| 使用 `GenerateName` 需要幂等 | 每次协调创建新资源 | 使用 `SafeHashConcatName` |
| 降级未清理 ACR | 权限残留 | `cleanupWorkspaceResources` 级联删除 |
| 任务比较含 Alias | 永远检测为不同 | 清除 Alias 后 `reflect.DeepEqual` |
| List 全量过滤 | 性能差 | 使用 Field Selector 索引查询 |
| 忽略 `IsNotFound` | 删除操作报错 | `kclient.IgnoreNotFound(err)` |

---

## 反模式

| 反模式 | 正确做法 |
|--------|----------|
| 直接创建资源不检查是否已存在 | Status guard + `SafeHashConcatName` |
| 缺少 Status 驱动的幂等性检查 | `if status.XXXGenerated { return nil }` |
| 使用随机命名需要幂等的资源 | `name.SafeHashConcatName(prefix, ...parts)` |
| 降级不清理关联资源 | `cleanupWorkspaceResources` 级联删除 |
| 不区分 PowerUser 和 PowerUserPlus | `role.HasRole(types.RolePowerUserPlus)` |
| List 全量 + 内存过滤 | `kclient.MatchingFields{...}` 索引查询 |
| 同步断言异步系统 | `Eventually` + 超时等待 |

---

## 相关 Skills

- [obot-invoke-engine](../obot-invoke-engine/SKILL.md)：调用引擎中的权限传递
- [obot-mcp-runtime](../obot-mcp-runtime/SKILL.md)：MCP 服务器生命周期
- [obot-gateway-security](../obot-gateway-security/SKILL.md)：API Key 权限范围
- [obot-controller-handler](../obot-controller-handler/SKILL.md)：控制器 Handler 模式
- [obot-service-wiring](../obot-service-wiring/SKILL.md)：依赖注入与初始化
