<script lang="ts">
	import CopyButton from '../CopyButton.svelte';
	import { ChevronLeft, ChevronRight, KeyRound, ShieldCheck } from 'lucide-svelte';
	import { onMount } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import { twMerge } from 'tailwind-merge';

	interface Props {
		servers: {
			url: string;
			name: string;
		}[];
	}

	let { servers }: Props = $props();
	let scrollContainer: HTMLUListElement;
	let showLeftChevron = $state(false);
	let showRightChevron = $state(false);
	let animationTimeout: ReturnType<typeof setTimeout> | undefined;

	type ConnectOptionKey = 'oauth' | 'apikey' | 'vscode';
	type ConnectOption = {
		label: string;
		description: string;
		instruction: string;
	};

	const optionMap: Record<ConnectOptionKey, ConnectOption> = {
		oauth: {
			label: 'OAuth Clients',
			description:
				'Use this format for MCP clients that support browser-based OAuth. The client will open a browser sign-in flow when authentication is needed.',
			instruction: 'Add this under the MCP configuration section used by your client.'
		},
		apikey: {
			label: 'API Key Clients',
			description:
				'Use this format for clients that cannot complete OAuth and need a static bearer token. Create an MCP-scoped API key, then replace YOUR_API_KEY.',
			instruction: 'Add this under the MCP configuration section used by your client.'
		},
		vscode: {
			label: 'VS Code',
			description:
				'Use this format for VS Code with an MCP-scoped API key. VS Code stores HTTP MCP servers under a top-level servers object.',
			instruction:
				'Add this to .vscode/mcp.json or your VS Code user MCP configuration. VS Code will prompt for the key.'
		}
	};

	const options = (Object.entries(optionMap) as [ConnectOptionKey, ConnectOption][]).map(
		([key, value]) => ({ key, value })
	);
	let selected = $state<ConnectOptionKey>(options[0].key);
	let previousSelected = $state<ConnectOptionKey>(options[0].key);
	let isAnimating = $state(false);
	let flyDirection = $state(100); // 100 for right, -100 for left

	function getFlyDirection(newSelection: ConnectOptionKey, oldSelection: ConnectOptionKey): number {
		const newIndex = options.findIndex((option) => option.key === newSelection);
		const oldIndex = options.findIndex((option) => option.key === oldSelection);

		// If new selection is before old selection, fly from left to right
		// If new selection is after old selection, fly from right to left
		return newIndex < oldIndex ? -100 : 100;
	}

	function checkScrollPosition() {
		if (!scrollContainer) return;

		const { scrollLeft, scrollWidth, clientWidth } = scrollContainer;
		showLeftChevron = scrollLeft > 0;
		showRightChevron = scrollLeft < scrollWidth - clientWidth - 1; // -1 for rounding errors
	}

	function scrollLeft() {
		if (scrollContainer) {
			scrollContainer.scrollBy({ left: -200, behavior: 'smooth' });
		}
	}

	function scrollRight() {
		if (scrollContainer) {
			scrollContainer.scrollBy({ left: 200, behavior: 'smooth' });
		}
	}

	function clearAnimationTimeout() {
		if (animationTimeout) {
			clearTimeout(animationTimeout);
			animationTimeout = undefined;
		}
	}

	function handleSelectionChange(newSelection: ConnectOptionKey) {
		if (newSelection !== selected) {
			previousSelected = selected;
			selected = newSelection;
			flyDirection = getFlyDirection(newSelection, previousSelected);
			isAnimating = true;

			clearAnimationTimeout();

			// Reset animation state after animation completes
			animationTimeout = setTimeout(() => {
				isAnimating = false;
				animationTimeout = undefined;
			}, 300); // Match the CSS animation duration
		}
	}

	function buildConfig(includeApiKey: boolean) {
		const mcpServers = Object.fromEntries(
			servers.map((server) => [
				server.name,
				includeApiKey
					? {
							url: server.url,
							headers: {
								Authorization: 'Bearer YOUR_API_KEY'
							}
						}
					: {
							url: server.url
						}
			])
		);

		return JSON.stringify({ mcpServers }, null, '\t');
	}

	function buildVSCodeConfig() {
		const vscodeServers = Object.fromEntries(
			servers.map((server) => [
				server.name,
				{
					type: 'http',
					url: server.url,
					headers: {
						Authorization: 'Bearer ${input:obot-api-key}'
					}
				}
			])
		);

		return JSON.stringify(
			{
				inputs: [
					{
						type: 'promptString',
						id: 'obot-api-key',
						description: 'Obot API Key',
						password: true
					}
				],
				servers: vscodeServers
			},
			null,
			'\t'
		);
	}

	function buildCodeSnippet(option: ConnectOptionKey) {
		if (option === 'vscode') {
			return buildVSCodeConfig();
		}

		return buildConfig(option === 'apikey');
	}

	onMount(() => {
		checkScrollPosition();
		scrollContainer?.addEventListener('scroll', checkScrollPosition);
		window.addEventListener('resize', checkScrollPosition);

		return () => {
			clearAnimationTimeout();
			scrollContainer?.removeEventListener('scroll', checkScrollPosition);
			window.removeEventListener('resize', checkScrollPosition);
		};
	});
</script>

<div class="flex w-full items-center gap-2">
	<div class="size-4">
		{#if showLeftChevron}
			<button type="button" aria-label="Scroll connection options left" onclick={scrollLeft}>
				<ChevronLeft class="size-4" />
			</button>
		{/if}
	</div>

	<ul
		bind:this={scrollContainer}
		class="default-scrollbar-thin scrollbar-none flex overflow-x-auto"
		style="scroll-behavior: smooth;"
	>
		{#each options as option (option.key)}
			<li class="w-49 flex-shrink-0">
				<button
					type="button"
					aria-pressed={selected === option.key}
					class={twMerge(
						'dark:hover:bg-surface3 relative flex w-full items-center justify-center gap-1.5 rounded-t-xs border-b-2 border-transparent py-2 text-[13px] font-light transition-all duration-200 hover:bg-gray-50',
						selected === option.key &&
							'dark:bg-surface2 bg-background hover:bg-transparent dark:hover:bg-transparent'
					)}
					onclick={() => {
						handleSelectionChange(option.key);
					}}
				>
					{#if option.key === 'oauth'}
						<ShieldCheck class="text-on-surface1 size-5 rounded-sm p-0.5" />
					{:else if option.key === 'apikey'}
						<KeyRound class="text-on-surface1 size-5 rounded-sm p-0.5" />
					{:else}
						<img
							src="/user/images/assistant/vscode-mark.svg"
							alt=""
							class="size-5 rounded-sm p-0.5 dark:bg-gray-600"
						/>
					{/if}
					{option.value.label}

					{#if selected === option.key}
						<div
							class={twMerge(
								'bg-primary absolute right-0 bottom-0 left-0 h-0.5 origin-left',
								isAnimating && selected === option.key ? 'border-slide-in' : ''
							)}
						></div>
					{:else if isAnimating && previousSelected === option.key}
						<div
							class="border-slide-out bg-primary absolute right-0 bottom-0 left-0 h-0.5 origin-left"
						></div>
					{/if}
				</button>
			</li>
		{/each}
	</ul>

	<div class="size-4">
		{#if showRightChevron}
			<button type="button" aria-label="Scroll connection options right" onclick={scrollRight}>
				<ChevronRight class="size-4" />
			</button>
		{/if}
	</div>
</div>

<div class="w-full overflow-hidden">
	<div class="flex min-h-[380px] w-[200%]">
		{#each options as option (option.key)}
			{#if selected === option.key}
				<div
					in:fly={{ x: flyDirection, duration: 200, delay: 200 }}
					out:fade={{ duration: 150 }}
					class="w-1/2 p-4"
				>
					<p>{option.value.description}</p>
					<p class="text-on-surface1 mt-2 text-sm font-light">
						{option.value.instruction}
					</p>
					{@render codeSnippet(buildCodeSnippet(option.key))}
				</div>
			{/if}
		{/each}
	</div>
</div>

{#snippet codeSnippet(code: string)}
	<div class="relative">
		<div class="absolute top-4 right-4 flex h-fit w-fit">
			<CopyButton
				text={code}
				showTextLeft
				tooltipText="Copy MCP configuration"
				class="text-white"
				classes={{ button: 'flex gap-1 flex-shrink-0 items-center text-white' }}
			/>
		</div>
		<pre><code>{code}</code></pre>
	</div>
{/snippet}

<style lang="postcss">
	@keyframes slideOut {
		from {
			transform: scaleX(1);
			opacity: 1;
		}
		to {
			transform: scaleX(0);
			opacity: 0;
		}
	}

	@keyframes slideIn {
		from {
			transform: scaleX(0);
			opacity: 0;
		}
		to {
			transform: scaleX(1);
			opacity: 1;
		}
	}

	.border-slide-out {
		animation: slideOut 0.3s ease-out forwards;
	}

	.border-slide-in {
		animation: slideIn 0.3s ease-out forwards;
	}
</style>
