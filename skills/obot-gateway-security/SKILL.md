---
name: obot-gateway-security
description: |
  基于 obot 项目 pkg/gateway/ 的 MCP 网关安全设计指南。涵盖 API Key 全生命周期（crypto/rand 生成、bcrypt 哈希、格式解析）、MCP Server 多层访问控制、认证 Webhook、Server/Client 分层架构、GORM 数据库模式、last-used 节流更新、Admin/User 端点分离等核心实践。
  MCP gateway security design patterns for the obot project. Covers API key full lifecycle (crypto/rand generation, bcrypt hashing, format parsing), multi-layer MCP server access control, authentication webhook, Server/Client split architecture, GORM database patterns, last-used throttled updates, and admin/user endpoint separation drawn from pkg/gateway/.
---

# Obot MCP 网关安全设计模式

## 适用范围

本 Skill 适用于在 `pkg/gateway/` 下新增或修改安全相关逻辑的场景：

- **API Key 管理**：生成、验证、吊销 API Key（`client/apikey.go`、`server/apikey.go`）
- **MCP Server 访问控制**：Owner/Catalog ACR/Workspace ACR 多层检查
- **认证 Webhook**：Bearer Token 提取、格式校验、bcrypt 验证、MCP Server 作用域检查
- **Admin 端点**：为管理员提供跨用户的 API Key 管理能力
- **数据库操作**：GORM 事务、条件查询、`ErrRecordNotFound` 处理

---

## 核心原则

### 1. Server/Client 分层架构

网关安全逻辑严格分为两层，职责清晰：

- `server/apikey.go`（Server 层）：处理 HTTP 请求、提取参数、权限前置检查、调用 Client 层
- `client/apikey.go`（Client 层）：处理数据库操作、密钥生成、bcrypt 哈希、格式解析

```go
// Server 层：HTTP 处理 + 权限检查，委托给 Client 层做数据操作
func (s *Server) createAPIKey(apiContext api.Context) error {
    // 1. 读取请求、校验参数
    // 2. 验证用户对 MCP Server 的访问权限
    // 3. 委托给 Client 层
    response, err := apiContext.GatewayClient.CreateAPIKey(ctx, userID, name, desc, expiresAt, mcpServerIDs)
    return apiContext.WriteCreated(response)
}

// Client 层：纯数据操作，不处理 HTTP
func (c *Client) CreateAPIKey(ctx context.Context, userID uint, name, description string, ...) (*types.APIKeyCreateResponse, error) {
    // 生成密钥、哈希、写入数据库
}
```

### 2. API Key 格式：`ok1-<userID>-<keyID>-<secret>`

密钥格式包含四个部分，通过 `-` 分隔：

| 部分 | 说明 | 示例 |
|------|------|------|
| `ok1` | 版本前缀，固定 3 字符 | `ok1` |
| `userID` | 用户 ID（uint） | `123` |
| `keyID` | 密钥 ID（GORM 自增主键） | `456` |
| `secret` | Base64 RawURL 编码的随机密钥 | `dGVzdHNlY3JldA` |

密钥 ID 由数据库自增生成，因此 `fullKey` 在 `Create` 写入数据库之后才能拼装：

```go
const (
    apiKeySecretLength = 32 // 32 bytes = 256 bits of entropy
    apiKeyPrefix       = "ok1"
)

func (c *Client) CreateAPIKey(ctx context.Context, userID uint, name, description string, expiresAt *time.Time, mcpServerIDs []string) (*types.APIKeyCreateResponse, error) {
    // 1. 生成密钥：crypto/rand 保证密码学安全
    secretBytes := make([]byte, apiKeySecretLength)
    if _, err := rand.Read(secretBytes); err != nil {
        return nil, fmt.Errorf("failed to generate secret: %w", err)
    }
    secret := base64.RawURLEncoding.EncodeToString(secretBytes)

    // 2. bcrypt 哈希：只存储哈希值，明文不落库
    hashedSecret, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
    if err != nil {
        return nil, fmt.Errorf("failed to hash secret: %w", err)
    }

    // 3. 写入数据库（ID 由 GORM autoIncrement 生成）
    apiKey := &types.APIKey{
        UserID:       userID,
        Name:         name,
        Description:  description,
        HashedSecret: string(hashedSecret),
        ExpiresAt:    expiresAt,
        CreatedAt:    time.Now(),
        MCPServerIDs: mcpServerIDs,
    }
    if err := c.db.WithContext(ctx).Create(apiKey).Error; err != nil {
        return nil, fmt.Errorf("failed to create API key: %w", err)
    }

    // 4. 拼装完整密钥（仅此一次返回明文）
    fullKey := fmt.Sprintf("%s-%d-%d-%s", apiKeyPrefix, userID, apiKey.ID, secret)

    return &types.APIKeyCreateResponse{
        APIKey: *apiKey,
        Key:    fullKey,
    }, nil
}
```

### 3. MCP Server 多层访问控制

访问检查按优先级依次执行，命中即返回：

```
Owner 检查 → Catalog ACR 检查 → Workspace ACR 检查 → 拒绝
```

```go
func (s *Server) userHasAccessToMCPServer(apiContext api.Context, server *v1.MCPServer) (bool, error) {
    userID := apiContext.User.GetUID()

    // 第一层：Owner 始终有权限
    if server.Spec.UserID == userID {
        return true, nil
    }

    // 第二层：Catalog 作用域——走 ACR 检查
    if server.Spec.MCPCatalogID != "" {
        return s.acrHelper.UserHasAccessToMCPServerInCatalog(apiContext.User, server.Name, server.Spec.MCPCatalogID)
    }

    // 第三层：Workspace 作用域——走 ACR 检查
    if server.Spec.PowerUserWorkspaceID != "" {
        return s.acrHelper.UserHasAccessToMCPServerInWorkspace(apiContext.User, server.Name, server.Spec.PowerUserWorkspaceID, server.Spec.UserID)
    }

    // 不属于任何作用域，拒绝
    return false, nil
}
```

### 4. 通配符 `"*"` 延迟校验

`"*"` 表示授权访问用户可达的所有 MCP Server。关键设计：创建时不校验，认证时校验。

```go
// 创建时：遇到 "*" 跳过校验
for _, serverID := range req.MCPServerIDs {
    if serverID == "*" {
        continue // 延迟到认证时检查
    }
    // 非通配符：立即校验用户是否有权限
}

// 认证时：检查通配符或精确匹配
hasWildcard := slices.Contains(apiKey.MCPServerIDs, "*")
if !hasWildcard && !slices.Contains(apiKey.MCPServerIDs, req.MCPID) {
    // 还需检查 composite server 的子组件
    // ...
}
```

### 5. HashedSecret 字段用 `json:"-"` 隐藏

数据库模型中 `HashedSecret` 标记 `json:"-"`，确保序列化时永远不会泄露哈希值：

```go
type APIKey struct {
    ID           uint       `json:"id" gorm:"primaryKey;autoIncrement"`
    UserID       uint       `json:"userId" gorm:"index"`
    Name         string     `json:"name"`
    Description  string     `json:"description,omitempty"`
    HashedSecret string     `json:"-"`                     // 永远不序列化
    CreatedAt    time.Time  `json:"createdAt"`
    LastUsedAt   *time.Time `json:"lastUsedAt,omitempty"`
    ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
    MCPServerIDs []string   `json:"mcpServerIds,omitempty" gorm:"serializer:json"`
}
```

---

## 操作流程

### 流程 1：认证 Webhook 完整链路

`authenticateAPIKey` 是外部 MCP Server 调用的认证端点，完整链路如下：

```
提取 Bearer Token → 校验 ok1- 前缀 → 解析请求体获取 MCPID
→ ValidateAPIKey（解析格式 + 查库 + bcrypt 验证 + 过期检查）
→ 查询用户信息 → 检查 MCP Server 作用域（通配符/精确/composite）
→ 验证用户仍有权访问该 Server → 返回用户身份 → 异步更新 last_used_at
```

```go
func (s *Server) authenticateAPIKey(apiContext api.Context) error {
    // 1. 提取 Bearer Token
    authHeader := apiContext.Request.Header.Get("Authorization")
    bearer, ok := strings.CutPrefix(authHeader, "Bearer ")
    if !ok || !strings.HasPrefix(bearer, "ok1-") {
        return apiContext.Write(apiKeyAuthResponse{Allowed: false, Reason: "invalid API key format"})
    }

    // 2. 解析请求体获取目标 MCP Server ID
    var req apiKeyAuthRequest
    if err := apiContext.Read(&req); err != nil {
        return apiContext.Write(apiKeyAuthResponse{Allowed: false, Reason: "invalid request body"})
    }

    // 3. 验证 API Key（格式解析 + 数据库查找 + bcrypt + 过期检查）
    apiKey, err := apiContext.GatewayClient.ValidateAPIKey(apiContext.Context(), bearer)
    if err != nil {
        return apiContext.Write(apiKeyAuthResponse{Allowed: false, Reason: "invalid or expired API key"})
    }

    // 4. 检查 MCP Server 作用域（通配符 / 精确匹配 / composite 子组件）
    hasWildcard := slices.Contains(apiKey.MCPServerIDs, "*")
    if !hasWildcard && !slices.Contains(apiKey.MCPServerIDs, req.MCPID) {
        var mcpServer v1.MCPServer
        if err := apiContext.Storage.Get(apiContext.Context(), kclient.ObjectKey{
            Namespace: system.DefaultNamespace, Name: req.MCPID,
        }, &mcpServer); err != nil || mcpServer.Spec.CompositeName == "" ||
            !slices.Contains(apiKey.MCPServerIDs, mcpServer.Spec.CompositeName) {
            return apiContext.Write(apiKeyAuthResponse{Allowed: false, Reason: "API key does not have access to this MCP server"})
        }
    }

    // 5. 验证用户仍有权访问该 Server（权限可能已被撤销）
    hasAccess, err := s.userHasAccessToMCPServerByUserID(apiContext, &server, apiKey.UserID)

    // 6. 返回用户身份信息
    err = apiContext.Write(apiKeyAuthResponse{
        Allowed:          true,
        Subject:          fmt.Sprintf("%d", apiKey.UserID),
        Name:             user.DisplayName,
        PreferredUsername: user.Username,
        Email:            user.Email,
    })

    // 7. 异步更新 last_used_at（失败不影响认证结果）
    if keyErr := s.updateKeyLastUsedTime(apiContext, apiKey); keyErr != nil {
        logger.Errorf("failed to update API key last used time: %v", keyErr)
    }
    return err
}
```

### 流程 2：ValidateAPIKey 事务操作

验证密钥在单个 GORM 事务中完成：查找 + bcrypt 验证 + 过期检查 + 节流更新 `last_used_at`：

```go
func (c *Client) ValidateAPIKey(ctx context.Context, key string) (*types.APIKey, error) {
    _, userID, keyID, secret, err := ParseAPIKey(key)
    if err != nil {
        return nil, err
    }

    var apiKey types.APIKey
    err = c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        // 按 keyID + userID 查找（双条件防止越权）
        if err := tx.Where("id = ?", keyID).Where("user_id = ?", userID).First(&apiKey).Error; err != nil {
            return err
        }

        // bcrypt 验证密钥
        if err := bcrypt.CompareHashAndPassword([]byte(apiKey.HashedSecret), []byte(secret)); err != nil {
            return fmt.Errorf("invalid API key")
        }

        // 过期检查
        if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
            return fmt.Errorf("API key has expired")
        }

        // 节流更新 last_used_at：仅当超过 1 分钟才写入
        now := time.Now()
        if apiKey.LastUsedAt == nil || now.Sub(*apiKey.LastUsedAt) > time.Minute {
            apiKey.LastUsedAt = &now
            return tx.Model(&apiKey).Update("last_used_at", now).Error
        }
        return nil
    })
    if err != nil {
        return nil, err
    }
    return &apiKey, nil
}
```

### 流程 3：ParseAPIKey 格式解析

使用 `fmt.Sscanf` 解析固定格式，配合前缀校验：

```go
func ParseAPIKey(key string) (prefix string, userID uint, keyID uint, secret string, err error) {
    n, err := fmt.Sscanf(key, "%3s-%d-%d-%s", &prefix, &userID, &keyID, &secret)
    if err != nil || n != 4 {
        return "", 0, 0, "", fmt.Errorf("invalid API key format")
    }
    if prefix != apiKeyPrefix {
        return "", 0, 0, "", fmt.Errorf("invalid API key prefix")
    }
    return prefix, userID, keyID, secret, nil
}
```

### 流程 4：Admin 与 User 端点分离

路由注册中，用户端点和管理员端点使用不同路径前缀：

```go
// 用户端点：/api/api-keys（操作自己的 Key）
mux.HandleFunc("POST /api/api-keys", wrap(s.createAPIKey))
mux.HandleFunc("GET /api/api-keys", wrap(s.listAPIKeys))
mux.HandleFunc("GET /api/api-keys/{id}", wrap(s.getAPIKey))
mux.HandleFunc("DELETE /api/api-keys/{id}", wrap(s.deleteAPIKey))

// 管理员端点：/api/admin-api-keys（操作任何人的 Key）
mux.HandleFunc("GET /api/admin-api-keys", wrap(s.listAllAPIKeys))
mux.HandleFunc("GET /api/admin-api-keys/{id}", wrap(s.getAnyAPIKey))
mux.HandleFunc("DELETE /api/admin-api-keys/{id}", wrap(s.deleteAnyAPIKey))

// 认证 Webhook：供 MCP Server 调用
mux.HandleFunc("POST /api/api-keys/auth", wrap(s.authenticateAPIKey))
```

对应的 Client 层方法也成对出现：

```go
// 用户方法：带 userID 过滤
func (c *Client) ListAPIKeys(ctx context.Context, userID uint) ([]types.APIKey, error)
func (c *Client) GetAPIKey(ctx context.Context, userID uint, keyID uint) (*types.APIKey, error)
func (c *Client) DeleteAPIKey(ctx context.Context, userID uint, keyID uint) error

// Admin 方法：无 userID 过滤
func (c *Client) ListAllAPIKeys(ctx context.Context) ([]types.APIKey, error)
func (c *Client) GetAPIKeyByID(ctx context.Context, keyID uint) (*types.APIKey, error)
func (c *Client) DeleteAPIKeyByID(ctx context.Context, keyID uint) error
```

### 流程 5：错误累积与批量校验

创建 API Key 时，对多个 MCP Server ID 逐一校验，累积所有错误后一次性返回：

```go
var errs []error
for _, serverID := range req.MCPServerIDs {
    if serverID == "*" {
        continue
    }

    var server v1.MCPServer
    if err := apiContext.Storage.Get(ctx, kclient.ObjectKey{
        Namespace: system.DefaultNamespace, Name: serverID,
    }, &server); err != nil {
        return types2.NewErrBadRequest("MCP server %q not found", serverID)
    }

    hasAccess, err := s.userHasAccessToMCPServer(apiContext, &server)
    if err != nil {
        return types2.NewErrHTTP(http.StatusInternalServerError, fmt.Sprintf("failed to check access: %v", err))
    }
    if !hasAccess {
        errs = append(errs, fmt.Errorf("MCP server %q not found", serverID))
    }
}

if len(errs) > 0 {
    return types2.NewErrHTTP(http.StatusBadRequest, errors.Join(errs...).Error())
}
```

注意：无权限时返回 "not found" 而非 "forbidden"，避免泄露资源存在性。

### 流程 6：GORM 数据库操作模式

```go
// 条件查询：链式 Where
c.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&keys)

// 双条件防越权：keyID + userID
c.db.WithContext(ctx).Where("id = ?", keyID).Where("user_id = ?", userID).First(&key)

// 事务操作：查找 + 验证 + 更新在同一事务中
c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
    if err := tx.Where("id = ?", keyID).First(&apiKey).Error; err != nil {
        return err
    }
    // ... 验证逻辑 ...
    return tx.Model(&apiKey).Update("last_used_at", now).Error
})

// 删除后检查 RowsAffected
result := c.db.WithContext(ctx).Where("id = ?", keyID).Where("user_id = ?", userID).Delete(&types.APIKey{})
if result.RowsAffected == 0 {
    return gorm.ErrRecordNotFound
}

// ErrRecordNotFound 映射为 HTTP 404
if errors.Is(err, gorm.ErrRecordNotFound) {
    return types2.NewErrNotFound("API key not found")
}
```

### 流程 7：Table-Driven 测试（ParseAPIKey）

使用 `wantErrSubstr` 字段验证错误消息内容，覆盖正常路径和各种异常格式：

```go
func TestParseAPIKey(t *testing.T) {
    tests := []struct {
        name          string
        key           string
        wantPrefix    string
        wantUserID    uint
        wantKeyID     uint
        wantSecret    string
        wantErr       bool
        wantErrSubstr string  // 验证错误消息包含特定子串
    }{
        {name: "valid key", key: "ok1-123-456-secretvalue", wantPrefix: "ok1", wantUserID: 123, wantKeyID: 456, wantSecret: "secretvalue"},
        {name: "secret starts with a dash", key: "ok1-1-3--secret", wantPrefix: "ok1", wantUserID: 1, wantKeyID: 3, wantSecret: "-secret"},
        {name: "empty key", key: "", wantErr: true, wantErrSubstr: "invalid API key format"},
        {name: "invalid prefix", key: "ok2-123-456-secret", wantErr: true, wantErrSubstr: "invalid API key prefix"},
        {name: "non-numeric user ID", key: "ok1-abc-456-secret", wantErr: true, wantErrSubstr: "invalid API key format"},
        // ... 更多边界用例
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            prefix, userID, keyID, secret, err := ParseAPIKey(tt.key)
            if tt.wantErr {
                if err == nil {
                    t.Errorf("ParseAPIKey(%q) expected error containing %q, got nil", tt.key, tt.wantErrSubstr)
                    return
                }
                if tt.wantErrSubstr != "" && !strings.Contains(err.Error(), tt.wantErrSubstr) {
                    t.Errorf("ParseAPIKey(%q) error = %q, want error containing %q", tt.key, err.Error(), tt.wantErrSubstr)
                }
                return
            }
            // ... 验证各字段
        })
    }
}
```

---

## 注意事项

### 关键约束

1. **明文密钥仅在创建时返回一次**
   - `CreateAPIKey` 返回 `APIKeyCreateResponse.Key` 包含明文
   - 数据库只存储 `HashedSecret`（bcrypt 哈希）
   - 后续所有查询接口均不返回密钥明文

2. **`last_used_at` 节流更新：超过 1 分钟才写入**
   - 高频认证场景下避免每次请求都写数据库
   - `ValidateAPIKey` 和 `UpdateAPIKeyLastUsed` 都实现了此逻辑（双重保护）
   - 使用 `now.Sub(*key.LastUsedAt) > time.Minute` 判断

3. **用户端点必须带 `userID` 过滤**
   - `GetAPIKey(ctx, userID, keyID)` 同时匹配 `id` 和 `user_id`
   - 防止用户通过猜测 keyID 访问他人的密钥
   - Admin 端点 `GetAPIKeyByID(ctx, keyID)` 不带 `userID` 过滤

4. **无权限时返回 "not found" 而非 "forbidden"**
   - `errs = append(errs, fmt.Errorf("MCP server %q not found", serverID))`
   - 避免泄露资源存在性信息

5. **认证 Webhook 失败不返回 HTTP 错误码**
   - 所有失败路径都返回 `apiKeyAuthResponse{Allowed: false, Reason: "..."}`
   - HTTP 状态码始终为 200，由 `Allowed` 字段表达认证结果

6. **Composite Server 透传检查**
   - 当 API Key 未直接包含目标 MCPID 时，还需检查该 Server 是否是某个 composite server 的子组件
   - 通过 `mcpServer.Spec.CompositeName` 关联到父 composite server

### 常见陷阱

| 问题 | 影响 | 解决方案 |
|------|------|----------|
| 用 `math/rand` 生成密钥 | 可预测，安全漏洞 | 使用 `crypto/rand.Read()` |
| 明文密钥存入数据库 | 数据库泄露即全部暴露 | 只存 bcrypt 哈希，`json:"-"` 隐藏 |
| 每次认证都更新 `last_used_at` | 高频写入压力 | 节流：超过 1 分钟才更新 |
| 用户端点不带 `userID` 过滤 | 越权访问他人密钥 | `Where("user_id = ?", userID)` 双条件 |
| 认证失败返回 HTTP 403 | 调用方无法统一解析 | 始终返回 200 + `{allowed: false}` |
| 通配符 `"*"` 在创建时校验 | 无法校验"所有 Server" | 创建时跳过，认证时实时校验 |

---

## 反模式（避免）

| 反模式 | 正确做法 |
|--------|----------|
| `rand.Intn()` 生成密钥 | `crypto/rand.Read()` + `base64.RawURLEncoding` |
| 明文密钥存数据库 | `bcrypt.GenerateFromPassword()` 存哈希 |
| `HashedSecret` 字段无 `json:"-"` | 始终标记 `json:"-"` 防止序列化泄露 |
| 认证 Webhook 返回 HTTP 401/403 | 返回 200 + `apiKeyAuthResponse{Allowed: false}` |
| 用户端点 `Where("id = ?", keyID)` 单条件 | `Where("id = ?", keyID).Where("user_id = ?", userID)` 双条件 |
| 每次请求都 `UPDATE last_used_at` | `now.Sub(*key.LastUsedAt) > time.Minute` 节流 |
| 无权限时返回 `"access denied"` | 返回 `"not found"` 隐藏资源存在性 |
| Admin 和 User 共用同一路由 | `/api/api-keys` vs `/api/admin-api-keys` 分离 |
| 通配符 `"*"` 创建时遍历所有 Server 校验 | 创建时 `continue`，认证时实时检查 |
| `ParseAPIKey` 测试只验证 `wantErr bool` | 使用 `wantErrSubstr` 验证错误消息内容 |

---

## 相关 Skills

- [obot-authz](../obot-authz/SKILL.md)：RBAC 授权机制与 ACR 集成
- [obot-api-handler](../obot-api-handler/SKILL.md)：API Handler 设计模式（Server 层调用约定）
- [obot-validation](../obot-validation/SKILL.md)：请求参数校验（错误累积、`errors.Join`）
- [obot-service-wiring](../obot-service-wiring/SKILL.md)：服务依赖注入（Gateway Client/Server 的构造）
- [review-go-style](../review-go-style/SKILL.md)：Go 代码风格审查
