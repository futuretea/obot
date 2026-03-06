---
name: discobot-hooks-and-services
description: |
  指导如何为项目创建 .discobot 文件夹，包含 hooks（自动化钩子脚本）和 services（开发服务定义）。
  根据项目技术栈（语言、包管理器、构建工具）生成匹配的配置文件。
  Use this skill to scaffold .discobot/hooks/ and .discobot/services/ for any project by analyzing its tech stack.
---

# Discobot Hooks & Services 配置

## 适用范围

为项目创建 `.discobot/` 目录结构，包含：

- **hooks/**：在特定事件（session 启动、文件变更、pre-commit）时自动触发的 shell 脚本
- **services/**：定义可在开发环境中启动的后台服务（Web Server、数据库 GUI 等）

---

## 目录结构

```
.discobot/
├── hooks/
│   ├── 01-install-deps.sh     # session 启动时安装依赖（blocking: false）
│   ├── 02-install-deps.sh     # 依赖文件变更时重新安装
│   ├── 03-*.sh                # 文件变更时的格式检查 / lint
│   ├── 04-*.sh                # 文件变更时的构建
│   ├── 05-docker-build.sh     # Dockerfile 变更时构建镜像
│   └── 06-ci.sh               # pre-commit 时运行 CI 检查
└── services/
    ├── ui.sh                  # 前端开发服务器（带 http 端口声明）
    ├── api                    # 静态 API 服务描述（YAML front matter）
    └── db.sh                  # 数据库 GUI 服务（可选）
```

---

## Hook 文件格式

每个 hook 是一个 shell 脚本，顶部用 `#---` 注释块声明元数据：

```bash
#!/bin/bash
#---
# name: <可读名称>
# type: session | file | pre-commit
# blocking: true | false          # 可选，仅 session 类型用
# pattern: "glob pattern"         # 仅 file 类型需要
#---
# 脚本内容
```

### type 说明

| type | 触发时机 |
|------|----------|
| `session` | 开发会话启动时（适合安装依赖等初始化操作） |
| `file` | 指定 pattern 的文件发生变更时 |
| `pre-commit` | git commit 前 |

### blocking 说明

- `blocking: false`：后台并行执行，不阻塞会话启动（适合耗时的依赖下载）
- 默认（不写）：阻塞执行

---

## Service 文件格式

### 静态描述文件（无 `.sh` 后缀）

用 YAML front matter 描述一个已运行的服务（如 API 服务由其他进程启动）：

```yaml
---
name: <服务名称>
description: <服务描述>
http: <端口号>
path: /api/ui          # 可选，健康检查或代理路径
---
```

### 可执行服务脚本（`.sh` 文件）

自启动的服务，脚本顶部使用 `#---` 注释块：

```bash
#!/bin/bash
#---
# name: <服务名称>
# description: <服务描述>
# http: <端口号>
#---

set +x

# 启动命令
exec <start-command>
```

---

## 操作流程

### 1. 分析项目技术栈

阅读项目根目录的以下文件来判断技术栈：

| 检查文件 | 推断信息 |
|----------|----------|
| `package.json` | 前端项目；脚本命令（`dev`、`build`、`lint`） |
| `bun.lock` | 包管理器为 bun |
| `pnpm-lock.yaml` / `pnpm-workspace.yaml` | 包管理器为 pnpm |
| `go.mod` | 含 Go 后端 |
| `Dockerfile` | 需要 docker build hook |
| `docker-compose.yml` | 可能有数据库服务 |
| `Makefile` | 查看 build/lint/check 目标 |

### 2. 确定 hooks 集合

根据技术栈选择需要的 hooks：

**所有项目都需要：**
- `01-install-deps.sh`（session，blocking: false）
- `02-install-deps.sh`（file，监听锁文件/依赖清单变更）

**前端项目（有 package.json）：**
- lint hook：监听 `**/*.{ts,tsx,js,jsx,json}` → 运行 lint 命令
- build hook：监听 `**/*.{ts,tsx,js,jsx,json,css}` → 运行 build 命令

**Go 项目：**
- `go-mod-tidy.sh`（file，监听 `**/go.mod`）→ `go mod tidy`
- backend lint hook：监听 `**/*.go` → 运行 golangci-lint 或项目定义的命令
- backend build hook：监听 `**/*.go` → `go build`

**有 Dockerfile：**
- docker-build hook：监听 `Dockerfile` → `docker build .`

**pre-commit：**
- CI hook：运行项目的完整检查命令

### 3. 确定 services 集合

| 场景 | 创建的 service |
|------|----------------|
| 有 Vite `dev` 脚本 | `dev.sh`，端口 5173（或 vite.config 中配置的端口） |
| 有 Go API 服务 | `api` 静态描述文件，端口参考项目配置 |
| 有 SQLite 数据库 | `db.sh`，用 datasette 或 sqlite-web 提供 GUI，端口 8080 |
| 有 PostgreSQL | `db.sh`，用 pgweb 或 adminer 提供 GUI |

### 4. 根据包管理器选择命令

| 包管理器 | 安装命令 | lint 命令 | build 命令 |
|----------|----------|-----------|------------|
| bun | `bun install --frozen-lockfile` | `bun run lint` | `bun run build` |
| pnpm | `pnpm install --frozen-lockfile` | `pnpm check:frontend:fix` | `pnpm build:frontend` |
| npm | `npm ci` | `npm run lint` | `npm run build` |
| yarn | `yarn install --frozen-lockfile` | `yarn lint` | `yarn build` |

实际命令以 `package.json` 的 `scripts` 字段为准。

### 5. 生成文件

按序号命名 hooks（`01-`、`02-`、...）以控制执行顺序。
所有 `.sh` 文件需要赋予执行权限：

```bash
chmod +x .discobot/hooks/*.sh .discobot/services/*.sh
```

---

## 完整示例

### 纯前端项目（Vite + React + bun）

```
.discobot/
├── hooks/
│   ├── 01-install-deps.sh     # session，bun install（blocking: false）
│   ├── 02-install-deps.sh     # file，{package.json,bun.lock}，bun install
│   ├── 03-check-frontend.sh   # file，**/*.{ts,tsx,js,jsx,json}，bun run lint
│   ├── 04-build-frontend.sh   # file，**/*.{ts,tsx,js,jsx,json,css}，bun run build
│   ├── 05-docker-build.sh     # file，Dockerfile，docker build .
│   └── 06-ci.sh               # pre-commit，bun run lint
└── services/
    ├── ui                     # 静态描述，http: 5173
    └── dev.sh                 # bun install && bun run dev -- --host
```

### 全栈项目（Go 后端 + pnpm 前端）

```
.discobot/
├── hooks/
│   ├── 01-install-deps.sh     # session，pnpm install + go mod download（blocking: false）
│   ├── 02-install-deps.sh     # file，{package.json,pnpm*.yaml}，pnpm install
│   ├── 03-go-mod-tidy.sh      # file，**/go.mod，go mod tidy
│   ├── 04-check-frontend.sh   # file，**/*.{ts,tsx,js,jsx,json}，pnpm check:frontend:fix
│   ├── 05-check-backend.sh    # file，**/*.go，pnpm check:backend:fix
│   ├── 06-build-frontend.sh   # file，**/*.{ts,tsx,js,jsx,json}，pnpm build:frontend
│   ├── 07-build-backend.sh    # file，**/*.go，pnpm build:server
│   ├── 07-docker-build.sh     # file，Dockerfile，docker build .
│   └── 08-ci.sh               # pre-commit，pnpm run check
└── services/
    ├── api                    # 静态描述，http: 3001，path: /api/ui
    ├── db.sh                  # SQLite GUI via datasette，http: 8080
    └── ui.sh                  # pnpm install && pnpm dev:backend
```

---

## 注意事项

### 关键约束

1. **session hook 设置 `blocking: false`**：安装依赖耗时，应异步执行
2. **file hook 的 pattern 要精确**：避免过于宽泛的 pattern 导致频繁触发
3. **services 端口不要冲突**：前端 5173、API 3001、DB GUI 8080 是常见约定
4. **脚本需要赋予执行权限**：创建后执行 `chmod +x`
5. **服务脚本使用 `set +x`**：避免输出冗余的命令回显

### 常见陷阱

| 问题 | 解决方案 |
|------|----------|
| 多个 Go 模块目录 | `03-go-mod-tidy.sh` 遍历 `$DISCOBOT_CHANGED_FILES` 逐目录执行 |
| pnpm monorepo | session hook 中分别对各子包执行 `go mod download` |
| 数据库文件不存在 | `db.sh` 中检查 DB 文件存在性，不存在时打印提示并退出 |
| Vite 默认只监听 localhost | service 启动命令加 `--host` 参数 |
