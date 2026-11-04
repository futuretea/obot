# design-interview · uni CLI 员工可见层更名

```yaml
design_interview:
  mode: completed
  skipped_reason:
  confirmed_answers:
    - question: 更名范围是员工可见层全部 obot → uni，还是只改界面文案？
      answer: 全改：二进制/命令名、flag、员工环境变量、钥匙串服务名、配置目录、页面复制命令、引导文件、文案
      evidence: 会话 2026-11-04，用户原话「肯定是要全改的」；此前我列出的清单含 OBOT_TOKEN/OBOT_BASE_URL/OBOT_NON_INTERACTIVE、钥匙串服务名、obot/config.json，用户以此回复
      confidence: high
      risk_if_wrong: 若用户只想要界面文案，CLI/资产改名是多余返工（可 revert）
      user_answer: 肯定是要全改的
      final_decision: 员工可见层全量更名
      decision_source: user_confirmed
      decision_owner: user
      decision_state: accepted
      affects: [scope, compatibility, acceptance]
      mapped_to: [goals, non_goals, success_criteria]
    - question: 隧道弹窗如何处理 Docker 命令（内网拉不到 ghcr.io 上游镜像）？
      answer: 去掉 Docker 标签，只保留一条 uni 命令
      evidence: 会话 2026-11-04，用户原话「我们可以去掉 Docker 标签，只留 uni 命令」
      confidence: high
      risk_if_wrong: 若仍有人以容器方式跑隧道，需自行用 uni 二进制；内网镜像仓库方案未定（见 closure advisories）
      user_answer: 去掉 Docker 标签，只留 uni 命令
      final_decision: 弹窗仅保留 uni tunnel 命令，无 Docker 标签
      decision_source: user_confirmed
      decision_owner: user
      decision_state: accepted
      affects: [scope, acceptance]
      mapped_to: [goals, success_criteria]
    - question: 服务端配置与 API 契约是否跟随更名？
      answer: 不改：OBOT_SERVER_*、登录地址、API 路径与字段（含 version.current.obot、remoteConfig.tunnelName）、Go module 路径
      evidence: 会话 2026-11-04 align 轮已确认「正在跑的服务端配置名不动」；用户未对该边界提出异议并转入执行
      confidence: high
      risk_if_wrong: 若用户后续想连服务端变量一起改，属新切片（当前部署配置会断）
      user_answer: （align 轮确认）
      final_decision: 服务端环境变量与 API 契约保持 obot
      decision_source: user_confirmed
      decision_owner: user
      decision_state: accepted
      affects: [scope, compatibility]
      mapped_to: [non_goals, decision_tree]
    - question: CLI 契约（flag/环境变量/钥匙串/配置路径）改名的兼容策略？
      answer: breaking-by-design：当前无已登录员工、CLI 尚未经内网分发，无存量消费者
      evidence: 会话 2026-11-04 用户原话「没有已经登录的员工」；内网下载服务器尚未建立（会话确认分发仍在规划）
      confidence: high
      risk_if_wrong: 若有未上报的存量安装，旧 keyring/config 失效需重新 setup（影响 = 重跑一次 setup）
      user_answer: 没有已经登录的员工
      final_decision: breaking-by-design，不做旧名迁移/别名
      decision_source: user_confirmed
      decision_owner: user
      decision_state: accepted
      affects: [compatibility, acceptance]
      mapped_to: [decision_tree]
    - question: Profile 菜单是否显示 "Obot v<版本>"，需要改成 "uni v<版本>"？
      answer: 前提不成立：当前代码不渲染 "Obot v" 前缀（Profile.svelte:347 直接渲染 {version.current.obot} 值；git log -S 'Obot v' 无结果）。但版本对话框把 API 字段名 key 当标签渲染（Profile.svelte:384 {key}），"obot:" 对用户可见
      evidence: Research sidecar 扫描 + 主 agent 读码复核（Profile.svelte:325-394）
      confidence: high
      risk_if_wrong: 无（事实修正，非决策）
      final_decision: 字段名 obot 不动（API 契约）；版本对话框增加显示层标签映射 obot→uni；其余版本展示不变
      decision_source: evidence_verified
      decision_owner: evidence
      decision_state: accepted
      affects: [scope]
      mapped_to: [goals]
    - question: 引导文件（bootstrap skills）目录名与模板是否纳入更名？
      answer: 纳入：四个资产目录 obot* → uni*，模板内 frontmatter name 与命令文本同步
      evidence: rethink round 1 发现 pkg/localagents/assets/files/skills/obot* 目录与模板内 obot 命令引用；用户「全改」覆盖员工装到本机 AI 客户端里的可见文件
      confidence: high
      risk_if_wrong: 无存量安装（无已登录员工），无迁移负担
      final_decision: 引导文件目录名与内容一并更名，不做旧目录清理
      decision_source: user_confirmed
      decision_owner: user
      decision_state: accepted
      affects: [scope]
      mapped_to: [goals]
    - question: 员工可见的非品牌契约名（.obotcatalogs/.ignoreobotcatalogs/.obot-catalog-* 临时前缀、obot-sentry、外链 docs.obot.ai、吉祥物资源/CSS 类、后端 "Obot MCP Server" 显示名）是否更名？
      answer: 不纳入：均为非 CLI 品牌文案——目录约定名是 catalog gitops 数据契约；obot-sentry 是另一产品二进制；外链指向外部站点；吉祥物资源与 CSS 类不渲染 "Obot" 文字；后端显示名属服务端字符串切片
      evidence: Research sidecar 影响面扫描（d/e/c 节）；dashboard 过滤逻辑 startsWith('Obot ') 与后端 controller.go:288 显示名配对，改后端即越出 CLI 切片
      confidence: medium
      risk_if_wrong: 若用户认为目录约定名也算品牌暴露面，需扩片（返工 = pkg/mcpcatalog 共享代码 + 存量 catalog 仓库兼容）
      final_decision: 列为非目标，登记后续增强候选
      decision_source: evidence_verified
      decision_owner: default
      decision_state: accepted
      affects: [scope]
      mapped_to: [non_goals]
  unresolved_questions: []
  measurable_goal_gaps: []
```

## 决策摘要（阶段 2.5 输入）

| # | 决策 | 来源 | 状态 |
|---|------|------|------|
| D1 | 员工可见层全量更名 obot→uni（二进制/命令/flag/env/钥匙串/配置路径/引导文件/UI 命令文案/当前版本文档命令引用） | user_confirmed | accepted |
| D2 | 隧道弹窗去 Docker 标签，仅保留 uni 命令 | user_confirmed | accepted |
| D3 | 服务端 OBOT_SERVER_*、API 路径与字段、module 路径不改 | user_confirmed | accepted |
| D4 | CLI 契约改名 breaking-by-design，无存量消费者，不做迁移 | user_confirmed | accepted |
| D5 | 版本对话框显示层映射 obot→uni（API 字段不动） | evidence_verified | accepted |
| D6 | .obotcatalogs 约定名、obot-sentry、外链、吉祥物资源、后端显示名列为非目标 | evidence_verified（default 分类，低影响） | accepted |
