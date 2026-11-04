# uni CLI 员工可见层更名

## 产物状态

| 字段 | 值 |
|------|----|
| 产物状态 | `delivered` |
| 计划准备度 | `pass` |
| 最新审查证据 | `.auto-runs/v2/auto-proposal/20261104-auto-proposal-uni-cli-rename/results.tsv` |
| 阻塞项 | `none` |
| 创建日期 | 2026-11-04 |

## 摘要

企业内部按 uni 品牌部署的平台（fork 基线 `enhancements/20261019-fork-v0263-uni`，界面已改名）分发给员工的 CLI 仍叫 `obot`，员工在同一产品里看到两个品牌；隧道弹窗默认展示的 Docker 命令引用上游 `ghcr.io/obot-platform/obot` 镜像，在内网必然拉取失败。本轮把员工可见层全部更名为 `uni`——二进制与命令名、flag、员工环境变量、钥匙串服务名、本机配置路径、安装到本机 AI 客户端的引导文件、UI 与当前版本文档中的命令引用——并把隧道弹窗收敛为一条 `uni tunnel` 命令。服务端环境变量、API 路径与字段、Go module 路径不动。当前无已登录员工、CLI 尚未分发，直接切换，不做旧名兼容。

**关联 Issue**: 无（内部 fork 需求，来源于 2026-11-04 会话对齐）

---

## 动机

### 目标

1. **员工拿到的程序和敲的命令都叫 `uni`**：`make build` 产出 `bin/uni`；`uni` 各级帮助与错误文案不再引用 `obot` 命令（如未登录提示变为 `run "uni login" first`）。验证：构建产物名 + 帮助输出抽查 + 全量残留 sweep（见实现概览封闭规则）；证据来自本地构建与 Go 测试。
2. **flag 与员工环境变量随品牌更名**：`--obot-base-url` 变为 `--uni-base-url`；`OBOT_TOKEN`/`OBOT_BASE_URL`/`OBOT_NON_INTERACTIVE` 变为 `UNI_TOKEN`/`UNI_BASE_URL`/`UNI_NON_INTERACTIVE`，旧名不再被读取。验证：`uni tunnel --help` 输出新 flag 名；环境变量用例（root_test.go 等）改用新名后通过。
3. **本机凭据与配置落在 uni 名下**：钥匙串服务名 `obot`→`uni`，配置路径 `~/.config/obot/config.json`→`~/.config/uni/config.json`。验证：setup 后落盘位置断言（localconfig/root/setup 测试）。
4. **隧道弹窗只给一条能用的命令**：`MCPTunnelSecretRevealDialog` 移除 Docker 标签与 ghcr 命令，仅展示 `uni tunnel --uni-base-url <当前地址>/api --token <密钥>`。验证：组件测试断言渲染输出（当前该组件无测试，本轮补）。
5. **引导文件装出来就是 uni**：`uni setup` 安装的四个技能目录为 `uni`、`uni-search-skills`、`uni-search-mcp-servers`、`uni-install-skill`，模板内命令均为 `uni ...`。验证：assets/claudecode/sharedagents/setup 测试断言。
6. **页面与文档不再教员工敲 obot**：Profile 版本对话框标签、curl 示例、设备码页、五处 `obot scan` 空态文案、TunnelsView 两处文案、docs/docs 五个文件的命令引用同步为 uni。验证：UI spec 抽查 + 文档 sweep 清零。

### 非目标

1. 服务端环境变量 `OBOT_SERVER_*` 与部署配置（含 Dockerfile 内 `POSTGRES_*`）。
2. API 路径与字段（`/api/*`、`version.current.obot`、`remoteConfig.tunnelName`、`Meta.Obot`、entitlement 值）。
3. Go module 路径 `github.com/obot-platform/obot` 与内部代码标识符（唯一例外：`tunnel.go` 的 `ObotBaseURL` 字段改名以生成 `--uni-base-url`）。
4. catalog 目录约定名 `.obotcatalogs`/`.ignoreobotcatalogs` 与 `.obot-catalog-*` 临时前缀（gitops 数据契约）。
5. `obot-sentry` 独立二进制、外部链接（docs.obot.ai 等）、吉祥物资源与 CSS 类、后端生成的 "Obot MCP Server" 显示名。
6. Docker 镜像内网源替换、macOS 签名公证、内网下载服务器建设（分发运营，另行处理）。
7. `docs/versioned_docs/**` 与 `packaging/releases/**` 历史快照。
8. 旧名别名、迁移或清理逻辑（breaking-by-design，无存量消费者）。

### 当前切片

- **本轮交付**: 员工可见层 100% uni 化的一个更名切片（CLI 本体 + 本机存储标识 + 引导资产 + UI 命令文案 + 当前版本文档 + 构建链产物名耦合）。
- **验收信号**: `bin/uni` 构建成功；`make test` 与 `pnpm run check` 通过；`uni setup --url <地址>` 端到端走通（keyring/config 落 uni 名下）；隧道弹窗复制出的命令可直接运行。
- **是否需要新增代码**: 是——均为既有代码的字符串/标识符修改、弹窗删减、版本对话框显示层标签映射（局部代码），以及补齐隧道弹窗组件测试。
- **现成能力检查**: 不新增依赖、配置项或抽象层；显示映射用 Profile.svelte 局部映射表实现（不引入 i18n/品牌参数化机制——上游无此机制，自建越出切片）。
- **不得混入**: 服务端改名、Docker 镜像源、mac 签名、品牌参数化重构、`uni scan` 命令实现。
- **升级触发条件**: none（纯更名，无容量/性能边界）。

### 后续增强

- UI 五处空态文案引用的 `scan` 命令上游已删除（#7102），改名后 `uni scan` 仍不存在——上游陈旧文案问题，另立切片决定补命令或删文案。
- 隧道以内容器化运行方案（内网镜像源）与 macOS 签名分发（待决清单 `.project/pending/20261104-uni-cli-rename.md` 第 3 项）。
- goreleaser 外部渠道段（brew/winget/GitHub release）对内网分发无用，可在独立清理切片删除。

---

## 提案

现网 case 来源：2026-11-04 会话对齐与 fork 发布说明——隧道弹窗 Docker 命令在内网拉不到 ghcr 镜像（触发特征：任何内网机器执行 docker pull 失败）；员工侧无存量登录（用户确认），改名无迁移负担。两 case 分别驱动故事 2 与 breaking-by-design 决策。

### 用户故事

**故事 1: 员工安装 CLI**
- **改造前**: 从内网服务器下载的程序叫 `obot`，教程和页面却自称 uni；帮助文本提示 `obot login`。
- **原因**: fork 只改了界面产品名，CLI 员工可见层未动。
- **改造后**: 下载 `uni`，跑 `uni setup --url https://<公司地址>`，浏览器登录后本机凭据存在 uni 名下；帮助与报错全部说 uni。

**故事 2: 管理员配置 MCP 隧道**
- **改造前**: 创建隧道后弹窗默认选中 Docker 标签，复制出的命令引用 `ghcr.io/obot-platform/obot`，内网执行失败。
- **原因**: 上游面向公网分发设计；内部不经这些渠道发布。
- **改造后**: 弹窗只有一条 `uni tunnel --uni-base-url <地址>/api --token <密钥>`，复制即可在能同时访问平台与内网 MCP 的机器上运行。

**故事 3: 本地 AI 客户端引导**
- **改造前**: `obot setup` 装入 `~/.agents/skills/obot*/` 的引导文件教 agent 调 `obot` 命令。
- **改造后**: `uni setup` 装入 `uni*/` 引导文件，agent 调 `uni` 命令；与员工实际安装的二进制一致。

### 详细用户体验

```bash
# 员工：安装后一次性配置（浏览器登录，keyring/config 落 uni 名下）
uni setup --url https://uni.internal.example

# 非交互/脚本
UNI_TOKEN=<key> UNI_BASE_URL=https://uni.internal.example/api uni mcp search --json

# 管理员：页面创建隧道后弹窗给出（唯一一条）
uni tunnel --uni-base-url https://uni.internal.example/api --token <隧道密钥>
```

### API 变更

HTTP API 无变更。变更的是 CLI 表面与本机标识：

| 资源 | 变更类型 | 字段/行为 | 说明 |
|------|----------|-----------|------|
| CLI flag | 重命名 | `--obot-base-url` → `--uni-base-url` | 由 `Tunnel.ObotBaseURL` 字段改名 `UniBaseURL` 自动生成；旧 flag 报未知 flag 错误 |
| 环境变量 | 重命名 | `OBOT_TOKEN`→`UNI_TOKEN`、`OBOT_BASE_URL`→`UNI_BASE_URL`、`OBOT_NON_INTERACTIVE`→`UNI_NON_INTERACTIVE` | 旧名不再被读取 |
| 钥匙串服务名 | 重命名 | `obot` → `uni` | credentials.DefaultService |
| 本机配置路径 | 重命名 | `obot/config.json` → `uni/config.json` | localconfig.configRelPath（XDG） |
| 引导资产 | 重命名 | `obot*` 四目录 → `uni*` | frontmatter name 与模板内容同步 |
| UI 显示 | 映射 | 版本对话框标签 `obot`→`uni` | 显示层映射；API 字段 `version.current.obot` 不动 |

---

## 进入计划准备度

当前没有阻塞项，可以进入 plan。访谈与闭合度证据在同目录 `design-interview.md`、`closure.md`、`glossary.md`，以下只留结论。

### 开放问题与假设

| ID | 类型 | 问题 / 假设 | 证据 / 回退方式 | 影响章节 |
|----|------|-------------|----------------|----------|
| Q1 | advisory | 内网下载服务器尚未建立，端到端分发验收暂以本地构建产物代替 | 服务器为分发运营事项；不影响代码切片 | 测试计划 |
| Q2 | advisory | macOS 签名/公证密钥在上游 Actions，本仓库无；mac 包暂不分发 | linux/windows 产物不受影响；待决清单第 3 项 | 升级策略 |
| Q3 | assumption | 上游同步时本切片形成固定补丁面（弹窗、资产模板、CLI 文案三处），每次 rebase 需重放 | 接受为 fork 固有成本；不建品牌参数化机制（上游无此机制）；修正成本=重放补丁 | 实现概览 |
| Q4 | advisory | UI `uni scan` 空态文案引用的命令不存在（上游已删） | 本切片只统一品牌，不实现命令；后续增强跟踪 | 后续增强 |

### 决策记录

| ID | 决策 | 原因 | 来源 |
|----|------|------|------|
| D1 | 员工可见层全量更名（二进制/命令/flag/env/钥匙串/配置/引导/UI/文档命令引用） | 用户「肯定是要全改的」；无存量消费者 | user_confirmed（会话 2026-11-04） |
| D2 | 隧道弹窗去 Docker 标签，仅留 uni 命令 | 用户「我们可以去掉 Docker 标签，只留 uni 命令」；ghcr 内网不可达 | user_confirmed（会话 2026-11-04） |
| D3 | 服务端 `OBOT_SERVER_*`、API 路径/字段、module 路径不改 | 部署配置与协议契约；改动断现网 | user_confirmed（align 轮） |
| D4 | 兼容策略 breaking-by-design，无旧名别名/迁移 | 用户确认「没有已经登录的员工」；CLI 尚未分发 | user_confirmed（会话 2026-11-04） |
| D5 | 版本对话框用显示层映射 `obot`→`uni`，API 字段不动 | 代码事实：无 "Obot v" 前缀（Profile.svelte:347 渲染裸值），对话框把 key 当标签渲染（:384） | evidence_verified（读码复核） |
| D6 | `.obotcatalogs` 约定名、obot-sentry、外链、吉祥物资源、后端显示名列为非目标 | 均非 CLI 品牌文案：数据契约/其他产品/外部站点/不渲染文字的资源 | evidence_verified（影响面扫描） |

`discuss` 裁决跳过（`discussion_required=false`）：范围内只剩一条机械执行路径，关键取舍（范围、弹窗、兼容策略）已在会话对齐中由用户逐项裁决，无 competing viable option。

### 保护策略

| guard_id | category | lifecycle_stage | active_consumers | trust_boundary | contract_surface | failure_policy | guard_level | source | status |
|----------|----------|-----------------|------------------|----------------|------------------|----------------|-------------|--------|--------|
| cli-surface-rename | compatibility | prelaunch（CLI 未分发） | no（用户确认无已登录员工） | 员工本机 CLI ↔ 平台 | 命令名/flag/员工 env/钥匙串服务名/配置路径 | fail-fast（旧 flag 报未知 flag；旧 env 被忽略走默认值） | none（无兼容层，by design） | user_confirmed | accepted |

不新增 validation/fallback/safety/release/migration guard；keyring/config 换新位置后旧位置无数据，无需迁移 guard（D4）。

---

## 设计细节

### 架构

```text
员工机器                                平台（不动）
┌─────────────────────────┐            ┌──────────────────┐
│ bin/uni                 │            │ API /api/* 字段   │
│  ├─ uni setup/login     │ ──HTTP──►  │ version.obot 等   │
│  ├─ keyring service=uni │            │ OBOT_SERVER_*    │
│  ├─ ~/.config/uni/…     │            └──────────────────┘
│  └─ ~/.agents/skills/uni*│
└─────────────────────────┘
   隧道弹窗（页面）只生成：uni tunnel --uni-base-url … --token …
```

### 实现概览

1. **flag 改名走字段改名**：`tunnel.go` 的 `ObotBaseURL`→`UniBaseURL`，usage tag 与 `env:"UNI_BASE_URL"` 随之更新；`--uni-base-url` 由 `obot-platform/cmd` 库从字段名生成，无手写 flag 注册。选择字段改名而非加别名，因为无存量消费者（D4）且别名会引入双名残留。
2. **员工可见字符串全集用封闭规则声明，不逐条内嵌**：本轮声明四个「无 obot 残留」声明面，各自生成规则如下（样例仅作 sweep 起点，非穷举；全量清零是 plan 的 sweep 任务义务）：
   - **S1 pkg/cli**：对 `pkg/cli/**/*.go` 非 import 行做大小写不敏感 `obot` 匹配；排除清单——module 路径、API 字段访问（`Meta.Obot`）、服务端 env（`OBOT_SERVER_*`）、目录约定名（`.obotcatalogs` 等）、纯内部标识符（`type Obot`、函数名、注释不改除非文件本身在改）。样例：`mcp.go:535`、`root.go:64`。
   - **S2 ui/user**：`ui/user/src/**` 的字符串字面量与模板文本 `obot` 匹配；排除——API 字段访问与服务端参数名（`version.current.obot`、`OBOT_AUTH_PROVIDER_*`、entitlement 值）、资源文件名与 CSS 类、外链、纯注释。样例：`curl.ts:4`、`device-code/+page.svelte:93`。
   - **S3 docs/docs**：CLI 命令、三变量、keyring 服务名匹配；排除 versioned_docs、服务端配置、obot-sentry、helm/chart 名。样例：`cli-setup.md:39`。
   - **S4 构建链**：`bin/obot`（Makefile:37）、goreleaser `project_name` 与 archive 模板、goreleaser `brews` 的 `name` 与 `bin.install` 行、`Dockerfile:52` COPY、`run.sh:5,37` 入口、`release.yml:84` 产物前缀；外部渠道目标（brew tap 仓库、winget 包 ID、ghcr FROM）保留原值。
3. **复杂度边界**：不新增依赖、配置项、公共 API 或品牌参数化抽象；不做旧名清理/迁移代码（无存量，控制圈内不为不存在的历史写代码）；`assets.go` 模板路径清单为既有硬编码数组，仅同步改名。
4. **保护策略映射**：cli-surface-rename guard 落到测试（旧 flag/env 不识别的负例可选、新名正例必测）与构建验证（`make build` 产物名）；无 release/rollback gate 需求（内部 fork 无外部发布渠道）。

### 测试计划

1. **Go 单测更新**：影响面清单列出的断言（root_test、setup_test、mcp_test、localconfig/store_test、token_test、assets/claudecode/sharedagents_test）全部改为新名后 `make test` 通过。
   - 预期结果: 全绿；断言新路径 `uni/config.json`、新资产目录 `uni*`、新文案。
   - 最小可运行检查: `make test`（含上述包）。
2. **隧道弹窗组件测试（新增）**：断言渲染含 `uni tunnel --uni-base-url` 与 token 拼接、不含 Docker 标签与 `ghcr.io`。
   - 预期结果: 现网 case（弹窗默认 Docker 命令内网不可用）在断言层固化：只剩一条 CLI 命令。
   - 最小可运行检查: `pnpm run test -- MCPTunnelSecretRevealDialog`（或等价 vitest 过滤）。
3. **构建与类型检查**：`make build` 产出 `bin/uni`；`pnpm run check` 通过。
   - 预期结果: 产物名与类型零错误。
   - 最小可运行检查: `ls bin/uni && file bin/uni`。
4. **端到端手验（本地代行，Q1）**：`./bin/uni setup --url http://localhost:8080 --clients none --non-interactive`（无平台时验证错误路径文案为 uni）、`uni mcp search --json` 未登录提示 `uni login`。
   - 预期结果: 帮助/错误文案无 obot；有平台时 keyring/config 落 uni 名下。
   - 最小可运行检查: `./bin/uni --help; ./bin/uni mcp search --json 2>&1`。
5. **残留 sweep（plan 义务，验收时执行）**：按实现概览 S1-S4 生成规则全量扫描并清零（排除清单内的命中需在 sweep 报告中逐类列出豁免依据）。

### 升级策略

- **兼容边界**: CLI 表面（命令/flag/员工 env/钥匙串服务名/配置路径）。无已上线 public contract 消费者：用户确认无已登录员工、CLI 未分发（D4）；服务端契约不动（D3）。
- **兼容策略状态**: `breaking-by-design`
- **兼容策略来源**: 用户选择（「肯定是要全改的」+「没有已经登录的员工」）。
- **兼容策略**: 直接切换；旧 flag 报未知 flag；旧 env 被忽略后走默认值（localhost:8080/api）；keyring/config 使用全新位置。受影响对象为零存量员工 + 本仓库构建链。
- **升级路径**: 内网服务器只挂 uni 产物；下载页与安装说明同步（运营事项）。
- **回滚方式**: 整切片 `git revert`；新 keyring/config 位置可能已有测试期数据，回滚后手动删除 `uni` 条目/`~/.config/uni/`（无其他外部副作用）。

---

## 备注

### 备选方案

| 方案 | 优点 | 缺点 | 不采用原因 |
|------|------|------|------------|
| 只改 UI 文案，CLI 保留 obot | 改动最小 | 员工仍敲 obot，双品牌持续 | 用户明确「全改」（D1） |
| CLI 加 uni 别名/双名并存 | 渐进切换 | 双名残留、sweep 无法清零、与无存量前提矛盾 | D4 breaking-by-design |
| 品牌参数化机制（BUILD_VAR 注入名字） | 上游同步零补丁 | 引入新机制与概念，上游无此机制 | 越出切片（克制原则）；Q3 登记补丁成本 |
| Docker 标签换内网镜像源 | 保留容器化跑法 | 内网 registry 未定；运营依赖 | 用户选移除（D2） |

### 参考资料

- fork 基线：`enhancements/20261019-fork-v0263-uni/release-note.md`（界面已改 uni、地址与代码标识不变）
- 影响面扫描与 git 历史：本 run Research sidecar 报告（2026-11-04），结论已合入 `closure.md`
- 待决清单：`.project/pending/20261104-uni-cli-rename.md`
