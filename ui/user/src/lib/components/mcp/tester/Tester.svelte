<script lang="ts">
	import { page } from '$app/state';
	import LogsInspector from '$lib/components/mcp/tester/LogsInspector.svelte';
	import PromptsInspector from '$lib/components/mcp/tester/PromptsInspector.svelte';
	import ResourcesInspector from '$lib/components/mcp/tester/ResourcesInspector.svelte';
	import ToolsInspector from '$lib/components/mcp/tester/ToolsInspector.svelte';
	import Loading from '$lib/icons/Loading.svelte';
	import type { MCPCatalogServer } from '$lib/services';
	import {
		MCPTesterSession,
		normalizeTesterSection,
		testerConnectionKey,
		type TesterSection,
		type TesterStatus
	} from '$lib/services/mcp/tester.svelte';
	import { version } from '$lib/stores';
	import { setUrlParamAndUpdateUrl } from '$lib/url';
	import { KeyRound, RotateCw, TriangleAlert } from '@lucide/svelte';
	import { onDestroy, untrack, type Snippet } from 'svelte';
	import { twMerge } from 'tailwind-merge';

	interface Props {
		server?: MCPCatalogServer;
		serverName: string;
		chatAvailable: boolean;
		chatUnavailableMessage: string;
		active?: boolean;
		loading?: boolean;
		icon?: Snippet;
		headerActions?: Snippet;
		placeholder?: Snippet;
		loadingContent?: Snippet;
		accessDeniedAction?: Snippet;
		reauthenticationAction?: Snippet;
		setupRequiredAction?: Snippet;
		unhealthySecondaryAction?: Snippet;
		onStatus?: (status: TesterStatus, error?: string) => void;
	}

	let {
		server,
		serverName,
		active = true,
		loading = false,
		icon,
		headerActions,
		placeholder,
		loadingContent,
		accessDeniedAction,
		reauthenticationAction,
		setupRequiredAction,
		unhealthySecondaryAction,
		onStatus
	}: Props = $props();

	let session = $state<MCPTesterSession>();
	let activeSection = $derived(normalizeTesterSection(page.url.searchParams.get('tab')));
	let statusLabel = $derived(
		server?.deploymentStatus || (server?.configured ? 'Configured' : 'Setup required')
	);

	const sections: Array<{ id: TesterSection; label: string }> = [
		{ id: 'tools', label: 'Tools' },
		{ id: 'prompts', label: 'Prompts' },
		{ id: 'resources', label: 'Resources' },
		{ id: 'logs', label: 'MCP Log' }
	];

	const CARD_CLASS =
		'dark:bg-base-200 dark:border-base-400 bg-base-100 flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-transparent p-4 shadow-sm';

	export function reconnect(): void {
		void session?.initialize(true);
	}

	let mountedConnectionKey: string | undefined;

	$effect(() => {
		const nextKey = active && !loading ? testerConnectionKey(server) : undefined;
		if (nextKey === mountedConnectionKey) {
			return;
		}

		const previousSession = untrack(() => session);
		previousSession?.close();
		mountedConnectionKey = nextKey;

		if (!nextKey) {
			session = undefined;
			return;
		}

		const target = untrack(() => server);
		if (!target) {
			session = undefined;
			return;
		}

		const mountedSession = new MCPTesterSession(
			target,
			{
				name: 'obot-mcp-tester',
				version: untrack(() => version.current.obot || 'unknown')
			},
			fetch
		);
		session = mountedSession;
		void mountedSession.initialize();
	});

	$effect(() => {
		const status = session?.status ?? 'idle';
		const error = session?.error;
		untrack(() => onStatus)?.(status, error);
	});

	onDestroy(() => {
		session?.close();
	});
</script>

<div class="flex h-full min-h-0 flex-col gap-3">
	{#if loading}
		<section
			class="dark:bg-base-200 dark:border-base-400 bg-base-100 flex flex-1 items-center justify-center rounded-lg border border-transparent p-6 shadow-sm"
			aria-live="polite"
		>
			{#if loadingContent}
				{@render loadingContent()}
			{:else}
				<Loading class="size-8" />
			{/if}
		</section>
	{:else if !active}
		{#if placeholder}
			{@render placeholder()}
		{/if}
	{:else}
		<header class="flex shrink-0 flex-wrap items-center justify-between gap-3">
			<div class="flex min-w-0 items-center gap-2">
				{#if icon}
					{@render icon()}
				{/if}
				<h1 class="min-w-0 truncate text-lg font-semibold">{serverName}</h1>
				<p class="shrink-0 text-xs text-muted-content">Status: {statusLabel}</p>
			</div>
			<div class="flex shrink-0 items-center gap-2">
				{#if headerActions}
					{@render headerActions()}
				{/if}
			</div>
		</header>

		<nav
			class="border-base-300 dark:border-base-400 flex shrink-0 overflow-x-auto border-b"
			aria-label="MCP tester sections"
		>
			{#each sections as section (section.id)}
				<button
					class={twMerge(
						'page-tab min-w-fit py-2 text-center font-medium',
						activeSection === section.id && 'page-tab-active'
					)}
					onclick={() => {
						setUrlParamAndUpdateUrl(page.url, 'tab', section.id);
					}}
					aria-current={activeSection === section.id ? 'page' : undefined}
				>
					{section.label}
				</button>
			{/each}
		</nav>

		{#if activeSection === 'logs'}
			<section class={CARD_CLASS}>
				<LogsInspector {session} {serverName} onretry={() => session?.initialize(true)} />
			</section>
		{:else if !session || session.status === 'idle' || session.status === 'initializing'}
			<section
				class="dark:bg-base-200 dark:border-base-400 bg-base-100 rounded-lg border border-transparent p-6 shadow-sm"
				aria-live="polite"
			>
				<h2 class="font-semibold">Connecting to {serverName}</h2>
				<p class="mt-1 text-sm text-muted-content">Opening an MCP session…</p>
			</section>
		{:else if session.status === 'access-denied'}
			<section class="notification-error p-6" role="alert">
				<TriangleAlert class="mb-2 size-5 text-error" aria-hidden="true" />
				<h2 class="font-semibold">Access denied</h2>
				<p class="mt-1 text-sm">
					Your permission to connect to this server is no longer available.
				</p>
				{#if accessDeniedAction}
					<div class="mt-4">
						{@render accessDeniedAction()}
					</div>
				{/if}
			</section>
		{:else if session.status === 'reauthentication-required'}
			<section class="notification-alert p-6" role="status">
				<KeyRound class="mb-2 size-5 text-warning" aria-hidden="true" />
				<h2 class="font-semibold">Reauthentication required</h2>
				<p class="mt-1 text-sm">Reconnect this server before using the tester.</p>
				{#if reauthenticationAction}
					<div class="mt-4">
						{@render reauthenticationAction()}
					</div>
				{/if}
			</section>
		{:else if session.status === 'setup-required'}
			<section class="notification-alert p-6" role="status">
				<TriangleAlert class="mb-2 size-5 text-warning" aria-hidden="true" />
				<h2 class="font-semibold">Server setup required</h2>
				<p class="mt-1 text-sm">Complete the server configuration before using the tester.</p>
				{#if setupRequiredAction}
					<div class="mt-4">
						{@render setupRequiredAction()}
					</div>
				{/if}
			</section>
		{:else if session.status === 'unhealthy' || session.status === 'error'}
			<section class="notification-error p-6" role="alert">
				<TriangleAlert class="mb-2 size-5 text-error" aria-hidden="true" />
				<h2 class="font-semibold">Server unavailable</h2>
				<p class="mt-1 text-sm">{session.error || 'The server is not currently healthy.'}</p>
				<div class="mt-4 flex flex-wrap gap-2">
					<button
						type="button"
						class="btn btn-primary btn-sm"
						onclick={() => session?.initialize(true)}
					>
						<RotateCw class="size-4" aria-hidden="true" /> Retry
					</button>
					{#if unhealthySecondaryAction}
						{@render unhealthySecondaryAction()}
					{/if}
				</div>
			</section>
		{:else if session.status === 'ready'}
			<section class={CARD_CLASS}>
				{#if activeSection === 'tools'}
					<ToolsInspector {session} />
				{:else if activeSection === 'prompts'}
					<PromptsInspector {session} />
				{:else if activeSection === 'resources'}
					<ResourcesInspector {session} />
				{/if}
			</section>
		{/if}
	{/if}
</div>
