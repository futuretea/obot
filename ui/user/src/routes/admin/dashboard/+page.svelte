<script lang="ts">
	import { resolve } from '$app/paths';
	import DotDotDot from '$lib/components/DotDotDot.svelte';
	import Layout from '$lib/components/Layout.svelte';
	import TweenedMetric from '$lib/components/TweenedMetric.svelte';
	import McpServerGitSync from '$lib/components/admin/McpServerGitSync.svelte';
	import {
		transformTopToolCalls,
		transformTopServerUsage,
		transformAvgToolCallResponseTime
	} from '$lib/components/admin/usage/utils';
	import DonutGraph from '$lib/components/graph/DonutGraph.svelte';
	import HorizontalBarGraph from '$lib/components/graph/HorizontalBarGraph.svelte';
	import SelectServerType from '$lib/components/mcp/SelectServerType.svelte';
	import { DEFAULT_MCP_CATALOG_ID } from '$lib/constants';
	import { formatNumber } from '$lib/format';
	import Loading from '$lib/icons/Loading.svelte';
	import { stripMarkdownToText } from '$lib/markdown';
	import {
		AdminService,
		type AuditLogUsageStats,
		type LaunchServerType,
		type MCPCatalogEntry,
		type MCPCatalogServer,
		type OrgUser,
		type TotalTokenUsage
	} from '$lib/services';
	import { errors, mcpServersAndEntries, profile } from '$lib/stores';
	import { goto } from '$lib/url';
	import { isWithinInterval, set, subMonths } from 'date-fns';
	import { Activity, ChevronRight, Coins, Plus, Server, Users, Wrench } from 'lucide-svelte';
	import { onMount } from 'svelte';
	import { fade } from 'svelte/transition';

	const TOP_TOOLS_LIMIT = 5;
	const TOP_SERVERS_LIMIT = 12;

	let loading = $state(true);
	let loadingToolUsage = $state(true);

	let usersData = $state<OrgUser[]>([]);
	let totalTokensData = $state<TotalTokenUsage>();

	let selectServerTypeDialog = $state<ReturnType<typeof SelectServerType>>();
	let sourceDialog = $state<ReturnType<typeof McpServerGitSync>>();

	type TopToolCallRow = {
		compositeKey: string;
		toolLabel: string;
		count: number;
		serverDisplayName: string;
	};

	type TopServerUsageRow = { serverName: string; count: number };

	type AvgToolCallResponseTimeRow = {
		toolName: string;
		averageResponseTimeMs: number;
		serverDisplayName: string;
	};

	let topToolCalls = $state<TopToolCallRow[]>([]);
	let topServerUsage = $state<TopServerUsageRow[]>([]);
	let avgToolCallResponseTime = $state<AvgToolCallResponseTimeRow[]>([]);

	const end = new Date();
	const start = subMonths(end, 1);

	function topToolCallsFromStats(stats: AuditLogUsageStats | undefined): TopToolCallRow[] {
		return transformTopToolCalls(stats)
			.map((t) => ({
				compositeKey: t.toolName,
				toolLabel: t.toolName,
				count: t.count,
				serverDisplayName: t.serverDisplayName
			}))
			.filter(
				(t) => !t.serverDisplayName.startsWith('nba1') && !t.serverDisplayName.startsWith('Obot ')
			);
	}

	function topServersFromStats(stats: AuditLogUsageStats | undefined): TopServerUsageRow[] {
		return transformTopServerUsage(stats).filter(
			(s) => !s.serverName.startsWith('nba1') && !s.serverName.startsWith('Obot ')
		);
	}

	function avgToolCallResponseTimeFromStats(stats: AuditLogUsageStats | undefined) {
		return transformAvgToolCallResponseTime(stats).filter(
			(t) => !t.serverDisplayName.startsWith('nba1') && !t.serverDisplayName.startsWith('Obot ')
		);
	}

	let monthlyActiveUsers = $derived(
		usersData.filter(
			(user) => user.lastActiveDay && isWithinInterval(new Date(user.lastActiveDay), { start, end })
		).length
	);

	let deployedCatalogEntryServers = $state<MCPCatalogServer[]>([]);
	let deployedWorkspaceCatalogEntryServers = $state<MCPCatalogServer[]>([]);
	let serversData = $derived(
		mcpServersAndEntries.current.loading || loading
			? []
			: [
					...deployedCatalogEntryServers.filter((server) => !server.deleted),
					...deployedWorkspaceCatalogEntryServers.filter((server) => !server.deleted),
					...mcpServersAndEntries.current.servers.filter((server) => !server.deleted)
				]
	);

	function compileServerAndEntries(data: MCPCatalogServer[], entries: MCPCatalogEntry[]) {
		const entriesMap = new Map(entries.map((e) => [e.id, e]));
		const catalogEntriesCount = data.reduce<
			Record<
				string,
				{
					entry?: MCPCatalogEntry | undefined;
					server?: MCPCatalogServer | undefined;
					count: number;
					id: string;
				}
			>
		>((acc, server) => {
			if (!server.catalogEntryID) {
				acc[server.id] = {
					server,
					count: 1,
					id: server.id
				};
				return acc;
			}
			if (!acc[server.catalogEntryID]) {
				const entry = entriesMap.get(server.catalogEntryID);
				if (!entry) return acc;
				acc[server.catalogEntryID] = {
					entry,
					count: 0,
					id: entry.id
				};
			}
			acc[server.catalogEntryID].count++;
			return acc;
		}, {});
		const sortByCountDescending = Object.values(catalogEntriesCount).sort(
			(a, b) => b.count - a.count
		);

		const entryTypes = data.reduce(
			(acc, server) => {
				if (!server.catalogEntryID) acc.multi++;
				else if (server.manifest.runtime === 'composite') acc.composite++;
				else if (server.manifest.runtime === 'remote') acc.remote++;
				else acc.single++;
				return acc;
			},
			{
				multi: 0,
				single: 0,
				remote: 0,
				composite: 0
			}
		);

		return {
			graphData: [
				{
					label: 'Multi-User',
					value: entryTypes.multi
				},
				{
					label: 'Single-User',
					value: entryTypes.single
				},
				{
					label: 'Remote',
					value: entryTypes.remote
				},
				{
					label: 'Composite',
					value: entryTypes.composite
				}
			],
			popularServers: sortByCountDescending.filter((s) => s.count > 0).slice(0, 5),
			totalServers: data.length
		};
	}

	const serverAndEntries = $derived(mcpServersAndEntries.current);
	const { graphData, popularServers, totalServers } = $derived(
		compileServerAndEntries(serversData, serverAndEntries.entries)
	);
	let isBootStrapUser = $derived(profile.current.isBootstrapUser?.() ?? false);

	onMount(async () => {
		const endToolStats = set(new Date(), { milliseconds: 0, seconds: 59 });
		const startToolStats = subMonths(endToolStats, 1);

		AdminService.listAuditLogUsageStats({
			start_time: startToolStats.toISOString(),
			end_time: endToolStats.toISOString()
		})
			.then((stats) => {
				topToolCalls = topToolCallsFromStats(stats).slice(0, TOP_TOOLS_LIMIT);
				topServerUsage = topServersFromStats(stats).slice(0, TOP_SERVERS_LIMIT);
				avgToolCallResponseTime = avgToolCallResponseTimeFromStats(stats).slice(0, TOP_TOOLS_LIMIT);
			})
			.catch((error) => {
				if (error?.name === 'AbortError') return;
				errors.append(error);
			})
			.finally(() => {
				loadingToolUsage = false;
			});

		const [users, tokens, catalogServers, workspaceServers] = await Promise.all([
			AdminService.listUsersIncludeDeleted(),
			AdminService.listTotalTokenUsage(),
			AdminService.listAllCatalogDeployedSingleRemoteServers(DEFAULT_MCP_CATALOG_ID),
			AdminService.listAllWorkspaceDeployedSingleRemoteServers()
		]);

		usersData = users;
		totalTokensData = tokens;
		deployedCatalogEntryServers = catalogServers;
		deployedWorkspaceCatalogEntryServers = workspaceServers;
		loading = false;
	});

	function handleSelectServerType(type: LaunchServerType) {
		selectServerTypeDialog?.close();
		goto(resolve(`/admin/mcp-servers?new=${type}`));
	}

	function getServerUrl(server: MCPCatalogServer) {
		if (server.powerUserWorkspaceID) {
			return `/admin/mcp-servers/w/${server.powerUserWorkspaceID}/s/${server.id}?view=audit-logs`;
		}
		return `/admin/mcp-servers/s/${server.id}?view=audit-logs`;
	}

	function getEntryUrl(entry: MCPCatalogEntry) {
		if (entry.powerUserWorkspaceID) {
			return `/admin/mcp-servers/w/${entry.powerUserWorkspaceID}/c/${entry.id}?view=audit-logs`;
		}
		return `/admin/mcp-servers/c/${entry.id}?view=audit-logs`;
	}
</script>

<Layout title="Dashboard" classes={{ childrenContainer: 'max-w-none', container: '' }}>
	<div class="@container grid grid-cols-12 gap-4">
		<div class="col-span-12 flex flex-col gap-4 @min-[768px]:col-span-8">
			<!-- this token usage graph-->
			<div class="grid grid-cols-12 gap-4">
				<div class="col-span-12 @min-[768px]:col-span-4">
					<div class="paper h-full gap-2">
						<div class="text-on-surface1 flex items-center gap-1 text-xs">Total Users</div>
						<div class="flex w-full justify-between">
							{#if loading}
								<Loading class="size-8" />
							{:else}
								<div class="text-3xl font-semibold">
									<TweenedMetric holdAtZero={loading} target={usersData.length} />
								</div>
								<Users class="text-primary size-8" />
							{/if}
						</div>
						{#if !isBootStrapUser}
							<a
								href={resolve('/admin/users')}
								class="bg-surface3/50 hover:bg-surface3 flex w-fit translate-x-2 items-center gap-1 self-end rounded-md px-2 py-0.5 text-[11px] transition-colors duration-200"
							>
								See More <ChevronRight class="size-3" />
							</a>
						{/if}
					</div>
				</div>
				<div class="col-span-12 @min-[768px]:col-span-4">
					<div class="paper h-full gap-2">
						<div class="text-on-surface1 flex items-center gap-1 text-xs">Monthly Active Users</div>
						<div class="flex w-full justify-between">
							{#if loading}
								<Loading class="size-8" />
							{:else}
								<div class="text-3xl font-semibold">
									<TweenedMetric holdAtZero={loading} target={monthlyActiveUsers} />
								</div>
								<Activity class="text-primary size-8" />
							{/if}
						</div>
						<div class="text-on-surface1 text-xs">Last 30 Days</div>
					</div>
				</div>
				<div class="col-span-12 @min-[768px]:col-span-4">
					<div class="paper h-full gap-2">
						<div class="text-on-surface1 flex items-center gap-1 text-xs">Total Tokens</div>
						<div class="flex w-full justify-between">
							{#if loading}
								<Loading class="size-8" />
							{:else}
								<div class="text-3xl font-semibold">
									<TweenedMetric
										holdAtZero={loading}
										target={totalTokensData?.totalTokens ?? 0}
										format={(n) => formatNumber(Math.max(0, Math.round(n)))}
									/>
								</div>
								<Coins class="text-primary size-8" />
							{/if}
						</div>
						{#if !isBootStrapUser}
							<a
								href={resolve('/admin/token-usage')}
								class="bg-surface3/50 hover:bg-surface3 flex w-fit translate-x-2 items-center gap-1 self-end rounded-md px-2 py-0.5 text-[11px] transition-colors duration-200"
							>
								See More <ChevronRight class="size-3" />
							</a>
						{/if}
					</div>
				</div>
			</div>
			{#if loadingToolUsage}
				<div class="bg-surface3 h-[400px] animate-pulse rounded-md"></div>
			{:else}
				<div in:fade={{ duration: 150 }} class="paper min-h-72 w-full gap-1">
					<div class="flex flex-wrap items-center justify-between gap-4">
						<h4 class="flex items-center gap-1 font-semibold">
							Top Servers Used <span class="text-on-surface1 text-xs font-light"
								>(Last 30 Days)</span
							>
						</h4>
					</div>
					<HorizontalBarGraph
						data={topServerUsage}
						labelKey="serverName"
						valueKey="count"
						formatValue={(value) => Math.round(value).toString()}
						class="h-[400px]"
					>
						{#snippet tooltipContent(item)}
							<div class="flex flex-col gap-0 text-xs">
								<div class="text-on-surface1 text-xs">{item.label}</div>
							</div>
							<div class="text-on-background font-semibold">
								{item.value} calls
							</div>
						{/snippet}
					</HorizontalBarGraph>
				</div>
			{/if}

			<div class="grid grow grid-cols-12 gap-4">
				<div class="paper col-span-12 flex h-full min-h-72 flex-col gap-1 @min-[768px]:col-span-6">
					<h4 class="mb-1 flex items-center gap-2 font-semibold">
						Recently Popular Tools
						<span class="text-on-surface1 text-xs font-light">(Last 30 Days)</span>
					</h4>
					{#if loadingToolUsage}
						<div class="flex w-full flex-col gap-4 pt-2">
							{#each Array.from({ length: TOP_TOOLS_LIMIT }) as _, i (i)}
								<div class="flex w-full animate-pulse items-center gap-2">
									<div class="bg-surface3 size-8 shrink-0 rounded-md"></div>
									<div class="flex flex-1 flex-col gap-2">
										<div class="bg-surface3 h-4 w-full rounded"></div>
										<div class="bg-surface3 h-3 w-full rounded"></div>
									</div>
								</div>
							{/each}
						</div>
					{:else if topToolCalls.length === 0}
						<p
							class="text-on-surface1 flex h-full grow items-center justify-center pt-2 text-center text-xs font-light"
						>
							No recent tool calls.
						</p>
					{:else}
						<ul class="flex flex-col gap-2 pt-2">
							{#each topToolCalls as row (row.compositeKey)}
								<li class="flex items-center gap-2">
									<div
										class="bg-surface1 dark:bg-surface2 size-8 shrink-0 items-center justify-center rounded-md p-1"
									>
										<Wrench class="size-6 shrink-0 opacity-65" />
									</div>
									<div class="flex min-w-0 flex-col gap-1">
										<p class="truncate text-sm font-medium">
											{row.toolLabel.split('.').slice(1).join('.') || row.compositeKey}
										</p>
										<p class="text-on-surface1 text-xs">
											{formatNumber(row.count)} calls · {row.serverDisplayName}
										</p>
									</div>
								</li>
							{/each}
						</ul>
					{/if}
					<div class="flex min-h-0 grow"></div>
					{#if topToolCalls.length > 0 && !isBootStrapUser}
						<a
							href={resolve('/admin/usage')}
							class="bg-surface3/50 hover:bg-surface3 mt-2 flex w-fit translate-x-2 items-center gap-1 self-end rounded-md px-2 py-0.5 text-[11px] transition-colors duration-200"
						>
							See More <ChevronRight class="size-3" />
						</a>
					{/if}
				</div>
				<div class="paper col-span-12 flex h-full min-h-72 flex-col gap-1 @min-[768px]:col-span-6">
					<h4 class="mb-1 flex items-center gap-2 font-semibold">
						Tool Call Average Response Time
						<span class="text-on-surface1 text-xs font-light">(Last 30 Days)</span>
					</h4>
					{#if loadingToolUsage}
						<div class="flex w-full flex-col gap-4 pt-2">
							{#each Array.from({ length: TOP_TOOLS_LIMIT }) as _, i (i)}
								<div class="flex w-full animate-pulse items-center gap-2">
									<div class="flex flex-1 flex-col gap-2">
										<div class="bg-surface3 h-4 w-full rounded"></div>
										<div class="bg-surface3 h-3 w-full rounded"></div>
									</div>
								</div>
							{/each}
						</div>
					{:else if avgToolCallResponseTime.length === 0}
						<p
							class="text-on-surface1 flex h-full grow items-center justify-center pt-2 text-center text-xs font-light"
						>
							No recent tool calls.
						</p>
					{:else}
						<div class="flex w-full flex-col gap-4 pt-2">
							<ul class="flex flex-col gap-2">
								{#each avgToolCallResponseTime as row (row.toolName)}
									<li class="flex items-center gap-2">
										<div class="flex min-w-0 grow flex-col gap-1 pr-4">
											<p class="truncate text-sm font-medium">
												{row.toolName.split('.').slice(1).join('.')}
											</p>
											<p class="text-on-surface1 text-xs">
												{row.serverDisplayName}
											</p>
										</div>
										<div class="text-sm">
											{row.averageResponseTimeMs}ms
										</div>
									</li>
								{/each}
							</ul>
						</div>
					{/if}
					<div class="flex min-h-0 grow"></div>
					{#if avgToolCallResponseTime.length > 0 && !isBootStrapUser}
						<a
							href={resolve('/admin/usage')}
							class="bg-surface3/50 hover:bg-surface3 mt-2 flex w-fit translate-x-2 items-center gap-1 self-end rounded-md px-2 py-0.5 text-[11px] transition-colors duration-200"
						>
							See More <ChevronRight class="size-3" />
						</a>
					{/if}
				</div>
			</div>
		</div>
		<div class="col-span-12 flex flex-col gap-4 @min-[768px]:col-span-4">
			{#if serverAndEntries.loading || loading}
				<div class="bg-surface3 h-[530px] animate-pulse rounded-md"></div>
				<div class="paper flex grow gap-1">
					<h4 class="flex items-center gap-2 font-semibold">Most Popular Servers</h4>
					<div class="flex flex-col gap-4 pt-2">
						{#each Array.from({ length: 5 }) as _, i (i)}
							<div class="flex w-full animate-pulse items-center gap-2">
								<div class="bg-surface3 size-8 shrink-0 rounded-md"></div>
								<div class="flex flex-1 flex-col gap-2">
									<div class="bg-surface3 h-4 w-full rounded"></div>
									<div class="bg-surface3 h-3 w-full rounded"></div>
								</div>
							</div>
						{/each}
					</div>
				</div>
			{:else}
				<div in:fade={{ duration: 150 }} class="paper min-h-96">
					{#if deployedCatalogEntryServers.length > 0 || deployedWorkspaceCatalogEntryServers.length > 0 || isBootStrapUser}
						<h4 class="font-semibold">Active Servers</h4>
						<div class="mb-2 flex flex-col items-center justify-center gap-2">
							<div class="flex w-full items-center justify-center gap-2">
								<div class="text-3xl font-semibold">
									<TweenedMetric holdAtZero={serverAndEntries.loading} target={totalServers} />
								</div>
								<Server class="text-primary size-8" />
							</div>
							<div class="text-xs">Total Currently Active</div>
						</div>

						<div class="bg-surface2 mb-4 h-px w-full"></div>

						<div class="flex h-72 flex-col items-center justify-center">
							{#if graphData.some((g) => g.value > 0)}
								<DonutGraph class="h-72" donutRatio={0.65} data={graphData} />
							{:else}
								<p class="text-on-surface1 pt-2 text-center text-xs font-light">
									No servers have been deployed yet.
								</p>
							{/if}
						</div>

						{#if !isBootStrapUser}
							<div class="flex justify-end">
								<a
									href={resolve('/admin/mcp-servers?view=deployments')}
									class="bg-surface3/50 hover:bg-surface3 flex w-fit translate-x-2 items-center gap-1 self-end rounded-md px-2 py-0.5 text-[11px] transition-colors duration-200"
								>
									See More <ChevronRight class="size-3" />
								</a>
							</div>
						{/if}
					{:else}
						<div class="flex grow flex-col items-center justify-center gap-4">
							<div>
								<p class="mb-1 text-center text-sm">
									Looks like you don't have any servers created yet.
								</p>
								<p class="text-center text-sm">Click below to get started!</p>
							</div>
							<DotDotDot
								class="button-primary w-full self-center text-sm md:w-fit"
								placement="bottom"
							>
								{#snippet icon()}
									<span class="flex items-center justify-center gap-1">
										<Plus class="size-4" /> Add MCP Server
									</span>
								{/snippet}
								<button
									class="menu-button"
									onclick={() => {
										selectServerTypeDialog?.open();
									}}
								>
									Add server
								</button>
								<button
									class="menu-button"
									onclick={() => {
										sourceDialog?.open();
									}}
								>
									Add server(s) from Git
								</button>
							</DotDotDot>
						</div>
					{/if}
				</div>
				<div in:fade={{ duration: 150 }} class="paper flex grow gap-1">
					<h4 class="flex items-center gap-2 font-semibold">Most Deployed Servers</h4>
					{#if mcpServersAndEntries.current.loading || loading}
						<div class="flex flex-col gap-4 pt-2">
							{#each Array.from({ length: 5 }) as _, i (i)}
								<div class="flex w-full animate-pulse items-center gap-2">
									<div class="bg-surface3 size-8 shrink-0 rounded-md"></div>
									<div class="flex flex-1 flex-col gap-2">
										<div class="bg-surface3 h-4 w-full rounded"></div>
										<div class="bg-surface3 h-3 w-full rounded"></div>
									</div>
								</div>
							{/each}
						</div>
					{:else if popularServers.length > 0}
						<div class="flex flex-col gap-2 pt-2">
							{#each popularServers as info (info.id)}
								{@const icon =
									'server' in info ? info.server?.manifest.icon : info.entry?.manifest.icon}
								{@const displayName =
									'server' in info
										? (info.server?.alias ?? info.server?.manifest.name)
										: info.entry?.manifest.name}
								{@const description =
									'server' in info
										? info.server?.manifest.description
										: info.entry?.manifest.description}
								{@const url = info.server
									? getServerUrl(info.server)
									: info.entry
										? getEntryUrl(info.entry)
										: undefined}
								<a
									class="dark:hover:bg-surface2 hover:bg-surface1 -mx-2 flex items-center gap-2 rounded-md px-2 py-1 transition-colors duration-150"
									href={url ? resolve(url as `/${string}`) : undefined}
								>
									{#if icon}
										<img
											src={icon}
											alt={info.id}
											class="bg-surface1 dark:bg-surface2 size-9 rounded-md p-1"
										/>
									{:else}
										<Server class="bg-surface1 size-9 rounded-md p-1 opacity-65" />
									{/if}
									<div class="flex max-w-[calc(100%-4.5rem)] flex-col gap-0.5">
										<p class="text-sm font-medium">{displayName}</p>
										{#if description}
											<p class="line-clamp-1 truncate text-xs font-light break-all">
												{stripMarkdownToText(description ?? '')}
											</p>
										{/if}
										<p class="text-on-surface1 text-xs italic">Deployed {info.count} times</p>
									</div>
									<ChevronRight class="size-5 shrink-0" />
								</a>
							{/each}
						</div>
					{:else}
						<p
							class="text-on-surface1 flex h-full items-center justify-center pt-2 text-center text-xs font-light"
						>
							No servers have been deployed yet.
						</p>
					{/if}
					<div class="flex grow"></div>
					{#if popularServers.length > 0 && !isBootStrapUser}
						<a
							href={resolve('/admin/mcp-servers')}
							class="bg-surface3/50 hover:bg-surface3 flex w-fit translate-x-2 items-center justify-end gap-1 self-end rounded-md px-2 py-0.5 text-[11px] transition-colors duration-200"
						>
							See More <ChevronRight class="size-3" />
						</a>
					{/if}
				</div>
			{/if}
		</div>
	</div>
</Layout>

<McpServerGitSync
	bind:this={sourceDialog}
	onSync={async () => {
		await AdminService.refreshMCPCatalog(DEFAULT_MCP_CATALOG_ID);
		goto('/admin/mcp-servers');
	}}
	defaultCatalogId={DEFAULT_MCP_CATALOG_ID}
/>
<SelectServerType bind:this={selectServerTypeDialog} onSelectServerType={handleSelectServerType} />

<svelte:head>
	<title>Dashboard</title>
</svelte:head>
