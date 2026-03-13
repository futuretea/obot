---
name: obot-authz
description: |
  基于 obot 项目 pkg/api/authz/ 的授权机制设计指南。涵盖 RBAC 角色组、基于路径的规则注册、资源级 checkX() 方法、Resources/Authorizated 结构体模式、ACR（AccessControlRule）集成等核心实践。
  Authorization design patterns for the obot project. Covers RBAC role groups, path-based rule registration, per-resource checkX() methods, Resources/Authorizated struct pattern, and ACR integration drawn from pkg/api/authz/.
---

# Obot 授权机制设计模式

## 适用范围

本 Skill 适用于在 `pkg/api/authz/` 下扩展或修改授权逻辑的场景：

- **新增资源授权**：为新的 API 端点添加权限控制
- **路径规则管理**：在 admin/owner 规则列表中注册路径
- **资源级别鉴权**：实现 `checkX()` 方法校验资源归属
- **ACR 集成**：使用 AccessControlRule 实现细粒度共享访问

---

## 核心原则

### 1. 授权分两层

**第一层：基于路径的静态规则**（`authz.go` 中的 slice）
- 匹配 HTTP 方法 + 路径模式
- 只决定"哪些角色可访问该路径"
- 不检查资源归属

**第二层：基于资源的动态检查**（`checkX()` 方法）
- 从路径中解析资源 ID
- 查询数据库验证资源归属或 ACR 权限
- 将授权后的资源对象存入 `Resources.Authorizated`

### 2. 角色组（Groups）

用户角色通过 `user.Info.GetGroups()` 获取，常量定义在 `apiclient/types/`:

| 常量 | 说明 |
|------|------|
| `types.GroupAdmin` | 系统管理员，拥有最高权限 |
| `types.GroupOwner` | 资源创建者，拥有自己资源的完整权限 |
| `types.GroupPowerUser` | 高级用户，可访问 workspace |
| `types.GroupAuthenticated` | 已认证用户（所有登录用户） |
| `types.GroupAuditor` | 审计员，只读访问审计日志 |

```go
// 检查用户是否属于某个 group
if slices.Contains(u.GetGroups(), types.GroupAdmin) {
    // admin 逻辑
}
```

---

## 操作流程

### 流程 1：注册基于路径的访问规则

所有管理员/Owner 可访问的路径在 `authz.go` 的 `adminAndOwnerRules` 中注册：

```go
var adminAndOwnerRules = []string{
    // 特定 HTTP 方法匹配（推荐，最精确）
    "POST /api/my-resources",
    "GET /api/my-resources/{id}",
    "PUT /api/my-resources/{id}",
    "DELETE /api/my-resources/{id}",
    "POST /api/my-resources/{id}/action",

    // 路径前缀匹配（允许该路径及所有子路径，谨慎使用）
    // "/api/my-resources/"  ← 仅在需要一次性覆盖所有子路径时使用
}
```

**匹配规则**：
- 无 Method 前缀：匹配所有 HTTP 方法
- 有 Method 前缀（如 `"GET /api/..."`）：只匹配指定方法
- 斜杠结尾：前缀匹配（`"/api/foo/"` 匹配 `/api/foo/bar`）

### 流程 2：实现资源级 checkX() 方法

为每类资源新增一个 `checkX()` 方法（独立文件，如 `mcpserver.go`）：

```go
// pkg/api/authz/myresource.go
package authz

func (a *Authorizer) checkMyResource(req *http.Request, resources *Resources, u user.Info) (bool, error) {
    // 1. 从 resources 中取解析好的 ID（由 parseResources 填充）
    if resources.MyResourceID == "" {
        return true, nil  // 没有该资源 ID 则跳过检查
    }

    // 2. 从存储中获取资源
    var myResource v1.MyResource
    if err := a.get(req.Context(), router.Key(system.DefaultNamespace, resources.MyResourceID), &myResource); err != nil {
        return false, err
    }

    // 3. 检查所有权（owner 检查）
    if myResource.Spec.UserID == u.GetUID() {
        resources.Authorizated.MyResource = &myResource
        return true, nil
    }

    // 4. 检查 ACR（共享访问）
    hasAccess, err := a.acrHelper.UserHasAccessToMyResource(u, myResource.Name)
    if err != nil || !hasAccess {
        return false, err
    }

    resources.Authorizated.MyResource = &myResource
    return true, nil
}
```

### 流程 3：Resources 结构体——解析与传递资源 ID

`Resources` 结构体在 `resources.go` 中定义，存储从请求路径解析出的 ID 和授权后的对象：

```go
type Resources struct {
    // 从路径解析的 ID 字段
    AgentID      string
    MCPServerID  string
    ThreadID     string
    // ... 其他资源 ID

    // 授权后的对象（checkX() 填充）
    Authorizated struct {
        Agent     *v1.Agent
        MCPServer *v1.MCPServer
        Thread    *v1.Thread
        // ...
    }
}
```

**在 handler 中读取授权后的对象**（避免重复查询）：

```go
func (a *Authorizer) Authorize(req *http.Request, u user.Info) (*Resources, bool, error) {
    resources := parseResources(req)

    // 依次执行各资源的 check，顺序按依赖关系排列
    if ok, err := a.checkAgent(req, resources, u); !ok || err != nil {
        return resources, false, err
    }
    if ok, err := a.checkMCPServer(req, resources, u); !ok || err != nil {
        return resources, false, err
    }
    // ...
    return resources, true, nil
}
```

### 流程 4：所有权检查模式

```go
// ✅ 检查资源归属于当前用户
if myResource.Spec.UserID == u.GetUID() && myResource.Spec.SharedCatalogID == "" {
    resources.Authorizated.MyResource = &myResource
    return true, nil
}

// ✅ Catalog 内共享资源——走 ACR 检查
if myResource.Spec.SharedCatalogID == system.DefaultCatalog {
    hasAccess, err := a.acrHelper.UserHasAccessToMCPServerInCatalog(u, myResource.Name, system.DefaultCatalog)
    if err != nil || !hasAccess {
        return false, err
    }
    resources.Authorizated.MyResource = &myResource
    return true, nil
}

// ✅ PowerUserWorkspace 内资源
if myResource.Spec.PowerUserWorkspaceID != "" {
    hasAccess, err := a.acrHelper.UserHasAccessToMCPServerInWorkspace(
        u, myResource.Name, myResource.Spec.PowerUserWorkspaceID, myResource.Spec.UserID,
    )
    if err != nil || !hasAccess {
        return false, err
    }
    resources.Authorizated.MyResource = &myResource
    return true, nil
}

return false, nil
```

## 流程 5：Handler 中读取 Authorizer 结果

授权成功后，handler **直接调用 `req.Get()`** 获取资源，不重新查询数据库（Kubernetes storage 层有缓存）。`Authorizated` 字段在 authz 内部流转，不暴露给 handler——handler 只关心"能否访问"，不关心"authz 如何决策的"。

```go
func (h *MyHandler) Get(req api.Context) error {
    id := req.PathValue("id")

    // authz 层已验证该用户有权访问此资源
    // 直接 Get，无需重复所有权检查
    var resource v1.MyResource
    if err := req.Get(&resource, id); err != nil {
        return err
    }

    return req.Write(convertResource(resource))
}
```

> 若需要在 handler 中做**额外的细粒度判断**（非所有权，如字段级权限），可结合 `req.UserIsAdmin()` 等方法，但所有权判断应只在 authz 层。

---

## 注意事项

### 关键约束

1. **checkX() 返回 `(bool, error)` 而非 `error`**
   - `(false, nil)` 表示无权限（403）
   - `(false, err)` 表示系统错误（500）
   - `(true, nil)` 表示授权通过

2. **check 方法有依赖顺序**
   - 子资源检查依赖父资源（如 `checkThread` 依赖 `checkAgent`）
   - 在 `Authorize()` 中按依赖顺序调用

3. **`Authorizated` 字段填充后续步骤可复用**
   - 避免对同一资源多次数据库查询
   - `checkX()` 成功后将对象存入 `Authorizated`

4. **路径规则注册不是细粒度控制**
   - 路径规则只控制"角色是否可访问该路径"
   - 资源级别（是否是 owner）由 `checkX()` 决定

5. **admin 组与 `checkX()` 的关系**
   - admin 通过路径规则后，资源级 `checkX()` 中通常无需额外判断 admin 角色
   - 但某些场景仍需区分（如审计日志、特定限额策略），此时可在 `checkX()` 内 `if slices.Contains(u.GetGroups(), types.GroupAdmin)` 单独处理
   - **不要**仅靠"admin 会绕过"的假设省略安全检查

### 常见陷阱

| 问题 | 影响 | 解决方案 |
|------|------|----------|
| 注册路径但未实现 `checkX()` | 任何人可访问任何该路径的资源 | 配套实现 `checkX()` |
| `checkX()` 返回 `(true, nil)` 但未填充 `Authorizated` | handler 无法复用授权对象 | 授权成功时必须填充 |
| check 方法顺序错误 | 父资源未授权就检查子资源 | 按依赖顺序排列 |
| 直接在 handler 中做 ownership 检查 | 逻辑分散、重复 | 统一在 authz 层 |

---

## 反模式（避免）

| 反模式 ❌ | 正确做法 ✅ |
|----------|------------|
| `checkX()` 返回 `error` 表示无权限 | 返回 `(false, nil)` 表示无权限，`(false, err)` 表示系统错误 |
| 在 handler 中 `if !req.UserIsAdmin() { return 403 }` | 在 authz 路径规则和 `checkX()` 中统一控制 |
| 每次 handler 调用都重新查询资源 | 通过 `Authorizated` 字段复用授权时的查询结果 |
| 将 `authz` 包 import 到 handler 包 | handler 通过 `api.Context` 获取已授权信息，不直接 import authz |

---

## 相关 Skills

- [obot-api-handler](../obot-api-handler/SKILL.md)：API Handler 设计模式
- [obot-service-wiring](../obot-service-wiring/SKILL.md)：服务依赖注入（Authorizer 的构造与注入）
- [review-go-style](../review-go-style/SKILL.md)：Go 代码风格审查
