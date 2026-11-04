# uni CLI 员工可见层更名 实现方案

## 产物状态

| 字段 | 值 |
|------|----|
| 产物状态 | `delivered` |
| 审查门禁 | `pass` |
| Skill 标注 | `complete` |
| 最新审查证据 | `.auto-runs/v2/auto-plan/20261104-auto-plan-uni-cli-rename/results.tsv` |
| 阻塞项 | `none` |

## 概述

本轮把员工可见层的 `obot` 全部换成 `uni`：CLI 本体（命令文案、flag、员工环境变量、钥匙串服务名、配置路径）、引导资产、构建链产物名、隧道弹窗（去 Docker 标签只留一条命令）、版本对话框标签映射、其余 UI 文案与当前版本文档。服务端契约（`OBOT_SERVER_*`、API 字段、module 路径）不动。从 T1 开始，完成后先跑 `go test ./pkg/cli/... ./pkg/localagents/...`，最终以 T7 的五个 sweep 面（SW1/SW1b/SW2/SW3/SW4）清零 + `make build` 产出 `bin/uni` 收口。上游输入：同目录 `proposal.md`（review-passed）；残留清单：`.auto-runs/v2/auto-plan/20261104-auto-plan-uni-cli-rename/residual-inventory.md`（removal 切片逐族穷举，正文「残留处置镜像」表为对账入口）。

---

## 实现入口

| 项 | 内容 |
|----|------|
| 当前切片 | 员工可见层 uni 化的唯一更名切片（G1-G6） |
| 允许修改范围 | `pkg/cli/**`、`pkg/localagents/**`、`ui/user/src/**`、`Makefile`、`.goreleaser.yaml`、`Dockerfile`、`run.sh`、`.github/workflows/release.yml`、`docs/docs/**`（当前版本文档） |
| 禁止触碰范围 | `OBOT_SERVER_*` 与 Dockerfile `POSTGRES_*`；API 路径/字段（`/api/*`、`version.current.obot`、`Meta.Obot`、`remoteConfig.tunnelName`、entitlement 值）；Go module 路径与内部标识符（唯一例外 `ObotBaseURL`→`UniBaseURL`）；`.obotcatalogs`/`.ignoreobotcatalogs`/`.obot-catalog-*`；`obot-sentry`、外链、吉祥物资源/CSS 类、后端显示名；`docs/versioned_docs/**`、`packaging/releases/**`、`third-party-projects/**`、`chart/**`、`apiclient/**`；DEVELOPMENT.md/CLAUDE.md（deferred，用户未裁决） |
| 入口任务 | T1 |
| 首个验证命令 | `go test ./pkg/cli/... ./pkg/localagents/...` |
| 阻塞条件 | sweep 排除清单出现无法归类豁免的命中；`pnpm run check` 或 `make test` 出现与更名无关的红；`goreleaser check` 失败 |
| 最小实现约束 | 零新增依赖/公共 API/配置项/抽象层；不做旧名别名、迁移或清理代码（D4）；版本对话框用局部显示映射，不引入品牌参数化机制 |

---

## 系统架构图

本切片为纯更名，模块边界与依赖方向不变，不画分层架构图（依据：write-plan 流程 1「单模块/小范围/无边界变化可用替代表达」）。更名触点图：

```text
pkg/cli（命令/flag/env/钥匙串/配置路径/文案）      构建链（Makefile→goreleaser→Dockerfile/run.sh→release.yml）
pkg/localagents/assets（4 个 uni* 资产目录+模板）   ui/user（弹窗单命令、版本标签映射、文案）   docs/docs（5 文件命令引用）
        └──────────────────── 全部只改「名字字符串」，不改控制流 ────────────────────┘
```

## 目录结构

无新增目录；改动都在既有目录内：

```text
pkg/cli/                       # 命令文案、usage/env tag、ObotBaseURL→UniBaseURL
pkg/cli/internal/credentials/  # DefaultService "obot"→"uni"
pkg/cli/internal/localconfig/  # configRelPath "obot/config.json"→"uni/config.json"
pkg/cli/internal/token.go      # TokenEnvVar "OBOT_TOKEN"→"UNI_TOKEN" 及提示文案
pkg/localagents/assets/files/skills/  # obot* 四目录→uni*，模板内容同步；assets.go 路径清单同步
ui/user/src/lib/components/admin/MCPTunnelSecretRevealDialog.svelte  # 去 Docker 标签
ui/user/src/lib/components/navbar/Profile.svelte                      # 版本对话框标签映射
docs/docs/{installation,functionality,configuration}/                # 5 文件命令引用
```

## 模块职责

- **CLI 本体**（pkg/cli）：输入=用户命令与环境；输出=平台 API 调用、本机 keyring/config 读写、终端文案。本轮只改名字符串与 `UniBaseURL` 字段名；无逻辑分支变化。
- **引导资产**（pkg/localagents）：输入=embed 的 SKILL.md.tmpl；输出=安装到 `~/.agents/skills`、`~/.claude/skills` 的技能目录。目录名与模板文本更名；`assets.go:16-21` 硬编码路径清单同步。
- **构建链**：产物名 `obot`→`uni`（Makefile `-o`、goreleaser `project_name`/brews `name`/`bin.install`/`description`、Dockerfile COPY、run.sh 入口、release.yml 产物前缀）；外部渠道目标（tap 仓库、winget ID、ghcr FROM）不动。
- **UI**：弹窗收敛为单条 `uni tunnel` 命令；Profile 版本对话框 key→label 显示映射（`obot`→`uni`）；curl 示例、device-code、五处 `obot scan` 空态、TunnelsView 两处文案。

扩展点：`not-applicable`（依据：纯更名切片，无新稳定接口）。

## 残留处置镜像

完整逐族清单见 residual-inventory.md（8 族、扫描口径、命中数、处置）；对账义务落在 T7。摘要：

| 族 | 处置 |
|---|---|
| 代码标识符 | 文案/env/usage 换 uni；内部标识符保留（唯一例外 `ObotBaseURL`） |
| 指标观测 / 角色枚举 | not-applicable（无触及） |
| 用户可见文案（UI） | 12 处换 uni + 1 处标签映射；API 字段访问豁免 |
| 配置与部署 | 员工 env/flag/产物名耦合同步；`OBOT_SERVER_*`/`POSTGRES_*`/外部注册名豁免 |
| API 契约 | 保留（D3/D5） |
| 测试资产 | Go 断言同步；弹窗补 spec；dashboard spec 豁免 |
| 文档与示例 | docs/docs 5 文件换 uni；DEVELOPMENT.md/CLAUDE.md deferred |

---

## 测试策略

| 任务 | 行为类型 | TDD 结论 | 首个失败测试 / 最小验证场景 | 最小可运行检查 | 验证层次 | 进程外依赖 | 运行路由 |
|------|----------|----------|------------------------------|----------------|----------|------------|----------|
| T1 CLI 更名 | 机械重命名（含 CLI 表面行为变化） | tdd-not-applicable：机械改名无新逻辑分支；替代验证=既有测试断言同步改 uni 后全绿 + S2/S4/S8 场景 | Red 基线：改动前 `go test ./pkg/cli/...` 全绿（锁定现状） | `go test ./pkg/cli/... ./pkg/localagents/...` | unit | not-applicable（keyring 在测试中用 fake store，既有模式） | `go test ./pkg/cli/...` |
| T2 引导资产 | 机械重命名 | tdd-not-applicable：同上；断言在 assets/claudecode/sharedagents/setup 测试 | Red 基线：同上 | 同上 | unit | not-applicable | `go test ./pkg/localagents/... ./pkg/cli/...` |
| T3 构建链 | 配置改名 | tdd-not-applicable：配置文本；替代验证=`goreleaser check` + 产物名断言 | `goreleaser check` 通过；`make build && test -f bin/uni` | `goreleaser check && make build && ls bin/uni` | manual | not-applicable | shell |
| T4 隧道弹窗 | 行为变更（UI 结构收敛） | tdd-required | 新 spec `MCPTunnelSecretRevealDialog.svelte.spec.ts`：断言渲染仅含 `uni tunnel --uni-base-url` 且无 Docker 标签/`ghcr.io`——对现状（默认 Docker 标签）Red | `pnpm run test -- MCPTunnelSecretRevealDialog` | integration | 单元 mock（组件测试，mock tunnel/version stores，既有模式见 mcpTunnel.test.mjs） | `pnpm run test` |
| T5 版本标签映射 | 行为变更（显示映射新增） | tdd-required | 新 spec 断言版本对话框中 `obot` 字段行标签显示 `uni`、值与链接不变——对现状（渲染 key 原文）Red | `pnpm run test -- Profile` | integration | 单元 mock（mock version store） | `pnpm run test` |
| T6 UI 文案+docs | 机械重命名 | tdd-not-applicable：文案替换；替代验证=`pnpm run check` + 现有 spec 全绿 + T7 sweep | `pnpm run check` | `pnpm run check` | unit | not-applicable | `pnpm run check` |
| T7 sweep+端到端 | 验收 | tdd-not-applicable：验证任务 | 五个 sweep 面（SW1/SW1b/SW2/SW3/SW4）清零 + S 场景对账 | 见任务表验证列 | manual+unit | not-applicable | shell + go/pnpm |

### 行为场景

- **S1**（来源 G3；验证层次 integration；路由 `go test ./pkg/cli/...`，包级运行——config/keyring 断言分布在 localconfig/setup 测试内，无统一 `-run` 过滤名）
  - Given 本机用户目录为测试隔离 HOME，无任何 uni/obot 配置与凭据
  - When `uni setup` 完成配置写入（Go 测试内以既有 fake home 模式执行）
  - Then 配置文件位于 `~/.config/uni/config.json`，钥匙串以服务名 `uni` 存取；`obot` 旧名位置无任何新写入（负例断言由 T1 在 localconfig/setup 测试中新增）
- **S2**（来源 G2；验证层次 manual；路由 `make build && ./bin/uni tunnel --help`——tunnel 无既有测试函数，不以 `-run` 幻名路由；flag 注册行为由包级 `go test ./pkg/cli/...` 回归）
  - Given `uni tunnel` 命令已注册
  - When 执行 `uni tunnel --help`
  - Then 输出包含 `--uni-base-url` 与 `UNI_BASE_URL`，不含 `obot` 字样
- **S3**（来源 G4；验证层次 integration；路由 `pnpm run test -- MCPTunnelSecretRevealDialog`）
  - Given 隧道 secret 已创建，弹窗打开
  - When 渲染完成
  - Then 只展示一条命令 `uni tunnel --uni-base-url <当前地址>/api --token <secret>`；无 Docker 标签、无 `ghcr.io`
- **S4**（来源 G1；验证层次 unit；路由 `go test ./pkg/cli/ -run TestMCPSearchRegistryAuthErrors`，真实测试位于 mcp_test.go:163，:170 断言登录提示文案）
  - Given 未登录且无缓存 token
  - When 执行 `uni mcp search --json`
  - Then 报错信息引导 `run "uni login" first`
- **S5**（来源 G5；验证层次 integration；路由 `go test ./pkg/localagents/... ./pkg/cli/...`，包级运行）
  - Given 客户端技能目录为测试隔离目录
  - When `uni setup` 安装引导资产
  - Then 出现 `uni`、`uni-search-skills`、`uni-search-mcp-servers`、`uni-install-skill` 四个目录；SKILL.md 内命令均为 `uni ...`，无 `obot` 命令
- **S6**（来源 G6；验证层次 integration；路由 `pnpm run test -- Profile`）
  - Given 平台 `/api/version` 返回含 `obot` 字段
  - When 打开版本信息对话框
  - Then 该行标签显示 `uni`，值与外链不变
- **S7**（来源 G1；验证层次 manual；路由 `make build && ./bin/uni --help && ./bin/uni mcp --help && ./bin/uni tunnel --help`）
  - Given 全部任务完成
  - When 构建 `make build` 并运行帮助命令
  - Then 产物为 `bin/uni`；三级帮助输出无 `obot` 命令引用
- **S8**（来源 G2/D4；验证层次 manual；路由 `./bin/uni tunnel --obot-base-url http://x 2>&1`）
  - Given 旧 flag 名 `--obot-base-url`
  - When 以旧名调用
  - Then 报 unknown flag 错误（fail-fast，无别名兼容）

### 外部环境数据依赖承接

- **内网下载服务器**（verification_dependency）：`non_goal`——端到端分发验收以本地构建产物代行（S7 用 `make build` 的 `bin/uni`）；服务器就绪后由分发运营侧验收，不阻塞本切片。
- **Apple 签名/公证密钥**（release_gate，仅 mac 产物）：`non_goal`——mac 包本轮不分发（密钥在上游 Actions，本仓库无）；linux/windows 产物与全部测试不受影响；重启条件见 `.project/pending/20261104-uni-cli-rename.md` 第 3 项。

### 不变量表

| ID | 不变量 | 来源 | 外部语义 / 注入故障 | 维护位置 | 故障注入测试 |
|----|--------|------|---------------------|----------|--------------|
| I1 | 更名后 `obot` 旧名位置（keyring 服务 `obot`、`~/.config/obot/`、`~/.agents|~/.claude/skills/obot*`）永不被 uni CLI 写入 | G3/G5/D4 | 无适用故障：本机 keyring/config/技能目录均为单进程单写者存储，无并发写者、无外部读源、无删除/放行判定 | pkg/cli/internal/credentials、localconfig、pkg/localagents | T1/T2 更新断言时新增负例：obot 旧路径/服务名不被写入（随 S1/S5 包级运行验证） |

---

## 任务规划

| ID | 任务 | 来源 | 范围 | 非目标 | 前置依赖 | 现成能力 | 复杂度边界 | Guard Policy | TDD | 验证方式 | 升级触发条件 | Skill 路由 |
|----|------|------|------|--------|----------|----------|--------------|--------------|-----|----------|----------------|------------|
| T1 | CLI 本体更名+测试同步：closure.md impact_surface 表「pkg/cli 面向用户的 obot 字符串」与「credentials/localconfig 常量引用点」两行的全部「要改」命中（root/login/logout/skills/mcp/setup/setup_status/tunnel/mcp_generate 文案与 env tag；token.go TokenEnvVar=UNI_TOKEN；credentials DefaultService=uni；localconfig configRelPath=uni/config.json；tunnel.go ObotBaseURL→UniBaseURL + env UNI_BASE_URL；usage 内 Obot→uni）；测试断言按 closure「测试断言面」行同步，并按 I1 新增旧名负例断言 | G1,G2,G3 | pkg/cli/** | 不改 OBOT_SERVER_*、API 字段访问、内部标识符、注释不动的文件 | None | 既有测试套件与 fake keyring/home 模式 | 不新增依赖/抽象；不加旧名别名 | cli-surface-rename（fail-fast）：场景 S2/S4/S8 验证；关闭=sweep 面 SW1 清零+测试绿 | not-applicable（机械重命名；替代验证如测试策略表） | `go test ./pkg/cli/...`；`./bin/uni tunnel --help` 抽查场景 S2/S8（该抽查在 T3 完成后执行——T3 前 `make build` 仍产 `bin/obot`；并行期可临时 `go build -o bin/uni .` 代行） | None | none（不路由 $[review-security]：本任务仅改凭据常量名与服务名，不触及鉴权逻辑、存储语义或信任边界；凭据行为由 S1 场景与包级测试验证） |
| T2 | 引导资产更名+测试同步：assets/files/skills 四目录改 uni*，模板 frontmatter name 与命令文本改 uni，assets.go:16-21 路径清单同步，claudecode.go:91/sharedagents.go:55 安装提示文案，assets/claudecode/sharedagents/setup 测试断言同步 | G5 | pkg/localagents/** | 不做旧 obot* 目录清理逻辑（无存量，控制圈） | None | go:embed 通配 + 既有渲染/安装测试 | 不新增清理机制 | not-applicable | not-applicable（机械重命名） | `go test ./pkg/localagents/... ./pkg/cli/...`（S5） | None | none |
| T3 | 构建链耦合同步：Makefile:37 `-o bin/uni`；.goreleaser.yaml project_name=uni + brews `name`/`bin.install`/`description`（:59-62，"Obot CLI"→"uni CLI"）；Dockerfile:52 COPY bin/uni；run.sh:5,37 入口 uni；release.yml:84 产物前缀 uni_ | G1 | Makefile,.goreleaser.yaml,Dockerfile,run.sh,.github/workflows/release.yml | 不动 tap 仓库/winget 包 ID/ghcr FROM/release.github 目标/POSTGRES_*/OBOT_SERVER_* | None | goreleaser check / make build | 不新增发布渠道配置 | not-applicable | not-applicable（配置改名） | `goreleaser check && make build && test -f bin/uni` | None | none |
| T4 | 隧道弹窗单命令：删 Docker 标签/ghcr 命令/dockerTagForVersion 逻辑与 Container 图标，CLI 命令改 `uni tunnel --uni-base-url`；先写 spec（Red）再改组件（Green） | G4 | MCPTunnelSecretRevealDialog.svelte + 新 spec | 不换内网镜像源；不动 TunnelsView 列表页逻辑 | None | 既有 CopyField/ResponsiveDialog；组件测试基础设施（vitest-browser-svelte） | 不新增组件/依赖 | not-applicable | tdd-required（S3） | `pnpm run test -- MCPTunnelSecretRevealDialog`（Red→Green） | None | $[tdd]、$[frontend-testing] |
| T5 | 版本对话框标签映射：Profile.svelte 对话框 key→label 局部映射（obot→uni），值与链接不变；先写 spec（Red）再改（Green） | G6 | Profile.svelte + 新 spec | 不改 API 字段/repoMap key/外链 | None | 组件测试基础设施 | 不引入 i18n/品牌参数化 | not-applicable | tdd-required（S6） | `pnpm run test -- Profile`（Red→Green） | None | $[tdd]、$[frontend-testing] |
| T6 | 其余 UI 文案+docs 同步：curl 示例生成器 `ui/user/src/lib/services/llm-gateway/curl.ts`（uni login、UNI_API_KEY/UNI_BASE_URL）、device-code 页、Devices/DeviceClients/DeviceMcpServers/DeviceSkills/OverviewView 五处 scan 空态、TunnelsView:79,93；docs/docs 五文件命令/env/keyring 服务名引用，mcp-tunnels.md Docker 段随 D2 调整为 CLI-only | G6 | ui/user/src/**（上述文件）、docs/docs/** | 不改 API 字段访问/服务端参数/外链/versioned_docs；DEVELOPMENT.md/CLAUDE.md deferred 不动 | None | 既有文案测试（如有）+ pnpm check | 不新增测试框架 | not-applicable | not-applicable（机械重命名） | `pnpm run check && pnpm run test` | None | none |
| T7 | 全量 sweep 验收+端到端验证：五个 sweep 面——SW1 pkg/cli（proposal 生成规则）、SW1b pkg/localagents/** 含模板 prose（residual-inventory 代码标识符族+i18n 族口径，模板内 "Use the Obot CLI" 类品牌句属于命中）、SW2 ui/user/src、SW3 docs/docs（口径=命令/env/keyring 服务名引用；大小写不敏感 `obot` 全量命中后按下述分类处置：改（T6 五文件命令/env/keyring 引用）或豁免（T7 豁免表））、SW4 构建链——逐面清零；排除清单逐类豁免对账（对账 residual-inventory.md 与下方 T7 豁免表）；`make test`、`pnpm run check && pnpm run test`、场景 S7/S8 手验；产出 sweep 报告 | G1-G6 | 全部允许范围 | 不修 sweep 之外的新发现（上报）；不改 docs 品牌词 prose 与示例占位文本（见豁免表） | T1,T2,T3,T4,T5,T6 | rg 扫描 + 既有构建/测试命令 | 不为 sweep 新增工具 | cli-surface-rename 关闭条件：sweep 清零+全绿 | not-applicable（验证任务） | sweeps + `make test` + `make build`（S7）+ `./bin/uni tunnel --obot-base-url …`（S8）；sweep 报告落 `.auto-runs/v2/auto-ready` 对应 run 或 enhancement 目录 | None | none |

T7 豁免表（sweep 命中先对账此表，可归类即豁免，不改不报）：

| 命中 | 豁免依据 |
|---|---|
| **docs 品牌词/产品名 prose 与示例占位文本**：`Obot Server/Agent/Platform/gateway/version/MCP` 等品牌词句子（约 40+ 文件，如 concepts/architecture.md、installation/overview.md）、`https://obot.example.com` 占位 URL、`OBOT_API_KEY` 示例 shell 变量与 Authorization 示例头 | 员工可见层裁决只覆盖命令/env/keyring 引用（G6/D1 范围）；品牌词 prose 属产品命名与文档叙述，改写是内容重写非本切片；示例占位 URL/变量名非 CLI 读取的环境变量（closure advisory 2）。改写它们将扩大 40+ 文件补丁面并与 Q3（fork 补丁成本）冲突 |
| **docs 服务端 env 文档行、真实外链（docs.obot.ai/charts.obot.ai）与 obot-sentry、`.obotcatalogs`/`.ignoreobotcatalogs` 约定名、Helm chart/namespace 示例资源名** | 服务端配置（D3）、obot-sentry 与数据约定名（D6）、chart 名（非目标）——proposal S3 排除清单条款 |
| **ui/user/src 其余 obot 命中**：子串误报（scrollToBottom/robot account）、内部标识符（obotURL、obotGroup*）、API 契约值与服务端参数（obotSentryVersion、OBOT_AUTH_*、entitlement）、localStorage key、服务端 IAM 示例 ARN、外链/邮箱、吉祥物体系（guides/Obot*、obot-icon 资源）、服务端 payload mock、纯注释 | proposal S2 排除清单逐类；逐文件分类见 sweep-dry-run 报告（run 目录） |
| Makefile:34/97 注释、Makefile:63-71 telepresence `obot-upstream` 开发资源、`tools/obot-proxy.yaml` | 开发工具与注释，非构建产物名耦合（residual 配置族已分类） |
| Makefile:32、.goreleaser.yaml:19/40 ldflags 内 Go module 路径 `github.com/obot-platform/obot` | Go module 路径（D3 禁止范围） |
| `.goreleaser.yaml` release.github / brews.repository / brews `homepage`（:63）、release.yml:85 winget 包 ID、Dockerfile FROM ghcr 镜像 | 外部渠道/注册名/外链，内部不经此发布（D3/非目标） |
| `third-party-projects/**` `obot-providers`、`bin/obot-provider`、Dockerfile:46-48 与 run.sh:21 的 `/obot-providers` 构建路径 | 范围外独立二进制与其构建路径 |
| run.sh/Dockerfile `OBOT_SERVER_*`、`POSTGRES_*` | 服务端配置（D3） |
| `docs/versioned_docs/**`、`packaging/releases/**` | 历史快照（非目标 7） |
| DEVELOPMENT.md:112、CLAUDE.md:31 `bin/obot` | P2-1 deferred，用户未裁决，本切片不动 |

---

## 后续增强

- `uni scan` 命令不存在但 UI 空态引用（上游陈旧文案）：补命令或删文案，独立切片。
- 隧道内容器化（内网镜像源）；macOS 签名分发（待决清单第 3 项）。
- goreleaser 外部渠道段（brew/winget/GitHub release）清理；DEVELOPMENT.md/CLAUDE.md `bin/obot` 引用更新（P2-1 deferred，待用户裁决）。

---

## 风险、假设与回退

| ID | 类型 | 触发条件 | 影响范围 | 检测方式 | 回退 / 降级动作 | 关联任务 |
|----|------|----------|----------|----------|-----------------|----------|
| R1 | 假设 | 上游 rebase 时弹窗/资产模板/CLI 文案三处补丁冲突 | 维护成本 | git rebase 冲突标记 | 重放补丁（接受为 fork 固有成本，Q3） | T1,T4,T6 |
| R2 | 风险 | UI sweep 误把 API 字段访问当文案改掉 | UI 编译/运行 | `pnpm run check` + `pnpm run test` | revert 单文件；对照 proposal S2 排除清单与 sweep-dry-run 报告 SW2 分组 | T6,T7 |
| R3 | 风险 | 构建链改名后镜像构建断（Dockerfile/run.sh 漏改） | 容器部署 | T3 验证命令 + `docker build`（如本地可用） | 补齐耦合点；residual-inventory 配置族对账 | T3,T7 |
| R4 | 风险 | release.yml 前缀改 uni_ 后 winget job 若被误触发会失败 | CI（内部不使用该渠道） | workflow 日志 | 非目标渠道，登记不修；必要时删除 winget job（后续增强） | T3 |
| R5 | 假设 | 无未上报的存量 obot CLI 安装（用户确认无已登录员工） | 若存在，旧 keyring/config 对新 CLI 不可见 | 用户侧反馈 | 受影响者重跑 `uni setup`（影响=一次重配置） | T1 |

必要保护：无新增信任边界/数据丢失面（更名不改鉴权与存储语义，guard cli-surface-rename 按 S2/S8 验证）。

---

## 实现指引

### 关键技术决策

1. **flag 改名走字段改名**：`ObotBaseURL`→`UniBaseURL` 后 `--uni-base-url` 由 cmd 库自动生成；不加别名（D4，S8 负例固化 fail-fast）。核实命令：`./bin/uni tunnel --help`。
2. **弹窗直接删 Docker 分支**：删除 dockerTagForVersion、Container 图标导入与标签切换状态，`commandTab` 状态整体移除，仅保留 CLI 命令；选择「删代码」而非「隐藏标签」，因为保留死分支会让 sweep 与后续维护双重负担。
3. **版本对话框映射用最小局部表**：在 Profile.svelte 对话框 each 处加 key→label 映射（仅 `obot`→`uni` 一项）；不改 API、不抽公共 helper（单点使用）。
4. **docs 的 mcp-tunnels.md Docker 段**：随 D2 改为仅 CLI 命令，删除 docker run 示例块。

### 注意事项

- 测试断言同步清单以 closure.md「测试断言面」节为唯一入口，逐行改，不整文件盲替换。
- `assets.go` 的 embed 是通配（`files/skills/*/SKILL.md.tmpl`），改目录名不破坏 embed，但 `fileSkillTemplates` 硬编码清单必须同步（assets_test.go 断言会兜底）。
- 内部标识符（`type Obot`、`AppURLForAPIBaseURL`、注释中的 Obot）保留；仅当所在文件因其它改动被触碰时，同行注释里的品牌词可顺手改为 uni，不单独开改动。
- sweep 排除清单（proposal S1-S4 定义 + residual-inventory 豁免列）是 T7 对账的唯一依据；命中无法归类豁免时停手上报，不自行扩范围。
