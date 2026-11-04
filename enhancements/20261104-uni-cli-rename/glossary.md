# glossary · uni CLI 员工可见层更名

| term | definition | allowed_synonyms | avoid |
|------|-----------|------------------|-------|
| uni CLI | 更名后的命令行程序，二进制与命令名均为 `uni`，由本仓库构建 | uni 命令、uni 二进制 | "uni 版 obot"（模糊）、"新 obot" |
| 员工可见层 | 员工在终端、页面、本机文件中看到的名字与文案：命令名、flag、员工环境变量、钥匙串服务名、配置路径、UI 命令文案、引导文件、当前版本文档命令引用 | 用户可见面 | 与「服务端配置」混用 |
| 服务端配置 | `OBOT_SERVER_*` 环境变量、部署配置、API 路径与字段、Go module 路径；本切片不改 | 服务端契约 | 写成「也要改成 uni 的部分」 |
| 隧道弹窗 | `MCPTunnelSecretRevealDialog.svelte`，创建/轮换隧道 secret 后展示复制命令的对话框 | tunnel secret 弹窗 | 与 TunnelsView 列表页混用 |
| uni 命令（弹窗内） | 弹窗唯一展示并供复制的命令：`uni tunnel --uni-base-url <地址>/api --token <密钥>` | tunnel 命令 | 保留任何 Docker 变体表述 |
| 引导文件 | `uni setup` 安装到本机 AI 客户端技能目录的 SKILL.md 资产（uni、uni-search-skills、uni-search-mcp-servers、uni-install-skill） | bootstrap skills、引导技能 | "obot skills"（旧名，仅历史语境） |
| 钥匙串服务名 | 操作系统凭据存储里 CLI 存 API key 的服务标识，更名后为 `uni` | keyring service | 与 API key 本身混用 |
| 员工环境变量 | CLI 读取的 `UNI_TOKEN`、`UNI_BASE_URL`、`UNI_NON_INTERACTIVE` | uni 环境变量 | 与服务端 `OBOT_SERVER_*` 混用 |
| 版本字段 | `/api/version` 返回的 JSON 字段 `obot`；API 契约不改，UI 显示层映射标签为 `uni` | version.current.obot | "把版本字段改成 uni"（错误表述） |
| breaking-by-design | 本切片兼容策略：旧 CLI 名不做别名/迁移，直接切换 | 有意不兼容 | 写成"向后兼容"或"平滑迁移" |

## 禁用表述

- 「把 obot 全部替换」——必须限定在员工可见层；服务端契约明确排除。
- 「Docker 命令换成内网镜像」——本切片是移除 Docker 标签，不是替换镜像源。
- 「Obot v 版本号显示」——不存在该前缀（evidence：Profile.svelte:347 渲染裸值）；只讨论版本对话框标签。
