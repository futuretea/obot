---
name: obot-svelte-ui
description: |
  基于 obot 项目 ui/user/ 的 SvelteKit 5 前端开发实践，涵盖 Svelte 5 Runes 状态管理、组件设计模式、Tailwind CSS 4 样式、SSR/CSR 渲染策略、TypeScript 类型安全等核心实践。
  SvelteKit 5 frontend development patterns covering Svelte 5 Runes state management, component design patterns, Tailwind CSS 4 styling, SSR/CSR rendering strategies, and TypeScript type safety.
---

# Obot SvelteKit 5 前端开发模式

## 适用范围

本 Skill 适用于在 `ui/user/` 下新增或修改前端组件的场景：

- **路由与页面**：`src/routes/` - 文件路由、布局、数据加载
- **组件库**：`src/lib/components/` - 可复用 Svelte 组件
- **服务层**：`src/lib/services/` - HTTP 客户端与 API 调用
- **状态管理**：`src/lib/stores/` - Svelte 5 rune stores
- **上下文**：`src/lib/context/` - 组件树上下文传递
- **自定义 Runes**：`src/lib/runes/` - 可复用响应式逻辑
- **自定义 Actions**：`src/lib/actions/` - DOM 行为封装

---

## 核心原则

### 1. Svelte 5 Runes 状态管理

使用 `$state`、`$derived`、`$effect`、`$props`、`$bindable` 替代旧的 `let` 声明式状态：

```svelte
<script lang="ts">
    // $state：可变响应式状态
    let spinning = $state(false);
    let blobUrl = $state<string>();

    // $derived：计算派生值（自动追踪依赖）
    let content = $derived(msg.message?.join('') || '');

    // $derived.by：复杂计算
    let rd = $derived.by(() => {
        const filtered = items.filter(i => i.active);
        return filtered.sort((a, b) => a.name.localeCompare(b.name));
    });

    // $effect：副作用（自动追踪 + cleanup）
    $effect(() => {
        if (setDarkMode !== darkMode.isDark) reload?.();
    });

    // $props：组件属性声明
    let { msg, content } = $props();

    // $bindable：双向绑定属性
    let value = $bindable('');
</script>
```

### 2. Tween 流式文本动画

使用 `Tween` 实现聊天消息的流式打字效果：

```svelte
<!-- src/lib/components/messages/Message.svelte:105-143 -->
<script lang="ts">
    import { Tween } from 'svelte/motion';

    let { msg, content } = $props();

    const shouldAnimate = !msg.done && !msg.toolCall && !msg.promptId && !msg.sent;
    let cursor = new Tween(0);
    let prevContent = '';
    let animating = false;
    let animatedText = $derived(shouldAnimate ? content.slice(0, cursor.current) : content);

    $effect(() => {
        if (!shouldAnimate) return;

        // 内容被替换（非追加）时，重置光标位置
        if (!content.startsWith(prevContent)) {
            cursor.set(0, { duration: 0 });
        }
        prevContent = content;
        animating = true;
        cursor.set(content.length, { duration: 500 }).then(() => (animating = false));
    });
</script>

{animatedText}
```

关键点：
- `shouldAnimate` 排除已完成、工具调用、提示和已发送的消息
- 内容替换（非追加）时 `duration: 0` 即时重置光标
- `$derived` 根据动画开关决定显示截断文本还是完整文本

### 3. Blob URL 生命周期管理

使用 `$effect` 返回 cleanup 函数防止内存泄漏：

```svelte
<!-- src/lib/components/editor/Pdf.svelte:1-27 -->
<script lang="ts">
    let { file } = $props();
    let blobUrl = $state<string>();

    $effect(() => {
        if (!file.file?.blob) return;

        const url = URL.createObjectURL(
            new Blob([file.file?.blob], { type: 'application/pdf' })
        );
        blobUrl = url;

        // cleanup：组件销毁或 file.blob 变化时释放 URL
        return () => URL.revokeObjectURL(url);
    });
</script>

{#if blobUrl}
    <iframe src={blobUrl} class="h-full w-full" title="PDF Viewer" />
{/if}
```

### 4. Context 上下文传递

使用 Svelte Context API + `$state` 实现组件树状态共享：

```typescript
// src/lib/context/projectTools.svelte.ts
import { getContext, hasContext, setContext } from 'svelte';

const PROJECT_TOOLS_CONTEXT_NAME = 'projectTools';

export function getProjectTools(): ProjectTools {
    if (!hasContext(PROJECT_TOOLS_CONTEXT_NAME)) {
        throw new Error('layout context not initialized');
    }
    return getContext<ProjectTools>(PROJECT_TOOLS_CONTEXT_NAME);
}

export function initProjectTools(init: ProjectTools) {
    const projectTools = $state<ProjectTools>(init);
    setContext(PROJECT_TOOLS_CONTEXT_NAME, projectTools);
}
```

### 5. 自定义 Rune Store

使用 `$state` 封装可复用的响应式逻辑：

```typescript
// src/lib/runes/localState.svelte.ts
export function localState<T = string>(
    key: string,
    defaultValue: T,
    { parse = defaultParser }: LocalStateParams<T> = {}
) {
    let value = $state<T | undefined | null>();
    let isReady = $state(false);

    $effect(() => {
        const local = localStorage.getItem(key);
        untrack(() => {
            value = local ? parse(local) : set(defaultValue);
            isReady = true;
        });
    });

    function set(v: T) {
        localStorage.setItem(key, JSON.stringify(v));
        value = v;
        return v;
    }

    return {
        get current() { return value; },
        set current(v) { value = v; }
    };
}

// src/lib/stores/profile.svelte.ts
const store = $state({
    current: { ... } as Profile,
    initialize(profile?: Profile) {
        if (profile) store.current = profile;
    }
});
export default store;
```

### 6. Snippet 模板复用

使用 `{#snippet}` 和 `{@render}` 替代旧的 slot 模式：

```svelte
<!-- 定义 snippet -->
{#snippet timeAndUsername()}
    <div class="flex items-center gap-2 text-xs text-gray-500">
        {#if msg.time}
            <span>{formatTime(msg.time)}</span>
        {/if}
    </div>
{/snippet}

<!-- 渲染 snippet -->
{@render timeAndUsername()}

<!-- snippet 作为 props 传递 -->
<Table data={items}>
    {#snippet row(item)}
        <td>{item.name}</td>
    {/snippet}
</Table>
```

### 7. 自定义 Action

封装 DOM 行为逻辑为可复用的 action：

```typescript
// src/lib/actions/tooltip.svelte.ts
export function tooltip(node: HTMLElement, opts: TooltipOptions | string | undefined) {
    let tt: ReturnType<typeof popover> | null = null;

    const enable = (opts: TooltipOptions) => {
        tt = popover(node, { content: opts.content, placement: opts.placement });
    };

    const disable = () => {
        tt?.destroy();
        tt = null;
    };

    if (opts) enable(typeof opts === 'string' ? { content: opts } : opts);

    return {
        update(newOpts: TooltipOptions | string | undefined) {
            disable();
            if (newOpts) enable(typeof newOpts === 'string' ? { content: newOpts } : newOpts);
        },
        destroy: () => disable()
    };
}

// 使用
<button use:tooltip="Delete this item">X</button>
```

---

## 操作流程

### 流程 1：数据加载与路由

```typescript
// src/routes/o/[project]/+page.ts
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch }) => {
    // 并行数据获取
    const [project, threads, tools] = await Promise.all([
        getProject(params.project, { fetch }),
        getThreads(params.project, { fetch }),
        getTools(params.project, { fetch }),
    ]);

    return { project, threads, tools };
};
```

```svelte
<!-- src/routes/o/[project]/+page.svelte -->
<script lang="ts">
    let { data } = $props();

    // 使用 $derived 计算派生状态
    let rd = $derived.by(() => {
        return data.tools.map(t => ({ ...t, enabled: true }));
    });

    // 使用 untrack 初始化（避免不必要的响应式追踪）
    untrack(() => {
        initLayout(data.project);
        initToolReferences(data.tools);
    });

    // 使用 $effect 响应路由变化
    $effect(() => {
        if (data.project.id !== currentProject?.id) {
            replaceState('', { project: data.project });
        }
    });
</script>
```

### 流程 2：HTTP 服务层

```typescript
// src/lib/services/http.ts
let baseURL = 'http://localhost:8080/api';
if (typeof window !== 'undefined') {
    baseURL = baseURL.replace('http://localhost:8080', window.location.origin);
}

// 通用 GET 请求
async function doGet<T>(url: string, opts?: RequestOptions): Promise<T> {
    const response = await fetch(`${baseURL}${url}`, {
        headers: getAuthHeaders(),
        ...opts,
    });
    if (!response.ok) {
        if (response.status === 401) {
            goto('/login');
        }
        throw new Error(await response.text());
    }
    return response.json();
}
```

### 流程 3：新增页面

1. 在 `src/routes/` 创建目录和 `+page.svelte`
2. 如需数据加载，创建 `+page.ts`（使用 `PageLoad` 类型）
3. 使用 `$props()` 接收 `data`
4. 使用 Context API 初始化共享状态
5. 使用 `$effect` 处理副作用

### 流程 4：新增组件

1. 确定组件职责（单一功能）
2. 使用 `$props()` 定义类型化属性
3. 使用 `$state` / `$derived` 管理内部状态
4. 使用 Tailwind CSS 样式 + `twMerge()` 条件合并
5. 使用 `{#snippet}` 替代 slot

### 流程 5：调试 SSR 问题

```typescript
import { browser, building } from '$app/environment';

// 构建阶段返回静态数据
if (building) {
    return { tools: [], shares: [] };
}

// 仅在浏览器端执行
if (browser) {
    localStorage.setItem('lastVisitedObot', params.project);
}
```

---

## 注意事项

### 关键约束

1. **Runes 语法**：使用 `$state`、`$derived`、`$effect` 而非旧的 `let` 响应式
2. **Blob URL 清理**：`$effect` 返回 cleanup 函数释放 `URL.revokeObjectURL`
3. **SSR 兼容**：检查 `browser`/`building` 环境变量
4. **untrack 初始化**：Context 初始化使用 `untrack()` 避免不必要的响应式追踪
5. **类型安全**：使用 `PageLoad` 类型、`$props<T>()` 泛型
6. **Snippet 替代 Slot**：新代码使用 `{#snippet}` + `{@render}` 模式
7. **twMerge**：条件类名使用 `twMerge()` 而非字符串拼接

### 常见陷阱

| 问题 | 影响 | 解决方案 |
|------|------|----------|
| 忘记 `revokeObjectURL` | 内存泄漏 | `$effect` 返回 cleanup |
| SSR 访问 `localStorage` | 构建失败 | 检查 `browser` 环境变量 |
| `$effect` 中无限循环 | 页面卡死 | 使用 `untrack()` 隔离写操作 |
| 内容替换时动画闪烁 | 用户体验差 | 检测 `startsWith` 并重置光标 |
| Context 未初始化 | 运行时错误 | `hasContext()` 前置检查 |

---

## 反模式

| 反模式 | 正确做法 |
|--------|----------|
| `let count = 0` 旧响应式 | `let count = $state(0)` |
| `<slot />` 旧 slot 模式 | `{#snippet}` + `{@render}` |
| 字符串拼接类名 | `twMerge('base', condition && 'extra')` |
| 直接操作 DOM | Svelte 绑定 + action |
| `any` 类型 | 具体类型 + `$props<T>()` |
| SSR 中访问浏览器 API | `if (browser) { ... }` |
| 大而全的组件 | 按功能拆分 + Context 共享状态 |

---

## 相关 Skills

- [obot-frontend-style](../obot-frontend-style/SKILL.md)：Svelte 前端组件风格与设计规范
- [obot-gateway-security](../obot-gateway-security/SKILL.md)：前端认证集成
- [obot-controller-handler](../obot-controller-handler/SKILL.md)：前后端数据交互
- [obot-api-handler](../obot-api-handler/SKILL.md)：后端 API Handler 设计
