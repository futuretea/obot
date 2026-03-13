---
name: obot-validation
description: |
  基于 obot 项目 pkg/validation/ 的请求参数校验设计指南。涵盖 RuntimeValidator 接口模式、注册表（Registry）模式、结构化错误类型、HTTP 错误返回、正则表达式预编译、多错误合并等核心实践。
  Input validation design patterns for the obot project. Covers RuntimeValidator interface pattern, registry pattern, structured error types, HTTP error returns, pre-compiled regexes, and multi-error joining drawn from pkg/validation/.
---

# Obot 请求参数校验设计模式

## 适用范围

本 Skill 适用于在 `pkg/validation/` 下新增或修改校验逻辑的场景：

- **新增 Runtime 类型**：为新的运行时配置实现校验器
- **API 入参校验**：在 Handler 中调用校验函数，拦截非法请求
- **结构化错误**：返回包含字段名和错误原因的结构化校验错误
- **复合校验**：使用 `errors.Join` 合并多个校验错误

---

## 核心原则

### 1. 接口 + 注册表模式

校验逻辑通过接口抽象，每个 Runtime 类型实现独立的校验器，统一注册到 map 中：

```go
// 接口定义
type RuntimeValidator interface {
    ValidateConfig(manifest types.MCPServerManifest) error
    ValidateCatalogConfig(manifest types.MCPServerCatalogEntryManifest) error
}

// 注册表类型
type RuntimeValidators map[types.Runtime]RuntimeValidator

// 注册表函数——统一的查找入口
func getRuntimeValidators() RuntimeValidators {
    return RuntimeValidators{
        types.RuntimeUVX:           UVXValidator{},
        types.RuntimeNPX:           NPXValidator{},
        types.RuntimeContainerized: ContainerizedValidator{},
        types.RuntimeRemote:        RemoteValidator{},
        types.RuntimeComposite:     CompositeValidator{},
    }
}
```

新增 Runtime 只需：1) 实现接口；2) 在注册表中添加一行。

### 2. 校验器使用值接收器（无状态）

校验器结构体无状态，使用值接收器：

```go
type UVXValidator struct{}  // 空结构体，无状态

func (v UVXValidator) ValidateConfig(manifest types.MCPServerManifest) error {
    // ...
}
```

### 3. 结构化错误，不用裸字符串

校验失败返回 `types.RuntimeValidationError`，而非 `errors.New("invalid")` 或 `fmt.Errorf("invalid")`：

```go
return types.RuntimeValidationError{
    Runtime: types.RuntimeUVX,   // 标识哪个 runtime
    Field:   "uvxConfig.package", // 哪个字段出错
    Message: "package field cannot be empty", // 人类可读的错误描述
}
```

### 4. 校验在 API 层触发，不在存储层

```
请求 → Handler → 调用 validation.ValidateXxx() → 返回 HTTP 400 → 存储 CRUD
```

存储层不负责业务规则校验，校验失败在到达存储之前就返回。

---

## 操作流程

### 流程 1：实现新的校验器

以新增 `DockerValidator` 为例：

```go
// pkg/validation/mcpvalidators.go

// DockerValidator implements RuntimeValidator for Docker runtime
type DockerValidator struct{}

func (v DockerValidator) ValidateConfig(manifest types.MCPServerManifest) error {
    // 1. 先检查 Runtime 类型是否匹配
    if manifest.Runtime != types.RuntimeDocker {
        return types.RuntimeValidationError{
            Runtime: manifest.Runtime,
            Field:   "runtime",
            Message: "expected docker runtime",
        }
    }

    // 2. 检查必要的 Config 字段是否存在
    if manifest.DockerConfig == nil {
        return types.RuntimeValidationError{
            Runtime: types.RuntimeDocker,
            Field:   "dockerConfig",
            Message: "docker configuration is required",
        }
    }

    // 3. 委托给私有方法做具体校验
    return v.validateDockerConfig(*manifest.DockerConfig)
}

func (v DockerValidator) ValidateCatalogConfig(manifest types.MCPServerCatalogEntryManifest) error {
    // 与 ValidateConfig 类似，针对 Catalog 入口
    // ...
}

func (v DockerValidator) validateDockerConfig(config types.DockerRuntimeConfig) error {
    // 空字符串检查：使用 strings.TrimSpace
    if strings.TrimSpace(config.Image) == "" {
        return types.RuntimeValidationError{
            Runtime: types.RuntimeDocker,
            Field:   "image",
            Message: "image field cannot be empty",
        }
    }

    // 范围检查
    if config.Port <= 0 || config.Port > 65535 {
        return types.RuntimeValidationError{
            Runtime: types.RuntimeDocker,
            Field:   "port",
            Message: "port must be between 1 and 65535",
        }
    }

    // 列表元素校验（带下标）
    for i, arg := range config.Args {
        if strings.TrimSpace(arg) == "" {
            return types.RuntimeValidationError{
                Runtime: types.RuntimeDocker,
                Field:   "args[" + strconv.Itoa(i) + "]",
                Message: "argument cannot be empty",
            }
        }
    }

    return nil
}
```

然后在注册表中添加一行：
```go
func getRuntimeValidators() RuntimeValidators {
    return RuntimeValidators{
        // ... 已有 ...
        types.RuntimeDocker: DockerValidator{},  // ← 新增
    }
}
```

### 流程 2：公开校验入口函数

```go
// 公开函数是唯一对外入口，不暴露注册表内部
func ValidateServerManifest(manifest types.MCPServerManifest) error {
    if validator, ok := getRuntimeValidators()[manifest.Runtime]; ok {
        return validator.ValidateConfig(manifest)
    }

    return types.RuntimeValidationError{
        Runtime: manifest.Runtime,
        Field:   "runtime",
        Message: "unsupported runtime",
    }
}
```

### 流程 3：在 Handler 中调用校验

```go
// pkg/api/handlers/mcp.go

func (h *MCPHandler) Create(req api.Context) error {
    var manifest types.MCPServerManifest
    if err := req.Read(&manifest); err != nil {
        return err
    }

    // 在存储前校验；校验失败时应显式包装为 HTTP 400
    // 注意：RuntimeValidationError 本身不会自动映射为 HTTP 400
    // 必须用 types.NewErrBadRequest() 显式包装
    if err := validation.ValidateServerManifest(manifest); err != nil {
        return types.NewErrBadRequest("validation failed: %v", err)
    }

    // 通过校验后再写存储
    server := v1.MCPServer{Spec: manifest}
    return req.Create(&server)
}
```

### 流程 4：URL / 格式校验

```go
// 预编译正则表达式（包级变量，不要在函数内编译）
var (
    hostnameRegex = regexp.MustCompile(`^(?:\*\.)?[a-zA-Z0-9-]+(?:\.[a-zA-Z0-9-]+)*$`)
    urlSchemes    = []string{"http", "https"}
)

func validateURL(rawURL string) error {
    parsedURL, err := url.Parse(rawURL)
    if err != nil {
        return types.RuntimeValidationError{
            Field:   "url",
            Message: fmt.Sprintf("invalid URL format: %v", err),
        }
    }

    if !slices.Contains(urlSchemes, parsedURL.Scheme) {
        return types.RuntimeValidationError{
            Field:   "url",
            Message: "URL scheme must be either https or http",
        }
    }

    return nil
}

func validateHostname(hostname string) error {
    if !hostnameRegex.MatchString(hostname) {
        return types.RuntimeValidationError{
            Field:   "hostname",
            Message: "hostname should only contain alphanumeric and hyphens",
        }
    }
    return nil
}
```

### 流程 5：互斥字段校验（恰好有一个）

```go
func validateExclusiveFields(config RemoteConfig) error {
    hasFixedURL     := strings.TrimSpace(config.FixedURL) != ""
    hasHostname     := strings.TrimSpace(config.Hostname) != ""
    hasURLTemplate  := strings.TrimSpace(config.URLTemplate) != ""

    // 至少一个
    if !hasFixedURL && !hasHostname && !hasURLTemplate {
        return types.RuntimeValidationError{
            Field:   "remoteConfig",
            Message: "either fixedURL, hostname, or urlTemplate must be provided",
        }
    }

    // 最多一个（互斥）
    count := 0
    if hasFixedURL    { count++ }
    if hasHostname    { count++ }
    if hasURLTemplate { count++ }

    if count > 1 {
        return types.RuntimeValidationError{
            Field:   "remoteConfig",
            Message: "cannot specify multiple URL configuration methods",
        }
    }

    return nil
}
```

### 流程 6：合并多个校验错误

```go
import "errors" // 标准库，Go 1.20+ 支持 errors.Join

func validateCompositeConfig(config CompositeConfig) error {
    var errs []error

    for i, component := range config.ComponentServers {
        if err := validateComponent(i, component); err != nil {
            errs = append(errs, err)
        }
    }

    // errors.Join 将多个错误合并为一个，保留所有错误信息
    return errors.Join(errs...)
}
```

### 流程 7：HTTP 层错误（非校验错误）

与 `RuntimeValidationError` 不同，Handler 层的权限、业务前置条件检查使用 `types.NewErrHTTP`：

```go
// 权限检查：403 Forbidden
if !req.UserIsAdmin() {
    return types.NewErrHTTP(http.StatusForbidden, "admin role required")
}

// 前置条件检查：409 Conflict
if resource.Status.Active {
    return types.NewErrHTTP(http.StatusConflict, "resource is already active")
}

// 未找到：404 Not Found
if !found {
    return types.NewErrHTTP(http.StatusNotFound, "resource not found")
}
```

---

## 注意事项

### 关键约束

1. **空字符串检查用 `strings.TrimSpace`**
   - `config.Field == ""` 无法捕获全空格字符串
   - 始终用 `strings.TrimSpace(config.Field) == ""`

2. **正则表达式在包级预编译**
   - 函数内 `regexp.MustCompile(...)` 每次调用都重新编译，性能差
   - 在包级声明 `var myRegex = regexp.MustCompile(...)`

3. **下标从 0 开始，字段名要带下标**
   - 校验列表元素时：`Field: "args[" + strconv.Itoa(i) + "]"`
   - 让用户知道是第几个元素出错

4. **`RuntimeValidationError` 不自动映射到 HTTP 400**
   - 框架错误处理仅识别 `types.ErrHTTP` 和 `apierrors.StatusError`
   - 直接返回 `RuntimeValidationError` 会被当作 500 内部错误
   - 正确做法：`return types.NewErrBadRequest("validation failed: %v", err)`

5. **校验器不做 I/O 操作**
   - 纯函数，只检查输入数据
   - 不查询数据库、不调用外部服务

6. **不暴露注册表**
   - `getRuntimeValidators()` 为包私有函数
   - 对外只暴露 `ValidateServerManifest()`、`ValidateCatalogEntryManifest()` 等入口

### 常见陷阱

| 问题 | 影响 | 解决方案 |
|------|------|----------|
| 返回 `errors.New("invalid field")` | 信息不足，难以定位 | 使用 `types.RuntimeValidationError{Field, Message}` |
| 在函数内 `regexp.MustCompile(...)` | 性能差 | 包级变量预编译 |
| 先写存储再校验 | 脏数据入库 | 先校验再写存储 |
| 对空字符串用 `== ""` 而非 `TrimSpace` | 误放全空格字段 | `strings.TrimSpace(s) == ""` |

---

## 反模式（避免）

| 反模式 ❌ | 正确做法 ✅ |
|----------|------------|
| `return errors.New("package is required")` | `return types.RuntimeValidationError{Field: "package", Message: "..."}` |
| `regexp.MustCompile(pattern)` in function body | 包级 `var re = regexp.MustCompile(pattern)` |
| 在存储层 webhook 中做业务校验 | 在 API Handler 层调用 `validation.ValidateXxx()` |
| 一个 Validator 处理所有 Runtime 类型 | 每个 Runtime 独立实现，注册到 map |
| `config.Field == ""` 检查空字符串 | `strings.TrimSpace(config.Field) == ""` |

---

## 相关 Skills

- [obot-api-handler](../obot-api-handler/SKILL.md)：在 Handler 中调用校验函数
- [obot-authz](../obot-authz/SKILL.md)：授权层的权限检查（与校验层分离）
- [review-go-style](../review-go-style/SKILL.md)：Go 代码风格审查
