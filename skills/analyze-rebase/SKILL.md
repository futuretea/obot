---
name: analyze-rebase
description: |
  分析分叉分支 rebase 的后果，包括功能影响、兼容性风险、冲突预测和安全执行策略，在执行 rebase 前生成结构化分析报告。
  Analyze the consequences of rebasing a diverged branch, covering functional impact, compatibility risks, conflict prediction, and safe execution strategy. Generates a structured analysis report before rebase execution.
---

# Rebase 影响分析器（Rebase Impact Analyzer）

## 适用范围

本 Skill 适用于以下场景：

- 本地分支**同时领先和落后**于目标分支（分叉状态）
- 需要在执行 `git rebase` 前**评估风险和影响**
- 涉及跨多个模块、文件或功能域的变更
- 需要判断 rebase 后是否会引入**功能性回归或兼容性问题**

**不适用场景**：

- 线性快进合并（fast-forward），无需分析
- 已完成 rebase 后的问题修复（此时应使用 debug 流程）
- 纯粹的 git 操作教学

---

## 核心原则

### 1. 安全第一（Safety First）

- **分析先于操作**：绝不在未分析前执行 rebase
- **备份先于 rebase**：始终创建备份分支 `git branch backup-<branch>-<date>`
- **可逆性保障**：确保任何操作都可以回退到 rebase 前的状态

### 2. 理解分叉拓扑（Understand Divergence Topology）

- 精确识别**落后的 commit**（目标分支有而本地没有的）
- 精确识别**领先的 commit**（本地有而目标分支没有的）
- 找到**共同祖先**（merge-base），理解分叉起点
- 绘制分叉拓扑图，直观展示分支关系

### 3. 区分冲突层次（Classify Conflict Levels）

冲突不仅仅是 git 报告的文本冲突，需要区分三个层次：

| 层次 | 说明 | 示例 |
|------|------|------|
| **文本冲突** | git 检测到的同文件同区域修改 | 两个分支都修改了同一行代码 |
| **语义冲突** | 无文本冲突但逻辑上不兼容 | A 分支重命名了函数，B 分支在旧名称上调用 |
| **功能冲突** | 无冲突但行为发生意外变化 | A 分支改变了默认值，B 分支依赖旧默认值 |

### 4. 评估功能影响（Evaluate Functional Impact）

- 分类每个变更的影响域：API、数据模型、配置、依赖、业务逻辑
- 评估变更的传播范围：局部、模块级、全局
- 识别高风险变更：接口签名变更、数据库 schema 变更、配置格式变更

### 5. 保护本地工作（Preserve Local Work）

- 本地 commit 的完整性是最高优先级
- 识别哪些本地 commit 最可能受 rebase 影响
- 为高风险的本地 commit 制定单独的处理策略

---

## 操作流程

### 阶段 0：确定分支角色

在开始分析前，必须先明确两个分支的角色：

- **local-branch**：你当前正在工作的分支（即将被 rebase 的分支），通过 `git branch --show-current` 自动获取
- **target-branch**：你要 rebase 到的目标分支（基准分支），**默认为 `main`**，用户可指定其他分支

#### 0.1 自动识别当前分支

```bash
# 当前所在分支就是 local-branch
git branch --show-current

# 查看所有本地分支及其跟踪关系
git branch -vv
```

#### 0.2 确定目标分支

target-branch 通常是以下之一：

| 场景 | local-branch | target-branch | 说明 |
|------|-------------|---------------|------|
| 同步上游主线 | master | main | 本地 master 落后于 main（或反之） |
| 同步远程 | main | origin/main | 本地 main 与远程 main 分叉 |
| 功能分支更新 | feature-xxx | main | 功能分支需要更新基准 |
| 发布分支同步 | release-x.y | main | 发布分支需要合入最新修复 |

**判断方法**：

```bash
# 查看分叉状态（ahead/behind）
git rev-list --count --left-right <local-branch>...<target-branch>
# 输出: <ahead>\t<behind>
# ahead = 本地领先的 commit 数
# behind = 本地落后的 commit 数

# 如果是远程分支，先 fetch 确保最新
git fetch origin
git rev-list --count --left-right HEAD...origin/main
```

#### 0.3 确认分叉状态

只有**同时 ahead > 0 且 behind > 0** 时，才属于分叉状态，需要使用本 Skill 分析。

- ahead > 0, behind = 0 → 无需 rebase，本地纯领先
- ahead = 0, behind > 0 → 快进合并即可：`git rebase <target>` 或 `git merge --ff-only <target>`
- ahead > 0, behind > 0 → **分叉状态，需要分析**（本 Skill 的核心场景）

---

### 阶段 1：分叉状态分析

#### 1.1 确定分叉拓扑

```bash
# 找到共同祖先
git merge-base <local-branch> <target-branch>

# 查看落后的 commit（目标分支有，本地没有）
git log --oneline <local-branch>..<target-branch>

# 查看领先的 commit（本地有，目标分支没有）
git log --oneline <target-branch>..<local-branch>

# 统计分叉程度
git rev-list --count --left-right <local-branch>...<target-branch>
# 输出格式: <ahead>\t<behind>
```

#### 1.2 可视化分叉图

```bash
# 图形化展示分叉
git log --oneline --graph --left-right <local-branch>...<target-branch>

# 查看完整的分叉历史
git log --oneline --graph --all --decorate \
  $(git merge-base <local-branch> <target-branch>)^..<local-branch> \
  $(git merge-base <local-branch> <target-branch>)^..<target-branch>
```

#### 1.3 分叉摘要

生成如下格式的摘要：

```
分叉分析摘要
=============
本地分支:     <local-branch>
目标分支:     <target-branch>
共同祖先:     <commit-hash> (<date>)
领先 commit:  N 个
落后 commit:  M 个
分叉时间跨度: X 天
```

---

### 阶段 2：冲突预测

#### 2.1 文件级冲突扫描

```bash
# 列出目标分支修改的文件
git diff --name-only $(git merge-base <local> <target>)..<target>

# 列出本地分支修改的文件
git diff --name-only $(git merge-base <local> <target>)..<local>

# 找出双方都修改的文件（潜在冲突文件）
comm -12 \
  <(git diff --name-only $(git merge-base <local> <target>)..<target> | sort) \
  <(git diff --name-only $(git merge-base <local> <target>)..<local> | sort)
```

#### 2.2 模拟 rebase 冲突检测

```bash
# 在临时分支上模拟 rebase（不影响当前分支）
git stash  # 暂存未提交的修改
git checkout -b temp-rebase-test <local-branch>
git rebase --no-commit <target-branch> || true
# 查看冲突文件
git diff --name-only --diff-filter=U
# 清理
git rebase --abort
git checkout <local-branch>
git branch -D temp-rebase-test
git stash pop  # 恢复暂存的修改
```

#### 2.3 冲突分类

将发现的冲突按以下维度分类：

| 文件 | 冲突类型 | 严重程度 | 影响范围 | 处理建议 |
|------|----------|----------|----------|----------|
| path/to/file | 文本/语义/功能 | 高/中/低 | 局部/模块/全局 | 具体建议 |

---

### 阶段 3：功能影响分析

#### 3.1 变更分类

将目标分支的 incoming commit 按功能域分类：

- **API 变更**：接口签名、路由、请求/响应格式
- **数据模型变更**：结构体定义、数据库 schema、序列化格式
- **配置变更**：配置文件格式、默认值、环境变量
- **依赖变更**：go.mod / package.json / requirements.txt 变更
- **业务逻辑变更**：核心算法、流程控制、状态机
- **基础设施变更**：CI/CD、Dockerfile、Makefile

```bash
# 查看 incoming 变更的详细 diff，按类型筛选
git diff $(git merge-base <local> <target>)..<target> -- "*.go"
git diff $(git merge-base <local> <target>)..<target> -- "go.mod" "go.sum"
git diff $(git merge-base <local> <target>)..<target> -- "*.yaml" "*.yml" "*.json"
git diff $(git merge-base <local> <target>)..<target> -- "Makefile" "Dockerfile*"
```

#### 3.2 影响传播分析

对每个 incoming 变更，分析其对本地 commit 的传播影响：

1. **直接影响**：本地修改的文件是否依赖 incoming 变更的文件？
2. **间接影响**：incoming 变更是否修改了本地代码使用的接口/函数？
3. **编译影响**：incoming 变更是否会导致本地代码编译失败？
4. **运行时影响**：incoming 变更是否改变了本地代码依赖的运行时行为？

```bash
# 查看本地 commit 涉及的 import/依赖
git diff $(git merge-base <local> <target>)..<local> -- "*.go" | grep -E "^[+-].*import"

# 检查函数签名变更
git diff $(git merge-base <local> <target>)..<target> -- "*.go" | grep -E "^[+-]func "
```

#### 3.3 影响评估矩阵

| 影响域 | incoming 变更 | 本地 commit 受影响 | 严重程度 | 说明 |
|--------|--------------|-------------------|----------|------|
| API | 列出变更 | 是/否 | 高/中/低 | 具体说明 |
| 数据模型 | 列出变更 | 是/否 | 高/中/低 | 具体说明 |
| 配置 | 列出变更 | 是/否 | 高/中/低 | 具体说明 |
| 依赖 | 列出变更 | 是/否 | 高/中/低 | 具体说明 |

---

### 阶段 4：兼容性评估

#### 4.1 API 兼容性

检查以下断裂性变更：

- 函数/方法签名变更（参数增减、类型变更）
- 接口定义变更（新增方法、移除方法）
- 导出符号重命名或移除
- 返回值类型或数量变更

```bash
# 对比 incoming 变更中的接口和函数签名
git diff $(git merge-base <local> <target>)..<target> -- "*.go" \
  | grep -E "^[+-].*(func |type .* interface)"
```

#### 4.2 依赖兼容性

```bash
# 检查 go.mod 变更
git diff $(git merge-base <local> <target>)..<target> -- "go.mod"

# 如果是前端项目
git diff $(git merge-base <local> <target>)..<target> -- "package.json"
```

重点关注：

- 直接依赖的大版本升级
- 间接依赖冲突（本地和 incoming 依赖同一库的不同版本）
- 新增或移除的依赖

#### 4.3 配置兼容性

- 配置文件格式变更（字段增删、嵌套结构变化）
- 环境变量变更（新增必需变量、重命名）
- 默认值变更（可能影响运行时行为）

#### 4.4 生成兼容性报告

```
兼容性评估
==========
API 兼容性:    [兼容 / 需适配 / 不兼容]
依赖兼容性:    [兼容 / 需适配 / 不兼容]
配置兼容性:    [兼容 / 需适配 / 不兼容]
数据格式兼容性: [兼容 / 需适配 / 不兼容]

总体风险等级:   [低 / 中 / 高 / 极高]
```

---

### 阶段 5：风险矩阵与决策

#### 5.1 综合风险矩阵

| 风险项 | 概率 | 影响 | 风险等级 | 缓解措施 |
|--------|------|------|----------|----------|
| 文本冲突 | 高/中/低 | 高/中/低 | 红/黄/绿 | 具体措施 |
| 语义冲突 | 高/中/低 | 高/中/低 | 红/黄/绿 | 具体措施 |
| 编译失败 | 高/中/低 | 高/中/低 | 红/黄/绿 | 具体措施 |
| 运行时回归 | 高/中/低 | 高/中/低 | 红/黄/绿 | 具体措施 |
| 依赖冲突 | 高/中/低 | 高/中/低 | 红/黄/绿 | 具体措施 |

#### 5.2 决策建议

根据风险矩阵，给出以下三种建议之一：

**低风险 -- 直接 rebase**：
- 冲突少（<5 个文件）、无语义冲突、无 API 断裂
- 建议：`git rebase <target>` + 验证编译和测试

**中风险 -- 交互式 rebase**：
- 存在部分冲突但可控、或有少量语义冲突
- 建议：`git rebase -i <target>`，逐 commit 处理

**高风险 -- 分阶段处理**：
- 大量冲突、API 断裂、依赖不兼容
- 建议：按功能域拆分本地 commit，分批 rebase

---

### 阶段 6：安全执行策略

#### 6.1 执行前准备

```bash
# 1. 创建备份分支
git branch backup-<branch>-$(date +%Y%m%d) <local-branch>

# 2. 确保工作区干净
git status
git stash  # 如有未提交的修改

# 3. 更新目标分支到最新
git fetch origin <target-branch>
```

#### 6.2 执行 rebase

```bash
# 低风险：直接 rebase
git rebase <target-branch>

# 中/高风险：交互式 rebase
git rebase -i <target-branch>
# 在编辑器中审查每个 commit，可以 reorder/squash/edit

# 遇到冲突时
git diff          # 查看冲突内容
# 手动解决冲突
git add <file>    # 标记已解决
git rebase --continue

# 放弃 rebase（任何时候都可以）
git rebase --abort
```

#### 6.3 执行后验证

```bash
# 1. 编译验证
go build ./...    # Go 项目
# npm run build  # 前端项目

# 2. 测试验证
go test ./...     # Go 项目
# npm test       # 前端项目

# 3. diff 验证：确认本地变更完整保留
git diff <target-branch>..HEAD  # 查看 rebase 后的本地变更

# 4. 与备份对比
git diff backup-<branch>-<date>..HEAD  # 应该只有 incoming 变更的差异

# 5. 确认无误后删除备份
git branch -D backup-<branch>-<date>
```

---

## 输出报告模板

执行完整分析后，生成如下结构的报告：

```markdown
# Rebase 影响分析报告

## 1. 分叉概览
- 本地分支: xxx
- 目标分支: xxx
- 共同祖先: xxx (yyyy-mm-dd)
- 领先: N 个 commit
- 落后: M 个 commit

## 2. 冲突预测
### 文本冲突（X 个文件）
- file1: 描述
- file2: 描述

### 语义冲突（X 处）
- 描述1
- 描述2

### 功能冲突（X 处）
- 描述1
- 描述2

## 3. 功能影响
| 影响域 | 变更内容 | 影响本地 | 严重程度 |
|--------|----------|----------|----------|
| ...    | ...      | ...      | ...      |

## 4. 兼容性评估
- API: [兼容/需适配/不兼容]
- 依赖: [兼容/需适配/不兼容]
- 配置: [兼容/需适配/不兼容]

## 5. 风险等级: [低/中/高]

## 6. 推荐策略
[具体执行建议]

## 7. 执行步骤
1. ...
2. ...
```

---

## 注意事项

### 关键约束

1. **绝不跳过备份**
   - rebase 前必须创建备份分支
   - 备份分支命名包含日期，便于追溯

2. **绝不忽略语义冲突**
   - git 不会报告语义冲突
   - 需要人工审查 incoming 变更对本地代码的隐含影响
   - 特别注意接口变更、默认值变更、行为变更

3. **绝不盲目 force push**
   - rebase 后需要 force push 时，先确认远程分支无他人使用
   - 优先使用 `--force-with-lease` 而非 `--force`

4. **渐进式处理**
   - 面对大量冲突时，不要试图一次性解决
   - 按功能域或时间顺序分批处理

### 常见陷阱

| 陷阱 | 后果 | 预防措施 |
|------|------|----------|
| 未创建备份直接 rebase | 丢失本地 commit | 始终先 `git branch backup-...` |
| 只检查文本冲突 | 语义冲突导致运行时错误 | 审查 incoming 的 API/接口变更 |
| rebase 后未验证编译 | 提交了无法编译的代码 | rebase 后立即 `go build ./...` |
| 忽略依赖变更 | go.sum / lock 文件冲突 | 单独检查 go.mod 和 lock 文件 |
| force push 到共享分支 | 覆盖他人工作 | 使用 `--force-with-lease` |
| 在脏工作区 rebase | 未提交的修改丢失 | 先 `git stash` 或 `git commit` |

---

## 相关 Skills

- [generate-pr-description](../generate-pr-description/SKILL.md)：rebase 完成后生成 PR 描述
- [code-simplifier](../code-simplifier/SKILL.md)：rebase 后对冲突解决代码进行简化
