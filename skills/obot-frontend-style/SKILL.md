---
name: obot-frontend-style
description: |
  Enforce obot's frontend visual style, code style, and best practices.
  Apply when writing or reviewing Svelte components, route pages, or service code in the obot UI.
---

# Obot Frontend Style & Best Practices

## 1. Tech Stack

| Layer | Technology |
|---|---|
| Framework | SvelteKit 5 with Svelte Runes |
| Language | TypeScript (strict) |
| Styling | Tailwind CSS v4 (`@theme {}` in `app.css`) |
| Class merging | `twMerge` from `tailwind-merge` |
| Icons | `lucide-svelte` |
| Floating UI | `@floating-ui/dom` via custom Svelte actions |
| Transitions | `svelte/transition` (`fade`, `slide`) |

---

## 2. CSS Design Token Usage

All tokens are defined in `ui/user/src/app.css`. **Always use tokens — never hardcode colors.**

### Surface tokens

| Token | Pair | Usage |
|---|---|---|
| `--background` | `--on-background` | Page background and default text |
| `--surface1` | `--on-surface1` | Cards, panels |
| `--surface2` | `--on-surface2` | Nested surfaces, secondary panels |
| `--surface3` | `--on-surface3` | Tertiary surfaces |
| `--primary` | — | Primary accent (`--color-blue-500` = `#4f7ef3`) |

Use via Tailwind's arbitrary value syntax or directly in CSS:
```svelte
<div class="bg-[--surface1] text-[--on-surface1]">...</div>
```

### Light / dark theme

- Light mode: tokens defined on `html {}`
- Dark mode: tokens redefined on `.dark {}`, activated by adding `class="dark"` to the root element
- Use Tailwind `dark:` variants for utility overrides
- **Never** check for dark mode in JavaScript — let CSS custom properties handle it

### Font

- Primary font: `Poppins` (via `--font-sans` / `--font-body`), loaded from Google Fonts
- Use Tailwind's `font-sans` utility — do not specify `font-family` manually

---

## 3. Global CSS Class System

These component classes are defined in `@layer components {}` in `app.css`. **Use them instead of composing raw Tailwind utilities for standard UI patterns.**

### Buttons

| Class | Usage |
|---|---|
| `.button` | Neutral / cancel action |
| `.button-primary` | Primary CTA |
| `.button-secondary` | Secondary action |
| `.button-small` | Compact variant (combine with any button class) |
| `.button-destructive` | Destructive confirm inside a dialog |
| `.button-auth` | Auth / login page button |
| `.icon-button` | Icon-only square button |
| `.icon-button-primary` | Primary variant of icon-only button |
| `.menu-button` | Item inside a `DotDotDot` dropdown |
| `.menu-button-destructive` | Destructive dropdown item (red text) |
| `.menu-button-primary` | Accented dropdown item |

### Form inputs

| Class | Usage |
|---|---|
| `.text-input-filled` | Standard filled text input |
| `.text-input` | Outlined text input |
| `.input-label` | `<label>` element for a form field |
| `.input-description` | Subtext description below a field |

### Notification banners

| Class | Usage |
|---|---|
| `.notification-error` | Error state |
| `.notification-alert` | Warning / caution state |
| `.notification-info` | Informational |

### Dialogs

| Class | Usage |
|---|---|
| `.dialog` | `<dialog>` element root |
| `.dialog-container` | Inner content wrapper |
| `.dialog-title` | Title bar text |
| `.dialog-backdrop` | Backdrop overlay |
| `.dialog-close-btn` | Close button inside dialog header |

Use `ResponsiveDialog.svelte` (`$lib/components/ResponsiveDialog.svelte`) as the standard dialog component — it handles mobile sheet vs desktop modal automatically.

### Pills / badges

`.pill-rounded`, `.pill-primary`, `.pill-warning`

---

## 4. Component Authoring Patterns

### Props declaration

Always use `interface Props` + `$props()`. Do **not** use `export let`.

```svelte
<script lang="ts">
  import { twMerge } from 'tailwind-merge';

  interface Props {
    title: string;
    class?: string;
    onclick?: () => void;
  }

  let { title, class: className, onclick }: Props = $props();
</script>
```

### Reactive state (Svelte Runes)

```svelte
let open = $state(false);
let count = $derived(items.length);

$effect(() => {
  // runs when reactive dependencies change
  console.log('open changed', open);
});
```

Do **not** use `writable()` stores or `$:` reactive declarations in components — use runes only.

### Class composition

Use `twMerge` for all conditional/composable class strings:

```svelte
<div class={twMerge('base-classes rounded p-4', className, open && 'ring-2 ring-primary')}>
```

Never concatenate class strings with `+` or template literals.

### Snippets (Svelte 5 slot replacement)

```svelte
<!-- define -->
{#snippet footer()}
  <button class="button-primary">Save</button>
{/snippet}

<!-- render -->
{@render footer()}
```

For optional snippets, accept via props:

```svelte
import type { Snippet } from 'svelte';

interface Props {
  children: Snippet;
  footer?: Snippet;
}
let { children, footer }: Props = $props();
```

### Imperative component API (dialogs)

Expose `open()` / `close()` functions so parents can control via `bind:this`:

```svelte
<script lang="ts">
  let dialogEl = $state<HTMLDialogElement>();

  export function open() {
    dialogEl?.showModal();
  }

  export function close() {
    dialogEl?.close();
  }
</script>
```

Parent usage:
```svelte
let dialogRef = $state<MyDialog>();
<MyDialog bind:this={dialogRef} />
<button onclick={() => dialogRef?.open()}>Open</button>
```

Do **not** use event dispatching (`createEventDispatcher`) for dialog lifecycle control.

### Loading state

```svelte
<script lang="ts">
  import { LoaderCircle } from 'lucide-svelte';
  let loading = $state(false);
</script>

<button class="button-primary" disabled={loading}>
  {#if loading}
    <LoaderCircle class="size-4 animate-spin" />
  {:else}
    Save
  {/if}
</button>
```

### Error handling

```ts
let errorMsg = $state('');

try {
  await AdminService.doSomething();
} catch (err) {
  errorMsg = (err as { message?: string })?.message ?? 'An unexpected error occurred';
}
```

Display errors with `.notification-error`:

```svelte
{#if errorMsg}
  <div class="notification-error p-3 text-sm">{errorMsg}</div>
{/if}
```

### Responsive behavior

```svelte
import { responsive } from '$lib/stores';

{#if responsive.isMobile}
  <!-- mobile layout -->
{:else}
  <!-- desktop layout -->
{/if}
```

### Svelte actions

```svelte
import { tooltip } from '$lib/actions/tooltip.svelte.js';
import { popover } from '$lib/actions/popover.svelte.js';

<button use:tooltip={"Helpful hint"}>...</button>
<div use:popover>...</div>
```

---

## 5. Icon Usage

- Always import from `lucide-svelte`
- Size with Tailwind classes — never set `width`/`height` attributes directly:

| Class | Size | Common use |
|---|---|---|
| `size-4` | 16px | Inline / button icons |
| `size-5` | 20px | Secondary emphasis |
| `size-6` | 24px | Primary / heading icons |

```svelte
import { LoaderCircle, ShieldAlert, Info } from 'lucide-svelte';

<Info class="size-5" />
<LoaderCircle class="size-4 animate-spin" />
```

Never use `<img>` tags for icons.

---

## 6. Service Layer Patterns

- API calls go through `AdminService.*` or `ChatService.*` exported from `$lib/services/index.ts`
- Service implementations live in `$lib/services/admin/operations.ts`
- Types live in `$lib/services/admin/types.ts`
- Constants live in `$lib/services/admin/constants.ts`

New service method template:

```ts
// in operations.ts
export async function updateResource(id: string, payload: ResourcePayload): Promise<Resource> {
  return fetch(`/api/resources/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  }).then(checkResponse).then((r) => r.json());
}
```

Export from the service index so callers use `AdminService.unlockLocalUser(id)`.

Type additions go in `types.ts` as interface extensions:

```ts
export interface OrgUser {
  // ... existing fields
  localAuthDisabled?: boolean;
}
```

---

## 7. Route Page Conventions

- Wrap page content in `<Layout title="...">` from `$lib/components/Layout.svelte`
- Apply page transitions with `in:fade` / `out:fade` using the shared `PAGE_TRANSITION_DURATION` constant:

```svelte
import { fade } from 'svelte/transition';
import { PAGE_TRANSITION_DURATION } from '$lib/constants.js';

const duration = PAGE_TRANSITION_DURATION;

<Layout title="Users">
  <div in:fade={{ duration }} out:fade={{ duration }}>
    <!-- content -->
  </div>
</Layout>
```

- Load initial data in `+page.ts` and receive it via `let { data } = $props()`
- Use `$state(untrack(() => data.something))` for mutable local copies of server-loaded data.
  `untrack()` prevents the `data` prop reactivity from propagating — without it, every time SvelteKit's `data` reference updates (e.g., after a navigation), Svelte would reinitialize `users` and discard local edits:

```svelte
import { untrack } from 'svelte';
let { data } = $props();
let users = $state(untrack(() => data.users));
```

- Add `<svelte:head><title>Page Title</title></svelte:head>` to every route page

---

## 8. File / Directory Naming

| Kind | Convention | Location |
|---|---|---|
| Generic component | `PascalCase.svelte` | `$lib/components/` |
| Feature component | `PascalCase.svelte` | `$lib/components/<feature>/` |
| Route page | `+page.svelte` + `+page.ts` | `src/routes/<path>/` |
| Service operations | `operations.ts` | `$lib/services/<service>/` |
| Service types | `types.ts` | `$lib/services/<service>/` |
| Service constants | `constants.ts` | `$lib/services/<service>/` |
| Svelte actions | `name.svelte.ts` | `$lib/actions/` |
| Stores | `name.svelte.ts` | `$lib/stores/` |

---

## 9. Shared Component Reference

Always reach for these existing components before writing new ones. All are imported from `$lib/components/`.

---

### `ResponsiveDialog` — standard modal / sheet

```svelte
import ResponsiveDialog from '$lib/components/ResponsiveDialog.svelte';

let dialogRef = $state<ResponsiveDialog>();

<!-- open/close imperatively -->
<button onclick={() => dialogRef?.open()}>Open</button>

<ResponsiveDialog
  bind:this={dialogRef}
  title="Edit User"
  class="w-full md:max-w-xl"
  onClose={() => { /* reset local state */ }}
>
  <!-- dialog body -->
  <div class="p-4">...</div>
</ResponsiveDialog>
```

**Key props:**

| Prop | Type | Default | Notes |
|---|---|---|---|
| `title` | `string` | — | Shown in header |
| `titleContent` | `Snippet` | — | Override title with custom markup |
| `class` | `string` | `max-w-2xl` | Size / padding overrides |
| `classes` | `{ header?, content?, title?, closeBtn? }` | — | Fine-grained class overrides |
| `animate` | `'slide' \| 'fade' \| null` | — | Entry animation |
| `hideClose` | `boolean` | `false` | Hide the X button |
| `disableClickOutside` | `boolean` | `false` | Prevent close on backdrop click |
| `onClose` | `() => void` | — | Called on close (including Escape) |
| `onOpen` | `() => void` | — | Called before modal opens |
| `onClickOutside` | `() => void` | — | Override backdrop click behavior |

**Exported methods:** `open()`, `close()`

---

### `Confirm` — confirmation dialog

Use for delete confirmations, destructive actions, and acknowledgements.

```svelte
import Confirm from '$lib/components/Confirm.svelte';

let deletingItem = $state<Item>();

<Confirm
  msg={`Delete "${deletingItem?.name}"?`}
  show={Boolean(deletingItem)}
  onsuccess={async () => {
    await AdminService.deleteItem(deletingItem!.id);
    deletingItem = undefined;
  }}
  oncancel={() => (deletingItem = undefined)}
/>
```

For non-destructive confirmations use `type="info"`:

```svelte
<Confirm
  type="info"
  title="Confirm Action"
  msg="Are you sure?"
  show={showConfirm}
  onsuccess={handleConfirm}
  oncancel={() => (showConfirm = false)}
/>
```

**Key props:**

| Prop | Type | Default | Notes |
|---|---|---|---|
| `show` | `boolean` | `false` | Controls visibility |
| `msg` | `string` | `'OK?'` | Main message text |
| `title` | `string` | `'Confirm Delete'` | Dialog title |
| `type` | `'delete' \| 'info'` | `'delete'` | Controls icon and button color |
| `onsuccess` | `() => void` | required | Confirmed callback |
| `oncancel` | `() => void` | required | Cancelled / closed callback |
| `loading` | `boolean` | — | Shows spinner on confirm button |
| `note` | `Snippet \| string` | `'This action is permanent...'` | Secondary note text or markup |
| `msgContent` | `Snippet` | — | Replace icon+msg area entirely |
| `disabled` | `boolean` | — | Disable confirm button |

---

### `DotDotDot` — action dropdown menu

The standard three-dot (⋮) context menu. Children receive a `toggle` function.

```svelte
import DotDotDot from '$lib/components/DotDotDot.svelte';

<DotDotDot>
  <button class="menu-button" onclick={() => handleEdit(item)}>Edit</button>
  <button class="menu-button" onclick={() => handleDuplicate(item)}>Duplicate</button>
  <button class="menu-button-destructive" onclick={() => (deletingItem = item)}>
    Delete
  </button>
</DotDotDot>
```

Custom trigger icon:

```svelte
<DotDotDot>
  {#snippet icon()}<Settings class="size-4" />{/snippet}
  <button class="menu-button">...</button>
</DotDotDot>
```

**Key props:**

| Prop | Type | Default | Notes |
|---|---|---|---|
| `children` | `Snippet<[{ toggle }]>` | required | Menu item buttons |
| `icon` | `Snippet` | `EllipsisVertical` | Custom trigger icon |
| `class` | `string` | `'icon-button'` | Trigger button class |
| `placement` | `Placement` | `'right-start'` | Floating UI placement |
| `classes` | `{ menu?, popover? }` | — | Menu/popover class overrides |
| `onClick` | `() => void` | — | Extra callback on trigger click |

---

### `Table` — data table with sorting, filtering, pagination

Requires items to have an `id: string | number` field.

```svelte
import Table from '$lib/components/table/Table.svelte';

<Table
  data={tableData}
  fields={['name', 'email', 'role', 'created']}
  sortable={['name', 'email', 'created']}
  filterable={['role']}
  headers={[
    { title: 'Full Name', property: 'name' },
    { title: 'Created At', property: 'created' }
  ]}
  initSort={{ property: 'created', order: 'desc' }}
  noDataMessage="No users found."
  onSort={setSortUrlParams}
  onFilter={setFilterUrlParams}
  onClearAllFilters={clearUrlParams}
>
  {#snippet onRenderColumn(property, d)}
    {#if property === 'created'}
      {formatTimeAgo(d.created, 'day').relativeTime}
    {:else}
      {d[property]}
    {/if}
  {/snippet}

  {#snippet actions(d)}
    <DotDotDot>
      <button class="menu-button" onclick={() => handleEdit(d)}>Edit</button>
      <button class="menu-button-destructive" onclick={() => (deletingItem = d)}>Delete</button>
    </DotDotDot>
  {/snippet}
</Table>
```

**Key props:**

| Prop | Type | Notes |
|---|---|---|
| `data` | `T[]` | Items (must have `id`) |
| `fields` | `string[]` | Property keys to display as columns |
| `headers` | `{ title, property, tooltip? }[]` | Override column header labels |
| `sortable` | `string[]` | Fields that can be sorted |
| `filterable` | `string[]` | Fields that show filter dropdown |
| `initSort` | `{ property, order }` | Initial sort state |
| `filters` | `Record<string, (string\|number)[]>` | Initial filter state |
| `pageSize` | `number` | Enable pagination |
| `noDataMessage` | `string` | Empty state text |
| `onRenderColumn` | `Snippet<[string, T]>` | Custom cell rendering |
| `actions` | `Snippet<[T]>` | Per-row action column |
| `onClickRow` | `(row, isCtrlClick) => void` | Row click handler |
| `setRowClasses` | `(row) => string` | Dynamic row CSS classes |
| `onSort` | `(property, order) => void` | Persist sort state (e.g. URL) |
| `onFilter` | `(property, values) => void` | Persist filter state |

**Exported method:** `clearSelectAll()`

---

### `Search` — debounced search input

```svelte
import Search from '$lib/components/Search.svelte';
import { debounce } from 'es-toolkit';

let query = $state('');
const updateQuery = debounce((value: string) => { query = value; }, 100);

<Search
  value={query}
  placeholder="Search by name or email..."
  class="flex-1 border border-transparent shadow-sm"
  onChange={updateQuery}
/>
```

**Key props:**

| Prop | Type | Default | Notes |
|---|---|---|---|
| `onChange` | `(value: string) => void` | required | Called after 300 ms debounce |
| `value` | `string` | `''` | Controlled value |
| `placeholder` | `string` | `'Search Projects...'` | Input placeholder |
| `compact` | `boolean` | `false` | Smaller padding variant |

**Exported method:** `clear()` — resets the input and calls `onChange('')`

---

### `Toggle` — boolean switch

```svelte
import Toggle from '$lib/components/Toggle.svelte';

let enabled = $state(false);

<!-- icon-only mode (label shown as tooltip) -->
<Toggle
  label="Enable feature"
  checked={enabled}
  onChange={(v) => (enabled = v)}
/>

<!-- inline label visible -->
<Toggle
  label="Enable feature"
  labelInline
  checked={enabled}
  onChange={(v) => (enabled = v)}
/>
```

**Key props:**

| Prop | Type | Notes |
|---|---|---|
| `label` | `string` | Required; shown as tooltip or inline text |
| `checked` | `boolean` | Current state |
| `onChange` | `(checked: boolean) => void` | Change callback |
| `labelInline` | `boolean` | Show label text next to toggle |
| `disabled` | `boolean` | Disable interaction |

---

### `Select` — searchable dropdown select

Options must have `{ id: string | number; label: string }`.

```svelte
import Select from '$lib/components/Select.svelte';

let selectedRole = $state<number>();
const roleOptions = [
  { id: 1, label: 'Admin' },
  { id: 2, label: 'Basic User' }
];

<Select
  options={roleOptions}
  bind:selected={selectedRole}
  placeholder="Select a role"
  onSelect={(opt) => { selectedRole = opt.id as number; }}
/>
```

**Key props:**

| Prop | Type | Notes |
|---|---|---|
| `options` | `T[]` | Must have `id` + `label` |
| `selected` | `string \| number` | Bindable selected ID |
| `onSelect` | `(option, value?) => void` | Selection callback |
| `multiple` | `boolean` | Multi-select mode |
| `searchable` | `boolean` | Show search input in dropdown |
| `placeholder` | `string` | Trigger button placeholder |
| `disabled` / `readonly` | `boolean` | Interaction control |
| `position` | `'top' \| 'bottom'` | Dropdown direction |
| `onClear` / `onClearAll` | `() => void` | Clear callbacks |

---

### `CopyButton` — clipboard copy

```svelte
import CopyButton from '$lib/components/CopyButton.svelte';

<!-- icon-only (shows tooltip "Copy" / "Copied!") -->
<CopyButton text={apiKey} />

<!-- with visible button text -->
<CopyButton text={apiKey} buttonText="Copy Key" />
```

**Key props:**

| Prop | Type | Default | Notes |
|---|---|---|---|
| `text` | `string` | — | Text to copy; hides button if empty |
| `tooltipText` | `string` | `'Copy'` | Tooltip shown before copy |
| `buttonText` | `string` | — | Visible label (uses pill style) |
| `showTextLeft` | `boolean` | `false` | Show label left of icon |
| `disabled` | `boolean` | — | Disable button |

---

### `InfoTooltip` — inline help icon with tooltip

```svelte
import InfoTooltip from '$lib/components/InfoTooltip.svelte';

<div class="flex items-center gap-1">
  <span>API Rate Limit</span>
  <InfoTooltip text="Max requests per minute allowed for this key." />
</div>

<!-- wider tooltip -->
<InfoTooltip
  text="Long explanation text goes here..."
  popoverWidth="lg"
  placement="top"
/>
```

**Key props:**

| Prop | Type | Default | Notes |
|---|---|---|---|
| `text` | `string` | required | Tooltip content |
| `popoverWidth` | `'sm' \| 'md' \| 'lg'` | `'md'` | `48 / 64 / 96` Tailwind width |
| `placement` | `Placement` | auto | Floating UI placement |

---

### `Layout` — admin page shell

Wrap every admin route page in `Layout`. It renders the sidebar nav, header, and content area.

```svelte
import Layout from '$lib/components/Layout.svelte';

<Layout title="Users">
  <div in:fade={{ duration }} out:fade={{ duration }}>
    <!-- page content -->
  </div>
</Layout>
```

---

## 10. Do / Don't

| Do | Don't |
|---|---|
| Use design token CSS vars (`--surface1`, `--primary`) | Hardcode hex/rgb colors or raw `bg-blue-500` |
| Use `.button`, `.button-primary`, `.menu-button`, etc. | Compose button styles from scratch with raw Tailwind |
| Use `twMerge()` for all class merging | Concatenate class strings with `+` or template literals |
| Use `lucide-svelte` icons with `size-4/5/6` classes | Use `<img>` for icons or set `width`/`height` directly |
| Use `ResponsiveDialog` for all dialogs | Write custom `<dialog>` markup without the component |
| Use `DotDotDot` + `.menu-button` for action menus | Create custom dropdown implementations |
| Use `$state`, `$derived`, `$effect`, `$props` (Runes) | Use `writable()` stores or `$:` in components |
| Declare props with `interface Props {}` + `$props()` | Use `export let` |
| Export `open()`/`close()` for dialog components | Dispatch events for dialog lifecycle control |
| Handle errors with the `(err as { message?: string })?.message` pattern | Access `.message` without a type cast |
| Use `dark:` Tailwind variants for theme overrides | Check dark mode in JavaScript |
| Use `AdminService.*` / `ChatService.*` for API calls | Call `fetch()` directly inside components |
