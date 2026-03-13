---
name: obot-invoke-engine
description: |
  基于 obot 项目 pkg/invoke/ 和 pkg/events/ 的调用/执行引擎设计模式指南。涵盖 Run 状态机、ChatState 持久化与链式恢复、ExternalCall 等待机制、Thread 创建与项目模型、事件流与 Emitter、资源转换函数、gz 压缩、ephemeral run、tool call 确认流程等核心实践。
  Invoke/execution engine design patterns for the obot project. Covers Run state machine, ChatState persistence and chain walking, ExternalCall waiting mechanism, Thread creation and project model, event streaming with Emitter, resource conversion functions, gz compression, ephemeral runs, and tool call confirmation flow drawn from pkg/invoke/ and pkg/events/.
---

# Obot Invoke Engine 设计模式

## 适用范围

本 Skill 适用于在 `pkg/invoke/`、`pkg/events/` 下新增或修改执行引擎逻辑的场景：

- **Run 生命周期管理**：创建、恢复、状态保存、超时控制
- **ChatState 链式恢复**：沿 `previousRunName` 链查找上一次有效状态
- **ExternalCall 等待/恢复**：实现外部调用的暂停与继续
- **Thread/Project 创建**：理解 Thread 与 Project 的关系模型
- **事件流**：通过 `Emitter` 将 GPTScript call frames 转换为 `Progress` 事件
- **资源转换**：`convertX()` 模式将内部 CR 转换为 API 类型
- **Tool Call 确认**：用户审批工具调用的完整流程

---

## 核心原则

### 1. Run 状态机

Run 有六种状态，定义在 `pkg/storage/apis/obot.obot.ai/v1/run.go`：

```go
const (
    Creating RunStateState = "creating"
    Running  RunStateState = "running"
    Continue RunStateState = "continue"
    Waiting  RunStateState = "waiting"
    Finished RunStateState = "finished"
    Error    RunStateState = "error"
)
```

关键转换逻辑在 `doSaveState()` 中——当 GPTScript 返回 `Continue` 但输出包含 `ExternalCall` 时，状态被提升为 `Waiting`：

```go
state := v1.RunStateState(runResp.State())
if state == v1.Continue && extCall != nil {
    state = v1.Waiting
}
```

`ExternalCall` 通过解析 run 输出的 JSON 检测：

```go
func toExternalCall(output string) *v1.ExternalCall {
    var call v1.ExternalCall
    if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &call); err != nil || call.Type != "obotExternalCall" || call.ID == "" {
        return nil
    }
    return &call
}
```

### 2. ChatState 链式恢复

`getChatState()` 沿 `PreviousRunName` 链向前查找最后一个 `Continue` 状态的 run，然后从 gateway 获取其 ChatState 并用 `gz.Decompress` 解压：

```go
func (i *Invoker) getChatState(ctx context.Context, c kclient.Client, run *v1.Run) (result string, _ error) {
    // 特殊处理 Waiting 状态：使用 ExternalCall ID 构造复合 key
    if run.Status.State == v1.Waiting {
        id := v1.RunStateNameWithExternalID(run.Name, run.Status.ExternalCall.ID)
        // ...获取或复制 RunState
        return result, gz.Decompress(&result, lastRun.ChatState)
    }

    if run.Spec.PreviousRunName == "" {
        return "", nil
    }

    // 链式查找：跳过非 Continue 状态的 run
    for {
        var previousRun v1.Run
        if err := c.Get(ctx, router.Key(run.Namespace, run.Spec.PreviousRunName), &previousRun); err != nil {
            if !apierror.IsNotFound(err) {
                return "", err
            }
            // 缓存未命中时回退到 uncached client
            if err := i.uncached.Get(ctx, router.Key(run.Namespace, run.Spec.PreviousRunName), &previousRun); err != nil {
                return "", err
            }
        }
        if previousRun.Status.State == v1.RunStateState(gptscript.Continue) {
            break
        }
        if previousRun.Spec.PreviousRunName == "" {
            return "", nil
        }
        run = &previousRun
    }

    lastRun, err := i.gatewayClient.RunState(ctx, run.Namespace, run.Spec.PreviousRunName)
    // ...
    return result, gz.Decompress(&result, lastRun.ChatState)
}
```

### 3. Thread 与 Project 模型

Thread 是执行的基本单元。Project 是一个特殊的 Thread（`Spec.Project: true`），不能直接被 invoke：

```go
if thread.Spec.Project && !opts.Ephemeral {
    return nil, fmt.Errorf("project threads cannot be invoked")
}
```

创建 Thread 时使用 `GenerateName` + `Finalizer` 模式：

```go
thread := &v1.Thread{
    ObjectMeta: metav1.ObjectMeta{
        GenerateName: system.ThreadPrefix,
        Namespace:    agent.Namespace,
        Finalizers:   []string{v1.ThreadFinalizer},
    },
    Spec: v1.ThreadSpec{
        Manifest: types.ThreadManifest{
            Tools: agent.Spec.Manifest.DefaultThreadTools,
        },
        AgentName:        agent.Name,
        ParentThreadName: parentThreadName,
        UserID:           userID,
    },
}
```

从 Project 创建子 Project 时，复制 manifest 但保持独立身份：

```go
func CreateProjectFromProject(ctx context.Context, c kclient.WithWatch, projectThread *v1.Thread, threadName, userUID string) (*v1.Thread, error) {
    thread := v1.Thread{
        ObjectMeta: metav1.ObjectMeta{
            Name:       threadName,
            Namespace:  projectThread.Namespace,
            Finalizers: []string{v1.ThreadFinalizer},
        },
        Spec: v1.ThreadSpec{
            Manifest: types.ThreadManifest{
                ThreadManifestManagedFields: types.ThreadManifestManagedFields{
                    Name:        projectThread.Spec.Manifest.Name,
                    Description: projectThread.Spec.Manifest.Description,
                    Icons:       projectThread.Spec.Manifest.Icons,
                },
                Prompt: projectThread.Spec.Manifest.Prompt,
            },
            AgentName:        projectThread.Spec.AgentName,
            ParentThreadName: projectThread.Name,
            UserID:           userUID,
            Project:          true,
        },
    }
    return &thread, c.Create(ctx, &thread)
}
```

### 4. Invoker 结构体与依赖注入

`Invoker` 持有执行引擎所需的所有依赖，通过构造函数注入：

```go
type Invoker struct {
    uncached                 kclient.WithWatch
    gatewayClient            *client.Client
    tokenService             *persistent.TokenService
    events                   *events.Emitter
    serverURL                string
    internalServerURL        string
    autonomousToolUseEnabled bool
}
```

### 5. gz 压缩/解压

所有大型状态数据（ChatState、CallFrame、Program、Output）通过 `pkg/gz` 进行 gzip 压缩存储：

```go
// 压缩：接受 string 或任意可 JSON 序列化的对象
runStateSpec.Output, err = gz.Compress(text)

// 解压：目标可以是 *string 或任意可 JSON 反序列化的对象
err = gz.Decompress(&result, lastRun.ChatState)
```

---

## 操作流程

### 流程 1：创建并执行 Run

`createRun()` 是核心入口，负责创建 Run 资源并启动执行：

```go
func (i *Invoker) createRun(ctx context.Context, gptClient *gptscript.GPTScript, c kclient.WithWatch, thread *v1.Thread, tool any, input string, opts runOptions) (*Response, error) {
    // 1. 构造 Run 对象
    run := v1.Run{
        ObjectMeta: metav1.ObjectMeta{
            GenerateName: generateName,
            Namespace:    thread.Namespace,
            Finalizers:   []string{v1.RunFinalizer},
        },
        Spec: v1.RunSpec{
            ThreadName:      thread.Name,
            PreviousRunName: previousRunName,
            Input:           input,
            Tool:            string(toolData),
            // ...
        },
    }

    // 2. 持久化（ephemeral run 跳过）
    if opts.Ephemeral {
        run.Name = fmt.Sprintf("%s-%d", ephemeralRunPrefix, ephemeralCounter.Add(1))
    } else {
        if err := c.Create(ctx, &run); err != nil {
            return nil, err
        }
    }

    // 3. 更新 Thread 状态（非系统任务）
    // 使用 retry.RetryOnConflict 处理并发冲突

    // 4. 同步模式：启动 event watch + goroutine 执行 Resume
    if opts.Synchronous {
        go func() {
            if err := i.Resume(ctx, gptClient, c, thread, &run); err != nil {
                log.Errorf("run failed: %v", err)
            }
        }()
    }

    return resp, nil
}
```

### 流程 2：Resume 执行（恢复 Run）

`Resume()` 是实际执行逻辑的入口：

```go
func (i *Invoker) Resume(ctx context.Context, gptClient *gptscript.GPTScript, c kclient.WithWatch, thread *v1.Thread, run *v1.Run) error {
    // 1. 等待 Thread 就绪（workspace 创建完成）
    thread, err = wait.For(ctx, c, thread, func(thread *v1.Thread) (bool, error) {
        return thread.Status.Created, nil
    })

    // 2. 处理 Waiting 状态的恢复
    if run.Status.State == v1.Waiting {
        // 查找匹配的 ExternalCallResult，构造恢复输入
    }

    // 3. 获取 ChatState（链式查找）
    chatState, err := i.getChatState(ctx, c, run)

    // 4. 构造 JWT token（包含用户、项目、模型等上下文）
    token, err := i.tokenService.NewToken(ctx, persistent.TokenContext{...})

    // 5. 构造 GPTScript 选项并执行
    options := gptscript.Options{
        ChatState:     chatState,
        Confirm:       !autonomousToolUseEnabled,
        // ...
    }

    // 6. 根据 Tool 格式选择执行方式
    switch run.Spec.Tool[0] {
    case '"': runResp, err = gptClient.Run(ctx, toolRef, options)
    case '[': runResp, err = gptClient.Evaluate(ctx, options, toolDefs...)
    case '{': runResp, err = gptClient.Evaluate(ctx, options, toolDef)
    }

    // 7. 流式处理事件
    return i.stream(ctx, gptClient, c, thread, run, runResp)
}
```

### 流程 3：事件流处理（stream）

`stream()` 是事件循环的核心，处理 GPTScript 的所有事件类型：

```go
func (i *Invoker) stream(ctx context.Context, gptClient *gptscript.GPTScript, c kclient.WithWatch, thread *v1.Thread, run *v1.Run, runResp *gptscript.Run) error {
    // DeepCopy 避免修改原始对象
    thread = thread.DeepCopyObject().(*v1.Thread)
    run = run.DeepCopyObject().(*v1.Run)

    // 后台定期保存状态（每秒）
    go func() {
        for {
            select {
            case <-saveCtx.Done():
                return
            case <-time.After(time.Second):
                _ = i.saveState(ctx, c, thread, run, runResp, nil)
            }
        }
    }()

    // 超时控制
    go timeoutAfter(runCtx, cancelRun, timeout)

    // 监听 Thread abort
    go i.watchThreadAbort(runCtx, c, thread, cancelRun)

    // 主事件循环
    for {
        select {
        case <-runCtx.Done():
            return context.Cause(runCtx)
        case frame, ok := <-runEvent:
            // 处理 Prompt、CallConfirm、CallProgress、CallFinish 等事件
        }
    }
}
```

关键设计：`defer` 中使用独立的 `context.WithTimeout(context.Background(), 15*time.Second)` 保存最终状态，避免父 context 取消导致状态丢失。

### 流程 4：Tool Call 确认流程

当 `autonomousToolUseEnabled` 为 false 时，工具调用需要用户确认：

```go
case gptscript.EventTypeCallConfirm:
    // 1. 检查是否已预批准
    if isApprovedTool(toolName, latestThread.Spec.ApprovedTools) {
        gptClient.Confirm(runCtx, gptscript.AuthResponse{ID: callID, Accept: true})
        break
    }

    // 2. 通过 JSON Patch 将 callID 添加到 run.Status.RequestedCallDecisions
    addRequestedCallDecision(runCtx, c.Status(), run, callID)

    // 3. 启动 goroutine 监听用户决策
    go watchRunCallDecision(runCtx, gptClient, c, run, callID)
```

`addRequestedCallDecision` 使用 JSON Patch 而非 Update，避免资源竞争：

```go
func addRequestedCallDecision(ctx context.Context, c kclient.SubResourceWriter, run *v1.Run, callID string) error {
    var patchPath = "/status/requestedCallDecisions"
    if len(run.Status.RequestedCallDecisions) > 0 {
        patchPath += "/-"  // JSON Patch append
    }
    // ...
    return c.Patch(ctx, run, kclient.RawPatch(ktypes.JSONPatchType, patchBytes))
}
```

### 流程 5：Emitter 事件广播

`Emitter` 使用 `sync.Cond` 广播机制将 live 状态推送给所有 watcher：

```go
type Emitter struct {
    liveStates    map[string][]liveState
    liveStateLock sync.RWMutex
    liveBroadcast *sync.Cond
}

func (e *Emitter) Submit(run *v1.Run, prg *gptscript.Program, frames gptscript.CallFrames) {
    e.liveStateLock.Lock()
    defer e.liveStateLock.Unlock()
    e.liveStates[run.Name] = append(e.liveStates[run.Name], liveState{Prg: prg, Frames: &frames})
    e.liveBroadcast.Broadcast()
}
```

`printRun()` 同时消费两个数据源：
- **live 内存状态**：通过 `sync.Cond` 广播接收
- **持久化 RunState**：通过定时 tick 从 gateway 拉取

使用 `hasRolledBack()` 检测并丢弃过时消息，避免重复输出。

### 流程 6：convertX() 资源转换模式

将内部 Kubernetes CR 转换为 API 响应类型，统一放在 `pkg/api/handlers/` 中：

```go
// pkg/api/handlers/projects.go
func convertProject(thread *v1.Thread, parentThread *v1.Thread) types.Project {
    p := types.Project{
        Metadata: MetadataFrom(thread),
        ProjectManifest: types.ProjectManifest{
            ThreadManifest:       thread.Spec.Manifest,
            DefaultModelProvider: thread.Spec.DefaultModelProvider,
            // ...
        },
        ParentID:    strings.Replace(thread.Spec.ParentThreadName, system.ThreadPrefix, system.ProjectPrefix, 1),
        AssistantID: thread.Spec.AgentName,
        UserID:      thread.Spec.UserID,
    }
    // 合并父项目的 tools
    if parentThread != nil {
        p.Tools = append(p.Tools, parentThread.Spec.Manifest.Tools...)
    }
    return p
}

// pkg/api/handlers/threads.go
func convertThread(thread v1.Thread) types.Thread { ... }

// pkg/api/handlers/tasks.go
func convertTask(workflow v1.Workflow, trigger *triggers) types.Task { ... }
```

### 流程 7：Ephemeral Run（临时执行）

用于系统任务等不需要持久化的场景：

```go
func (i *Invoker) EphemeralThreadTask(ctx context.Context, ...) (string, error) {
    resp, err := i.createRun(ctx, gptClient, i.uncached, thread, tool, inputString, runOptions{
        Ephemeral:   true,
        Synchronous: true,
    })
    defer resp.Close()
    result := strings.Builder{}
    for event := range resp.Events {
        if event.Error != "" {
            return "", errors.New(event.Error)
        }
        result.WriteString(event.Content)
    }
    return result.String(), nil
}
```

Ephemeral run 的特点：
- 使用计数器生成名称（`ephemeral-run-N`），不写入 Kubernetes
- 不保存状态（`saveState` 中 `isEphemeral` 检查直接返回）
- 不监听 Thread abort
- 不更新 Thread 的 `LastRunName`

### 流程 8：Project 复制（CopyProject）

复制项目时沿 `ParentThreadName` 链向上查找根项目，然后创建独立副本：

```go
func (h *ProjectsHandler) CopyProject(req api.Context) error {
    // 1. 沿 ParentThreadName 链找到根 Thread
    for thread.Spec.ParentThreadName != "" {
        if err := req.Get(&thread, thread.Spec.ParentThreadName); err != nil {
            return err
        }
    }

    // 2. 创建新 Thread，复制 manifest 但不复制 model/credentials
    newThread := v1.Thread{
        Spec: v1.ThreadSpec{
            Manifest:         thread.Spec.Manifest,
            AgentName:        thread.Spec.AgentName,
            SourceThreadName: thread.Name,
            Project:          true,
            UserID:           req.User.GetUID(),
            // Explicit ignoring model provider and model here.
        },
    }
    newThread.Spec.Manifest.Name = "Copy of " + newThread.Spec.Manifest.Name

    return req.Create(&newThread)
}
```

### 流程 9：前端 msg.ignore 模式

前端使用 `msg.ignore` 标志控制消息的显示/隐藏，用于处理并发 tool call 的去重和用户主动清除：

```svelte
<!-- 用户点击清除按钮 -->
<button onclick={() => (msg.ignore = true)}>
    <X class="icon-default" />
</button>

<!-- 整个消息组件的渲染由 ignore 控制 -->
{#if !msg.ignore}
    <!-- 消息内容 -->
{/if}
```

### 流程 10：前端 SSR 环境分离

SvelteKit 中使用 `browser` 判断运行环境，避免 SSR 阶段访问浏览器 API：

```typescript
import { browser } from '$app/environment';

// 仅在浏览器端执行
if (browser) {
    localStorage.setItem('lastVisitedObot', params.project);
}

// HTTP 基础 URL 根据环境切换
export let baseURL = 'http://localhost:8080/api';
if (typeof window !== 'undefined') {
    baseURL = baseURL.replace('http://localhost:8080', window.location.origin);
}
```

---

## 注意事项

### 关键约束

1. **状态保存使用独立 context**
   - `stream()` 的 defer 中使用 `context.WithTimeout(context.Background(), 15*time.Second)`
   - 确保父 context 取消后仍能保存最终状态

2. **DeepCopy 防止并发修改**
   - `stream()` 入口处对 thread 和 run 做 DeepCopy
   - 避免事件循环中的修改影响外部引用

3. **Ephemeral run 不持久化**
   - `isEphemeral()` 检查贯穿整个生命周期
   - 不写 Kubernetes、不保存 RunState、不更新 Thread

4. **ExternalCall 的幂等性**
   - `getChatState()` 在 Waiting 状态下使用 `RunStateNameWithExternalID` 构造复合 key
   - 首次访问时复制现有 RunState，确保后续调用幂等

5. **retry.RetryOnConflict 处理并发**
   - Thread 状态更新使用 `retry.RetryOnConflict`
   - 失败时不返回错误（非关键路径），仅记录日志

6. **JSON Patch 替代 Update**
   - `addRequestedCallDecision` 使用 JSON Patch 追加数组元素
   - 避免与其他并发更新产生冲突

### 常见陷阱

| 问题 | 影响 | 解决方案 |
|------|------|----------|
| 父 context 取消后保存状态失败 | 最终状态丢失 | 使用 `context.Background()` + timeout |
| 不做 DeepCopy 直接修改 thread/run | 竞态条件 | 入口处 `DeepCopyObject()` |
| Ephemeral run 尝试保存状态 | 不必要的 I/O | 检查 `isEphemeral()` |
| `getChatState` 未处理 uncached 回退 | 缓存延迟导致 NotFound | 先查 cached，NotFound 时查 uncached |
| 忘记 `resp.Close()` | goroutine 泄漏 | 始终 defer Close |

---

## 反模式（避免）

| 反模式 | 正确做法 |
|--------|----------|
| 在 `stream()` defer 中使用父 ctx 保存状态 | `context.WithTimeout(context.Background(), 15s)` |
| 直接修改传入的 thread/run 指针 | `DeepCopyObject()` 后操作副本 |
| 用 `Status().Update()` 追加数组元素 | 用 JSON Patch（`addRequestedCallDecision`） |
| Ephemeral run 写入 Kubernetes | 使用计数器命名，跳过 `c.Create()` |
| `getChatState` 只查 cached client | NotFound 时回退到 `i.uncached` |
| 吞掉非关键错误不记录 | `log.Errorf` 记录但不返回（如 Thread 状态更新失败） |
| 硬编码超时时间 | 使用 `run.Spec.Timeout.Duration`，提供合理默认值 |

---

## 相关 Skills

- [obot-api-handler](../obot-api-handler/SKILL.md)：API Handler 如何调用 Invoker
- [obot-controller-handler](../obot-controller-handler/SKILL.md)：Controller 如何触发 Workflow 执行
- [obot-service-wiring](../obot-service-wiring/SKILL.md)：Invoker 的依赖注入与初始化
- [obot-authz](../obot-authz/SKILL.md)：Run 执行中的用户权限与 token 生成
- [review-go-style](../review-go-style/SKILL.md)：Go 代码风格审查
