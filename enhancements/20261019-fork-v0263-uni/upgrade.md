# 从 fork/v0.20.1-uni 升到 fork/v0.26.3-uni

本文只给操作步骤。它留在仓库里，不推送，不发到仓库外。

## 上游从 v0.20.1 到 v0.26.3 的变化

下面按上游版本列出。带“升级时处理”的项会改变现有用法，其余是新能力，不用就不用改配置。

- v0.21：网络策略可以用 Helm 部署；增加存储用的服务账号。
- v0.22：升级时处理：旧聊天从界面和接口移除，包括 `/api/agents`、`/api/assistants`、`/api/projects`、`/api/shares`、`/api/tasks`。0.20 的 Agent、项目、线程和任务没有搬进新版本。旧 Agent 上的定时任务由当时的 CronJob 控制器触发；0.26 不再注册这个控制器，也没有转换这些记录，所以升级后它们不再触发。新界面里的定时页只管理 nanobot 容器里新建的任务。同步新增命令行安装、技能页、镜像拉取密钥、外部密钥、OAuth 调试、本地扫描和多用户服务的自定义请求头。
- v0.23：升级时处理：旧的认证 token 被弃用，API token 的范围有变化；模型与认证 provider 改为在容器里运行。同步新增 MCP OAuth 同意页、企业许可证、多用户目录模板、OpenAI 与 Anthropic 直通，以及拦截内网地址的选项。
- v0.24：新增内置本地登录、Git 凭据、LLM 审计日志和导出、AWS Bedrock、Azure、通用模型供应商、设备登记与 MDM 下载、本地工具调用审计。组合 MCP 不再单独起容器。数据库连接可以按连接数和生命周期分开配置。
- v0.25：升级时处理：原来的 API key 改为 Agent Auth Scope。同步新增用户数和设备数限制、社区版登记、MCP WebSocket 隧道、工具调用拦截。
- v0.26：升级时处理：现有 MCP 目录和服务会迁移成虚拟 MCP；一批旧 MCP 接口和授权路径被删除。新的 Agent 不是旧 Agent 的升级：它会起一个 nanobot 容器，用自己的状态库，并通过 MCP 提供服务；旧 Agent 存在应用进程里，带工具、知识和线程。新功能默认关闭，只有库里已经有 NanobotAgent 时才自动打开。0.20 的旧 Agent 不算数；要在升级后使用，设置 `OBOT_ENABLE_AGENTS=true`。同步新增虚拟 MCP 的 Git 同步、MCP 测试器、模型代理、产品使用统计开关，以及认证 provider 的安全切换。

数据库仍是原来的 Postgres。`OBOT_SERVER_DSN` 这个键名不变，Helm 里从 config 挪到 secret。应用启动时在一个事务里自动迁移，失败会回滚这一次事务；不要把迁移过的库直接交回 0.20。

启动迁移会删除这些表和其中的数据：`run`、`thread`、`run_states`、`runstate`、`sessions`、`session_cookies`、`obot_mcp_tokens`。旧结构的 `mcp_oauth_tokens` 也会被删掉重建。用户创建的 API key 不在其中：迁移只补一列技能权限，已有 key 都写成允许，key 本身不变。旧 Agent 和 CronJob 存在同一库的对象表里，启动不会删除这些记录，但 0.26 不再读取它们，所以它们不会继续运行。升级前先备份整个库。
- 镜像内容从 `obot-tools` 换成 `obot-providers`。0.20 的 Dockerfile 复制 `/obot-tools`；这一版复制 `/obot-providers`。
- 数据库连接串仍是 Postgres，键名仍是 `OBOT_SERVER_DSN`。0.20 放在 chart 的 config 下，这一版放在 secret 下。升级时把原值挪过去，不要改成新库。
- 应用启动时用 GORM 自动迁移现有库。不另跑手工迁移脚本。
- Keycloak 登录资料和用户、设备不限仍在。单会话不在这一版里：同一账号可以同时留多个登录。
- 界面上用户能看见的 “Obot” 显示为 “uni”。地址、环境变量和代码标识不变。

## 升级

1. 停写之前，给正在使用的 Postgres 做一份备份，并记下当前镜像标签。
2. 用 `fork/v0.26.3-uni` 构建新镜像。构建时使用 `obot-providers`，不要再挂 `obot-tools`。
3. 在 Helm 值里把原来的 `OBOT_SERVER_DSN` 填到 secret 下的同名键。保留原库。
4. 发布新镜像。应用启动后自己迁移这一个库。
5. 按下面的验证做完，再恢复写入。

## 回滚

1. 停新版本。
2. 用升级前的备份恢复 Postgres。不要把新版本迁移过的库直接交回 0.20。
3. 把镜像和 Helm 值切回 `fork/v0.20.1-uni` 当时的版本，包括 `obot-tools` 和原来的 DSN 位置。

## 验证

- 应用能连上原来的 Postgres，并完成启动。
- 打开登录页，跳到 Keycloak，登录后回到应用，页面显示已登录。
- 抽查一个升级前就存在的用户，确认还能登录并看到原有数据。
- 确认界面可见产品名是 “uni”，登录地址和环境变量没有改名。
- 确认同一账号可以同时保留两个登录。这是这一版不再提供单会话的结果。
