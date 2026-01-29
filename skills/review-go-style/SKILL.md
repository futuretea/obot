---
name: review-go-style
description: |
  基于 Uber Go Style Guide 的代码风格审查指导。涵盖 Go 代码的惯用约定、性能优化和编程模式，确保代码符合工业标准和最佳实践。
  适用于 Go 代码的风格审查、重构优化和代码评审场景。
  Code style review based on Uber Go Style Guide. Covers idiomatic conventions, performance optimizations, and programming patterns to ensure code meets industry standards and best practices.
---

# Go 代码风格审查

## 适用范围

基于 [Uber Go Style Guide](https://github.com/uber-go/guide) 的代码风格审查：

- **代码评审**：检查 Go 代码是否符合惯用约定
- **重构优化**：识别性能问题和反模式
- **代码质量提升**：确保代码可维护性和可读性
- **团队协作**：统一代码风格标准

---

## 核心原则

### 1. 接口使用规范

- **永远不要使用指向接口的指针**：接口本身是引用类型
- **编译时验证接口实现**：使用 `var _ InterfaceName = (*Type)(nil)` 模式
- **零值可用性**：确保零值的接收器可以正常调用方法

### 2. 资源管理

- **使用 defer 释放资源**：文件、锁、连接等必须用 defer 关闭
- **在边界处拷贝 Slices 和 Maps**：避免外部修改影响内部状态
- **Channel 要么无缓冲，要么 size=1**：避免不明确的缓冲大小

### 3. 错误处理

- **一次处理错误**：不要既打日志又返回错误
- **错误包装使用 `%w`**：需要调用者判断错误类型时使用 `fmt.Errorf("%w", err)`
- **错误命名以 `Err` 开头**：例如 `ErrNotFound`

### 4. 性能优化

- **优先使用 `strconv` 而不是 `fmt`**：类型转换性能更好
- **指定容器容量**：预分配 slice/map 容量减少扩容
- **避免字符串到字节的转换**：使用 `[]byte` 直接操作

### 5. 代码规范

- **相似声明放一组**：使用括号组织 import、const、var
- **减少嵌套**：优先处理错误和特殊情况，提前返回
- **缩小变量作用域**：在最小作用域声明变量
- **使用字段名初始化结构体**：避免位置依赖

---

## 操作流程

### 流程 1：代码风格审查

#### 1.1 接口和类型检查

**检查项**：
- [ ] 是否有指向接口的指针（`*InterfaceName`）
- [ ] 接口实现是否有编译时验证
- [ ] 零值 Mutex/sync 类型是否正确使用

**示例**：
```go
// ❌ 错误：指向接口的指针
func ProcessData(cfg *ConfigInterface) error

// ✅ 正确：接口作为值传递
func ProcessData(cfg ConfigInterface) error

// ✅ 编译时验证接口实现
var _ http.Handler = (*Server)(nil)
```

#### 1.2 资源管理检查

**检查项**：
- [ ] 文件、连接、锁是否使用 defer 关闭
- [ ] 接收/返回的 slice/map 是否需要拷贝
- [ ] goroutine 生命周期是否明确管理

**示例**：
```go
// ✅ 使用 defer 关闭资源
func ReadConfig(path string) ([]byte, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()
    return io.ReadAll(f)
}

// ✅ 边界拷贝防止外部修改
func (s *Store) Items() []Item {
    s.mu.RLock()
    defer s.mu.RUnlock()
    items := make([]Item, len(s.items))
    copy(items, s.items)
    return items
}
```

#### 1.3 错误处理检查

**检查项**：
- [ ] 错误是否只处理一次（避免既打日志又返回）
- [ ] 错误包装是否使用 `%w`（需要判断类型时）
- [ ] 错误变量命名是否以 `Err` 开头

**示例**：
```go
// ❌ 错误：既打日志又返回错误
if err := process(); err != nil {
    log.Error("process failed", err)
    return err  // 调用者可能再次打日志
}

// ✅ 正确：只返回错误，由调用者决定如何处理
if err := process(); err != nil {
    return fmt.Errorf("process data: %w", err)
}

// ✅ 错误变量命名
var ErrNotFound = errors.New("resource not found")
```

#### 1.4 性能优化检查

**检查项**：
- [ ] 类型转换是否使用 `strconv` 而不是 `fmt`
- [ ] slice/map 是否预分配容量
- [ ] 是否有不必要的字符串到字节转换

**示例**：
```go
// ❌ 性能较差
s := fmt.Sprint(123)

// ✅ 性能更好
s := strconv.Itoa(123)

// ✅ 预分配容量
items := make([]Item, 0, len(source))
```

#### 1.5 代码风格检查

**检查项**：
- [ ] import 是否按标准库/第三方/本项目分组
- [ ] 是否有不必要的 else
- [ ] 变量作用域是否最小化
- [ ] 结构体初始化是否使用字段名

**示例**：
```go
// ❌ 不必要的 else
if condition {
    return true
} else {
    return false
}

// ✅ 提前返回
if condition {
    return true
}
return false

// ✅ 使用字段名初始化
cfg := Config{
    Host: "localhost",
    Port: 8080,
}
```

---

### 流程 2：重构优化

#### 2.1 识别反模式

常见反模式：
- **panic 滥用**：除 `init()` 外禁止使用 panic
- **全局可变变量**：避免全局状态，使用依赖注入
- **goroutine 泄漏**：确保 goroutine 有明确退出机制
- **内置名称覆盖**：避免使用 `error`、`string` 等作为变量名

#### 2.2 应用最佳实践

**时间处理**：
```go
// ✅ 使用 time.Duration 而不是整数
func Poll(delay time.Duration) {
    time.Sleep(delay)
}

// ✅ 使用 time.Time 表达瞬时时间
func Process(deadline time.Time) error {
    if time.Now().After(deadline) {
        return ErrTimeout
    }
    // ...
}
```

**枚举类型**：
```go
// ✅ 枚举从 1 开始，0 表示无效值
type Status int

const (
    StatusUnknown Status = iota  // 0 为无效值
    StatusPending                // 1
    StatusRunning                // 2
)
```

**函数选项模式**：
```go
// ✅ 使用函数选项提供灵活配置
type Option func(*Server)

func WithTimeout(d time.Duration) Option {
    return func(s *Server) {
        s.timeout = d
    }
}

func NewServer(opts ...Option) *Server {
    s := &Server{timeout: 30 * time.Second}
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

---

### 流程 3：代码评审输出

#### 3.1 分类问题

按严重程度分类：
- **🔴 Must Fix**：违反核心原则，必须修改
- **🟡 Should Fix**：影响性能或可维护性
- **🟢 Consider**：优化建议，可选

#### 3.2 提供具体修改建议

每个问题包含：
1. **问题描述**：说明违反了哪条规范
2. **影响**：解释为什么需要修改
3. **修改建议**：提供具体代码示例

---

## 注意事项

### 关键约束

1. **不要过度优化**
   - 先保证代码正确和可读
   - 性能问题出现后再针对性优化

2. **保持一致性**
   - 与现有代码风格保持一致
   - 同一文件/包内规则统一

3. **实用主义**
   - 规则服务于可维护性
   - 不要为了规则而牺牲可读性

### 常见陷阱

| 问题 | 影响 | 解决方案 |
|------|------|----------|
| 接口指针 | 语义不明确 | 使用接口值传递 |
| 忘记 defer 关闭资源 | 资源泄漏 | 获取资源后立即 defer |
| 错误处理两次 | 日志重复 | 选择打日志或返回错误 |
| 全局可变状态 | 并发不安全 | 使用依赖注入 |

---

## 反模式（避免）

| 反模式 ❌ | 正确做法 ✅ |
|----------|------------|
| `func f(cfg *ConfigInterface)` | `func f(cfg ConfigInterface)` |
| `fmt.Sprint(123)` | `strconv.Itoa(123)` |
| 既打日志又返回错误 | 只返回错误，由调用者处理 |
| `make([]T, 0)` 然后 append | `make([]T, 0, expectedSize)` |
| 使用 `panic` 处理普通错误 | 返回 `error` |

---

## 快速检查命令

```bash
# 运行 linters
golangci-lint run

# 格式化代码
goimports -w .

# 检查 vet 错误
go vet ./...

# 运行测试
go test -race ./...
```

---

## 审查清单

### 代码结构
- [ ] 接口使用正确（无指针，有编译验证）
- [ ] 资源管理使用 defer
- [ ] 错误处理符合规范
- [ ] 变量作用域最小化

### 性能
- [ ] 使用 `strconv` 而不是 `fmt` 做类型转换
- [ ] 容器预分配容量
- [ ] 避免不必要的内存分配

### 风格
- [ ] import 分组正确
- [ ] 无不必要的 else
- [ ] 结构体初始化使用字段名
- [ ] 命名符合 Go 惯例

### 并发
- [ ] goroutine 生命周期明确
- [ ] Channel 使用正确
- [ ] 并发安全（data race 检查）
