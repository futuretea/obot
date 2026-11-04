# closure · uni CLI 员工可见层更名

## goal_closure

- 交付物清楚：一个更名切片，产出 uni 命名的 CLI、单命令隧道弹窗、uni 引导文件、同步的 UI/文档引用。
- 范围清楚：clarify.md goals G1-G6 + non_goals 1-8。
- 约束清楚：服务端契约与服务端配置不动（D3）；无存量消费者（D4）。
- 成功标准清楚：clarify.md success_criteria 1-4。
- 非目标清楚：non_goals 显式列出。

## measurable_goal_check

G1-G6 均为 `acceptance_signal` 型，五字段（verification_type/signal_or_metric/target_or_condition/verification_method/evidence_source）逐项填写于 clarify.md；无无来源百分比、无绝对可靠性目标。`measurable_goal_contract_status=closed`。

## environment_closure

- 上下文可用性：代码、fork 发布说明（enhancements/20261019-fork-v0263-uni/）、本次 Research 影响面扫描全部可读。
- 可复现性：`make build`/`make test`/`pnpm run check` 本地可跑；无需运行中的平台即可验证绝大多数断言。
- 权限充足性：仓库读写权限已具备（本 run 已在写入 enhancements/）。
- 客观反馈：Go 测试、UI check、构建产物名为客观信号。
- 外部稳定性：改动不依赖外部服务；内网下载服务器与 Apple 签名属分发运营（见外部依赖节，advisory）。

`environment_closure_status=closed`

## impact_surface

切片性质：变更既有代码/文案，无删除行为面；不写入既有读取方消费的新数据面（改名后 keyring/config 路径是全新位置，旧位置无存量数据）。搜索规则与命中分类如下（来源：Research sidecar 2026-11-04 全仓扫描，主 agent 复核关键项）：

| 规则（关键字 × 范围） | 命中 | 分类 |
|---|---|---|
| `obot` × pkg/cli/**（文案/usage/env tag） | root.go:64,67,75；login.go:21,28,33,55；logout.go:12,18；skills.go:54,66,134；mcp.go:374,390,535；setup.go:46,87,139,145,267,327,336,346,349,359,362,365；setup_status.go:58,84；tunnel.go:15,16；mcp_generate_vmcp_catalog.go:41,51-54,89；internal/token.go:28,225 | in-scope |
| flag 字段名 | tunnel.go:16 `ObotBaseURL`（`--obot-base-url` 的生成源，CLI 中唯一 obot 前缀 flag） | in-scope（字段改名） |
| API 字段访问 | mcp.go:456 `Meta.Obot` | out-of-scope（契约） |
| 服务端 env | server.go:25 `OBOT_SERVER_MODEL_PROXY_URL` 及 server*_test.go | out-of-scope（D3） |
| `DefaultService="obot"` × credentials | store.go:10；keyring.go:39,44（仅 2 引用点） | in-scope |
| `configRelPath="obot/config.json"` × localconfig | store.go:17；store.go:77,109（仅 2 引用点） | in-scope |
| obot 命令/品牌 × ui/user/src | MCPTunnelSecretRevealDialog.svelte:40,43,48；curl.ts:4,27,29；Devices.svelte:162；DeviceClients.svelte:131；DeviceMcpServers.svelte:87；DeviceSkills.svelte:112；OverviewView.svelte:186；TunnelsView.svelte:79,93；device-code/+page.svelte:93；Profile.svelte:384（key 标签渲染） | in-scope |
| 代码字段/常量 × ui | version.current.obot 访问、repoMap key 'obot'、OBOT_AUTH_PROVIDER_* 等服务端参数、OBOT_CLOUD 等 entitlement 值、allowAllObotHostedMcpServers | out-of-scope（契约/服务端） |
| 吉祥物/资源/CSS × ui | guides/Obot.svelte、guide-obot__*、obot-icon-*.svg、OBOT_GUIDE_KEYS | out-of-scope（不渲染 Obot 文字） |
| 外链 × ui | docs.obot.ai、obot.ai、github.com/obot-platform | out-of-scope（外部站点） |
| `obot` × pkg/localagents | assets/files/skills/{obot,obot-search-skills,obot-search-mcp-servers,obot-install-skill}/ 全部模板；assets.go:16-21 路径清单；claudecode.go:91；sharedagents.go:55 | in-scope |
| 产物名 × 构建链 | Makefile:37（bin/obot）；.goreleaser.yaml:3（project_name）,45（archive 名模板）,59-68（brews name/bin.install）；Dockerfile:52；run.sh:5,37；.github/workflows/release.yml:84（obot_ 前缀） | in-scope（机械耦合同步） |
| 外部渠道目标 × 构建链 | .goreleaser.yaml release.github/brews.repository；release.yml:85 winget 包 ID；Dockerfile FROM ghcr 镜像 | out-of-scope（外部注册名，内部不分发经此） |
| CLI 命令 × docs/docs | cli-setup.md（title,5,11,14,19,22,27,39,45,63,69,79-80,90,96,101,104,117,122,125）；mcp-tunnels.md（8,15,75-78,82,85-88,92,95-96）；llm-gateway.md（16 处）；agent-auth-scopes.md（206-222）；mcp-server-gitops.md（17 处） | in-scope |
| obot-sentry/服务端 env × docs/docs | device-management.md:56-57；server-configuration.md:55；helm/chart 名 | out-of-scope |
| 目录约定名 | .obotcatalogs/.ignoreobotcatalogs（mcp_convert.go:30 等）、.obot-catalog-*（mcp_convert.go:79） | out-of-scope（D6，gitops 数据契约） |
| 测试断言面 | localconfig/store_test.go:140；root_test.go:22,61,101,117,127-152；setup_test.go:57,65,95,103,127,154,190,407,436,462,490；mcp_test.go:170；internal/token_test.go:689,329；assets_test.go:16-62；claudecode_test.go:52-63；sharedagents_test.go:27-38 | in-scope（同步更新断言） |
| MCPTunnelSecretRevealDialog 测试 | 无 spec 文件（G4 需补组件测试） | in-scope（补测试） |

`needs-policy` 成员：无。

## history_signal

命令：`git log --oneline -n 50 -- pkg/cli`（108 commits 总量）、`-- pkg/localagents/assets`（11）、`-- ui/user/src/lib/components/admin/MCPTunnelSecretRevealDialog.svelte`（2）。
判定：`none`。上游历史上 bootstrap 模板（#6711→#6827）与 CLI token（#6951→#6963）各有一段连续修正，但均为功能迭代/缺陷修复，不是「更名/字符串一致性」机制的反复返工；本切片不改动这些机制。

## 场景闭合

关键链路枚举（写持久化状态、跨进程调用或改变用户可见结果）：

**L1 setup/login 认证与落盘**（keyring+config 写入）
- 并发：不适用（单进程单用户命令；上游现状即如此，本切片只改名字符串）。
- 异常：不适用（错误路径不变，仅文案 obot→uni）。
- 边界：不适用（无输入解析变化）。
- 数据量：not-applicable。
- 下游影响：keyring 服务名/配置路径换成新值；无存量读取方（D4）。旧名位置不存在数据。
- 时序并行：not-applicable（无周期性写入；状态持续期=永久、失效条件=用户 logout/删除，均为既有行为，改名不改变语义）。
- 逆向：logout 按新服务名删除；无旧名孤儿（无存量）。

**L2 隧道命令生成 → 员工复制执行**
- 并发/异常/边界/数据量/时序并行/逆向：not-applicable（弹窗是纯展示字符串；变化仅命令文本与 flag 名，运行时行为由既有 tunnel 实现承载）。
- 下游影响：复制出的命令必须与实际二进制的 flag 名一致（`--uni-base-url` 由 tunnel.go 字段改名同步生成）——G2 与 G4 互为校验。

**L3 引导文件安装**
- 并发/异常/边界/数据量：not-applicable（安装逻辑不变，仅资产名与内容）。
- 下游影响：安装产物目录名变为 uni*；`replaceDir` 只覆盖同名目录，旧 obot* 目录不会被清理——无存量安装（D4），记为已知边界而非新增清理机制（控制圈：不为不存在的历史写迁移代码）。
- 时序并行：not-applicable。逆向：卸载=删目录，无外部副作用。

**L4 版本对话框渲染**
- 全维度 not-applicable（纯显示层 key→label 映射；API 响应不变）。

## external_environment_data_dependencies

| dependency | role | source | coverage | stability | failure_modes | behavior_on_failure | plan_mapping | classification |
|---|---|---|---|---|---|---|---|---|
| 内网下载服务器（分发入口） | verification_dependency（端到端验收 G 路径时） | 用户会话确认「打包的二进制放到内网服务器上提供下载」 | 未覆盖：服务器尚未建立 | 运营侧随时可变 | 服务器未就绪时端到端验收改在本地构建产物上执行 | 不阻塞代码切片 | 明确非目标（分发运营） | advisory |
| Apple 签名/公证密钥 | release_gate（仅 mac 产物分发） | 上游 GitHub Actions secrets 持有；本仓库无 | 仅 linux/windows 产物可本地验证 | 上游所有 | mac 未签名包被系统拦截 | mac 分发延后，不影响本切片代码与测试 | 明确非目标 + 待决清单第 3 项 | advisory |

两者均不进入代码设计前提；`blocking_unknowns` 为空。

## closure_gaps

无 blocking 缺口。非阻塞：内部 fork 的 goreleaser 外部渠道段（brew/winget/GitHub release）对内网分发无意义但保留原值——改动无效果、删配置属另一切片（advisory）。

## blocking_unknowns

（空）

## assumptions

1. 员工机器以 linux/windows 为主的分发顺序不影响本切片（全部平台产物名统一改 uni）。修正成本低（配置一处）。
2. 上游同步时本切片会形成固定补丁面（UI 弹窗、资产模板、CLI 文案三处），每次 rebase 需重放——接受为 fork 固有成本，不为此引入品牌参数化机制（克制：上游无此机制，自建抽象越出切片）。

## advisories

1. UI 五处 `obot scan` 空态文案引用的 `obot scan` 命令上游已删除（#7102）——改名后 `uni scan` 仍指向不存在的命令。属上游文案陈旧问题，本切片只做品牌一致性，不新增命令；登记后续增强候选。
2. `curl.ts` 示例中 shell 变量名 `OBOT_API_KEY`/`OBOT_BASE_URL` 属示例文本，非 CLI 读取的环境变量；随品牌改为 `UNI_API_KEY`/`UNI_BASE_URL`。
3. docs/docs 的 mcp-tunnels.md Docker 段随 G4 调整为 CLI-only。
4. goreleaser/winget/brew 外部渠道目标保持原值（内部不经这些渠道发布）。

## readiness_verdict

`goal_closure_status=closed`；`measurable_goal_contract_status=closed`；`environment_closure_status=closed`；blocking_unknowns 为空。**可以进入 plan**。
