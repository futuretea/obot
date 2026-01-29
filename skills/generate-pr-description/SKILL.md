---
name: generate-pr-description
description: |
  指导如何生成标准化的中文 PR 描述:通过比较当前分支与目标分支的差异,按照 Problem/Solution/Test Plan 格式生成结构化描述。
  Use this skill to generate structured PR descriptions in Chinese using the Problem/Solution/Test Plan format based on git diff output.
---

# 生成 PR 描述

## 适用范围

为代码变更生成标准化的中文 Pull Request 描述,遵循 Problem/Solution/Test Plan 三段式格式。

---

## 核心原则

### 1. 固定格式

PR 描述使用三段式结构:

```markdown
# Problem
描述解决的问题或需求背景(1-3 句话)

# Solution
说明实现方案和关键变更(3-8 点)

# Test Plan
列出详细的人工测试步骤(3-8 步)
```

### 2. 内容要求

- 全文使用中文,代码、命令、文件名保持原样
- 严禁使用 emoji
- 基于 `git diff` 分析变更,聚焦核心变更点
- Solution 使用无序列表(`-`)或小标题(`###`)
- Test Plan 使用有序列表(`1.`, `2.`, ...)

---

## 操作流程

### 1. 获取分支差异

```bash
git diff master...HEAD  # 或 main...HEAD
```

### 2. 分析变更内容

从 diff 中识别:
- 核心功能变更(API、控制器、配置)
- 跨文件的关联变更
- 影响现有功能的修改
- 忽略格式化等次要细节

### 3. 填写 Problem 部分

回答:要解决什么问题?为什么需要改动?用 1-3 句话描述。

### 4. 填写 Solution 部分

- 使用无序列表(`-`)或小标题(`###`)组织
- 列出 3-8 个关键实现点
- 每点 1-2 句话,聚焦技术方案
- 不罗列所有文件,只提核心变更

### 5. 填写 Test Plan 部分

- 使用有序列表(`1.`, `2.`, ...)编排 3-8 个测试步骤
- 每步包含:操作 + 预期结果
- 覆盖正常流程和边缘场景
- 写清如何部署、操作、观察指标

### 6. 质量检查

- [ ] 三部分齐全且格式正确
- [ ] Problem ≤ 3 句话,Solution ≤ 8 点
- [ ] Test Plan 可操作且详细
- [ ] 描述与 diff 一致
- [ ] 全文中文,无 emoji

---

## 注意事项

### 避免的反模式

- 列举所有文件 → 聚焦核心变更
- Problem 过于宽泛 → 具体说明解决的问题
- Solution 复制代码 → 用自然语言描述方案
- Test Plan 写"已测试" → 详细操作步骤 + 预期结果
- 中英混杂 → 统一使用中文
- 使用 emoji → 严禁
- 格式混乱 → Solution 用列表/小标题,Test Plan 用有序列表