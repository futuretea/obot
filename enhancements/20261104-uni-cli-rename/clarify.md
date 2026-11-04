# clarify · uni CLI 员工可见层更名

## problem_statement

企业内部按 uni 品牌部署的 obot 平台（fork enhancements/20261019-fork-v0263-uni，界面产品名已改 uni），其 CLI 仍以 `obot` 之名分发给员工：二进制叫 `obot`、命令示例写 `obot login`、隧道弹窗复制出的命令指向员工机器拉不到的上游 Docker 镜像。员工在同一产品里看到两个品牌，且 Docker 标签给出的命令在内网必然失败。

## goals

编号目标供 plan 任务「来源」列回引（G1..G6）。

- **G1 命令与二进制更名**：`make build` 产出 `bin/uni`；`uni --help` 及各子命令帮助/错误文案中不再出现 `obot` 命令引用（如 `run "obot login" first`→`uni login`）。
  - verification_type: acceptance_signal
  - signal_or_metric: 构建产物名 + 帮助文本输出
  - target_or_condition: `bin/uni` 存在；`uni --help`/`uni mcp --help`/`uni tunnel --help` 输出不含 "obot" 字样（代码标识符 import 路径不算）
  - verification_method: 运行 `make build && ./bin/uni --help` 等命令抽查；全量 sweep 由 plan 残留清单任务执行
  - evidence_source: 本地构建与命令输出；Go 测试
- **G2 flag 与员工环境变量更名**：`--obot-base-url`→`--uni-base-url`（字段 `ObotBaseURL`→`UniBaseURL`）；`OBOT_TOKEN`→`UNI_TOKEN`、`OBOT_BASE_URL`→`UNI_BASE_URL`、`OBOT_NON_INTERACTIVE`→`UNI_NON_INTERACTIVE`。
  - verification_type: acceptance_signal
  - signal_or_metric: `uni tunnel --help` 的 flag 名；环境变量读取行为
  - target_or_condition: help 显示 `--uni-base-url`；设置 `UNI_BASE_URL` 后 client BaseURL 生效；旧 `OBOT_*` 不再被读取
  - verification_method: Go 单测（root_test.go 环境变量用例改为 UNI_* 后通过）+ 命令抽查
  - evidence_source: pkg/cli 测试
- **G3 本机存储标识更名**：钥匙串服务名 `obot`→`uni`（credentials.DefaultService）；配置路径 `obot/config.json`→`uni/config.json`（localconfig.configRelPath）。
  - verification_type: acceptance_signal
  - signal_or_metric: setup 后的落盘位置
  - target_or_condition: `uni setup` 写 `~/.config/uni/config.json`；keyring 以服务名 `uni` 存取
  - verification_method: localconfig/root/setup Go 测试断言新路径通过
  - evidence_source: pkg/cli 测试
- **G4 隧道弹窗单一命令**：`MCPTunnelSecretRevealDialog.svelte` 移除 Docker 标签与 ghcr 命令，仅展示 `uni tunnel --uni-base-url <地址>/api --token <密钥>`。
  - verification_type: acceptance_signal
  - signal_or_metric: 组件渲染输出
  - target_or_condition: 弹窗无 Docker 标签、无 ghcr.io 引用；只剩一条 uni 命令
  - verification_method: 新增/更新组件测试断言命令字符串；`pnpm run check` 通过
  - evidence_source: ui/user 测试
- **G5 引导文件更名**：`pkg/localagents/assets/files/skills/` 四个目录 `obot*`→`uni*`，`assets.go` 模板路径清单同步；模板内 frontmatter name 与命令文本改为 uni；安装提示文案（claudecode.go:91、sharedagents.go:55）同步。
  - verification_type: acceptance_signal
  - signal_or_metric: 安装到客户端技能目录的目录名与内容
  - target_or_condition: `uni setup --clients agents` 后 `~/.agents/skills/uni*/SKILL.md` 存在且内容含 `uni` 命令、不含 `obot` 命令
  - verification_method: assets/claudecode/sharedagents/setup Go 测试断言更新后通过
  - evidence_source: pkg/localagents、pkg/cli 测试
- **G6 版本显示映射与其余 UI/文档命令引用**：Profile 版本对话框标签 `obot`→`uni`（显示层映射，API 字段不动）；`curl.ts` 生成的示例、设备码页 `obot login`、五处 inventory/dashboard `obot scan` 空态文案、`TunnelsView` 两处 `obot tunnel` 文案改为 uni；当前版本文档（docs/docs）中 CLI 命令/环境变量/keyring 服务名引用同步（cli-setup、mcp-tunnels、llm-gateway、agent-auth-scopes、mcp-server-gitops）。
  - verification_type: acceptance_signal
  - signal_or_metric: UI 渲染文本与文档内容
  - target_or_condition: 上述位置不再出现 obot 命令/变量名（API 字段名与外链除外）
  - verification_method: UI spec 抽查 + 文档 sweep（由 plan 残留清单任务全量执行）
  - evidence_source: ui/user 测试、docs 内容

## non_goals

1. 服务端环境变量 `OBOT_SERVER_*`、部署配置（Dockerfile 内 POSTGRES_* 等）。
2. API 路径与字段：`/api/*`、`version.current.obot`、`remoteConfig.tunnelName`、`Meta.Obot`、`allowAllObotHostedMcpServers`、license entitlement 值。
3. Go module 路径 `github.com/obot-platform/obot` 与内部代码标识符（除影响 flag 生成的 `ObotBaseURL` 字段外）。
4. catalog 目录约定名 `.obotcatalogs`/`.ignoreobotcatalogs` 与 `.obot-catalog-*` 临时前缀（gitops 数据契约）。
5. `obot-sentry` 独立二进制、外部链接（docs.obot.ai 等）、吉祥物资源/CSS 类、后端生成的 "Obot MCP Server" 显示名。
6. Docker 镜像分发方案（内网 registry）与 macOS 签名公证（运营决策，另行处理）。
7. docs/versioned_docs/**（历史版快照）与 packaging/releases/**（已打包快照）。
8. 旧名兼容别名、迁移或清理逻辑（breaking-by-design，无存量消费者）。

## success_criteria

1. `make build` 产出 `bin/uni`；`make test` 通过；`ui/user` 的 `pnpm run check`（type-check）通过。
2. Docker 镜像构建链不断：Dockerfile:52 COPY 与 run.sh 入口同步改名后 `docker build` 语义不变（以文件一致性审查 + 既有 CI 为准）。
3. 员工路径端到端：下载 uni → `uni setup --url <内网地址>` → 浏览器登录 → keyring/config 落在 uni 名下 → `uni mcp search --json` 可用。
4. 隧道管理员路径：页面创建隧道后复制到的一条 `uni tunnel` 命令可直接运行。

## decision_tree

- 改名遇到「是品牌文案还是契约」→ 员工在终端/页面/本机文件看到的字符串与名字：改；跨进程协议字段、服务端配置、数据约定：不改（D3/D6）。
- 改名遇到「改名会不会破坏什么」→ 逐项对照影响面清单（closure.md）；机械耦合（Dockerfile/run.sh/release.yml 产物前缀/goreleaser project_name）必须同步；外部渠道目标（brew tap repo、winget 包 ID、ghcr 镜像名）保持原值。
- 测试断言 → 凡断言旧名的用例同步改新名；不断言品牌名的用例不动。

## design_interview_summary

7 条关键决策全部闭合（design-interview.md）：范围 D1、弹窗 D2、服务端边界 D3、兼容策略 D4（breaking-by-design）、版本显示 D5（修正了 "Obot v" 前缀的错误前提，改为对话框标签映射）、引导文件 D6'（纳入）、非目标分类 D6。无 blocking question。

## confirmed_answers

见 design-interview.md「决策摘要」表（D1-D6）。
