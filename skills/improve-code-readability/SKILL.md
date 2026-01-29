---
name: improve-code-readability
description: |
  提高代码人类可读性的重构指南。识别并消除过度抽象、职责混乱、命名冗余等影响代码清晰度的模式，优先保持局部性和上下文完整性。
  Use this skill to refactor code for better human readability by reducing over-abstraction, clarifying responsibilities, and maintaining context locality.
---

# 提高代码人类可读性

## 适用范围

用于识别和改进影响代码阅读体验的结构问题：

- **过度抽象**：函数调用层级过深，上下文分散
- **职责混乱**：单个文件包含多个不相关职责
- **命名问题**：冗长但信息量低的命名
- **局部性差**：相关逻辑分散在多处

**不适用场景**：
- 性能优化（使用性能分析工具）
- 语法错误修复（使用 linter）
- 单纯的代码风格问题（使用 review-go-style）

---

## 核心原则

### 1. 局部性优先（Locality First）

相关的逻辑应该在同一处可见：

- ✅ **高局部性**：在一个函数内能看到完整的业务流程
- ❌ **低局部性**：需要跨多个函数跳转才能理解流程

**判断标准**：
- 阅读一个函数时，是否需要频繁"点进去-点回来"？
- 修改一个行为时，是否需要修改 3 个以上的函数？

### 2. 控制抽象层级（Control Abstraction Depth）

函数调用深度应保持在合理范围：

- ✅ **3 层以内**：入口 → 业务逻辑 → 工具函数
- ⚠️ **4-5 层**：需要评估是否过度拆分
- ❌ **6 层以上**：严重影响可读性

**反模式**：
```go
// ❌ 5 层嵌套，每层只做很少的事
Entry() → ProcessData() → ValidateInput() 
  → CheckField() → IsFieldValid()
```

### 3. 避免单行转发函数（No Single-Line Wrappers）

不要为了"封装"而创建只转发调用的函数：

```go
// ❌ 无意义的封装
func (h *Handler) getBranches(ns string) ([]*Branch, error) {
    return h.cache.GetByIndex(IndexName, ns)
}

// ✅ 直接调用
branches, err := h.cache.GetByIndex(IndexName, ns)
```

**例外**：需要统一错误处理或日志时可以封装。

### 4. 内聚职责（Cohesive Responsibilities）

一个函数应该做"一件完整的事"，而不是"一件事的一小步"：

```go
// ❌ 职责过细
func buildRule() {
    if strategy == "header" {
        applyHeader()  // 只是调用另一个函数
    }
}

// ✅ 职责完整
func buildRule() {
    if strategy != "header" {
        return emptyRule
    }
    // 直接构造规则，上下文清晰
    return Rule{Matches: ..., BackendRefs: ...}
}
```

### 5. 显式错误处理策略（Explicit Error Handling）

错误处理应该统一且可预测：

- **选择 A**：返回 error，由调用者决定
- **选择 B**：记录日志并继续执行

**反模式**：同一个文件混用两种策略，导致行为不可预测。

---

## 操作流程

### 流程 1：识别可读性问题

#### 1.1 检查函数调用深度

**工具**：绘制调用图
```
Entry()
  ├─> Layer1()
  │     ├─> Layer2()
  │     │     └─> Layer3()  ← 3层，合理
  │     │           └─> Layer4()  ← 4层，需评估
  │     │                 └─> Layer5()  ← 5层，过深
```

**判断**：
- 超过 4 层 → 考虑合并中间层
- 某层只有单行代码 → 考虑内联

#### 1.2 检查局部性

**测试方法**：随机选择一个函数
- 能否在不跳转的情况下理解它的作用？
- 需要查看多少个其他函数才能理解完整流程？

**指标**：
- ✅ 优秀：0-1 次跳转
- ⚠️ 可接受：2-3 次跳转
- ❌ 差：4 次以上跳转

#### 1.3 检查命名质量

**反模式识别**：
```go
// ❌ 冗长但信息量低
syncHTTPRoutesForBranches()
cleanupHTTPRoutesForBaseNamespace()

// ✅ 精确且简洁
generateDesiredHTTPRoutes()
deleteOrphanedHTTPRoutes()
```

**标准**：
- 函数名应该描述"做什么"，而不是"怎么做"
- 避免重复的前缀/后缀（如 `sync*`、`*ForBaseNamespace`）

---

### 流程 2：应用重构模式

#### 2.1 内联单行转发函数

**识别特征**：
```go
func wrapper(arg) {
    return actualFunc(arg)  // 只有这一行
}
```

**重构**：
1. 找到所有调用 `wrapper` 的地方
2. 替换为 `actualFunc` 的直接调用
3. 删除 `wrapper` 函数

#### 2.2 合并过细的函数

**识别特征**：
- 函数 A 只调用函数 B
- 函数 B 只在函数 A 中被调用
- 两个函数加起来不到 30 行

**重构**：
```go
// 合并前
func processItems() {
    for _, item := range items {
        result := transformItem(item)  // 只调用
        results = append(results, result)
    }
}
func transformItem(item Item) Result {
    return Result{...}  // 简单逻辑
}

// 合并后
func processItems() {
    for _, item := range items {
        result := Result{...}  // 内联，减少跳转
        results = append(results, result)
    }
}
```

#### 2.3 减少函数调用深度

**策略 A**：提升内层函数到外层
```go
// 重构前：3 层
func outer() {
    middle()
}
func middle() {
    inner()
}

// 重构后：2 层
func outer() {
    inner()  // 跳过 middle
}
```

**策略 B**：将深层逻辑内联到调用处
```go
// 重构前：需要跳转
func process() {
    validate()
    transform()
}

// 重构后：逻辑可见
func process() {
    // 验证逻辑内联
    if input == nil {
        return error
    }
    // 转换逻辑内联
    result = transform(input)
}
```

#### 2.4 统一错误处理策略

**重构步骤**：
1. 识别当前文件的错误处理模式
2. 选择主流模式（返回 vs 日志）
3. 统一所有函数的错误处理方式

```go
// ❌ 混乱：有的返回，有的记日志
func syncA() error { return err }
func syncB() { log.Error(err); continue }

// ✅ 统一：都返回 error
func syncA() error { return err }
func syncB() error { return err }
```

---

### 流程 3：验证改进效果

#### 3.1 可读性自测

**测试**：请不熟悉代码的人阅读
- 能否在 5 分钟内理解主流程？
- 修改一个行为需要改动几个函数？

**指标**：
- ✅ 优秀：5 分钟内理解，修改 1-2 个函数
- ⚠️ 可接受：10 分钟理解，修改 2-3 个函数
- ❌ 差：超过 10 分钟，修改 4 个以上函数

#### 3.2 编译和测试

```bash
# 确保重构没有破坏功能
go build ./...
go test ./...
```

---

## 注意事项

### 关键约束

1. **保持功能不变**
   - 重构只改变代码结构，不改变行为
   - 每次重构后立即运行测试

2. **渐进式改进**
   - 不要一次性大规模重构
   - 优先改进最影响阅读的部分

3. **权衡抽象与可读性**
   - 不是所有抽象都是坏的
   - 重复 2-3 次才考虑抽象

### 何时保留抽象

保留抽象的情况：
- 函数被 3 处以上调用
- 包含复杂的错误处理或日志逻辑
- 是公共 API，外部依赖

---

## 反模式（避免）

| 反模式 ❌ | 正确做法 ✅ |
|----------|------------|
| 5 层以上函数嵌套 | 控制在 3 层以内 |
| 单行转发函数 | 直接调用底层函数 |
| 函数只调用一次但单独抽出 | 内联到调用处 |
| 命名冗长但不精确 | 简洁且描述业务意图 |
| 错误处理混用返回和日志 | 统一为返回 error |

---

## 相关 Skills

- [code-simplifier](../code-simplifier/SKILL.md)：通用代码简化指南
- [review-go-style](../review-go-style/SKILL.md)：Go 代码风格审查

---

## 决策树：何时拆分 vs 合并

```
是否需要拆分函数？
├─ 函数超过 100 行？ → 是 → 考虑拆分
├─ 函数做了 3 件以上无关的事？ → 是 → 拆分
├─ 函数被 3 处以上调用？ → 是 → 保持独立
└─ 其他情况 → 保持内联

是否需要合并函数？
├─ 函数只被调用 1 次？ → 是 → 考虑内联
├─ 函数只有 1-5 行代码？ → 是 → 考虑内联
├─ 函数只是转发调用？ → 是 → 内联
└─ 其他情况 → 保持独立
```
