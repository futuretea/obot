<script lang="ts">
	import { ChevronLeft, ChevronRight } from 'lucide-svelte';
	import { onMount } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import { twMerge } from 'tailwind-merge';
	import CopyButton from '../CopyButton.svelte';

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

	const optionMap: Record<string, { label: string; icon: string }> = {
		oauth: {
			label: 'OAuth Clients',
			icon: '/user/images/assistant/oauth-mark.svg'
		},
		apikey: {
			label: 'API Key Clients',
			icon: '/user/images/assistant/apikey-mark.svg'
		},
		vscode: {
			label: 'VSCode',
			icon: '/user/images/assistant/vscode-mark.svg'
		}
	};

	const options = Object.keys(optionMap).map((key) => ({ key, value: optionMap[key] }));
	let selected = $state(options[0].key);
	let previousSelected = $state(options[0].key);
	let isAnimating = $state(false);
	let flyDirection = $state(100); // 100 for right, -100 for left

	function getFlyDirection(newSelection: string, oldSelection: string): number {
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

	function handleSelectionChange(newSelection: string) {
		if (newSelection !== selected) {
			previousSelected = selected;
			selected = newSelection;
			flyDirection = getFlyDirection(newSelection, previousSelected);
			isAnimating = true;

			// Reset animation state after animation completes
			setTimeout(() => {
				isAnimating = false;
			}, 300); // Match the CSS animation duration
		}
	}

	onMount(() => {
		checkScrollPosition();
		scrollContainer?.addEventListener('scroll', checkScrollPosition);
		window.addEventListener('resize', checkScrollPosition);

		return () => {
			scrollContainer?.removeEventListener('scroll', checkScrollPosition);
			window.removeEventListener('resize', checkScrollPosition);
		};
	});
</script>

<div class="flex w-full items-center gap-2">
	<div class="size-4">
		{#if showLeftChevron}
			<button onclick={scrollLeft}>
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
					class={twMerge(
						'dark:hover:bg-surface3 relative flex w-full items-center justify-center gap-1.5 rounded-t-xs border-b-2 border-transparent py-2 text-[13px] font-light transition-all duration-200 hover:bg-gray-50',
						selected === option.key &&
							'dark:bg-surface2 bg-background hover:bg-transparent dark:hover:bg-transparent'
					)}
					onclick={() => {
						handleSelectionChange(option.key);
					}}
				>
					<img
						src={option.value.icon}
						alt={option.value.label}
						class="size-5 rounded-sm p-0.5 dark:bg-gray-600"
					/>
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
			<button onclick={scrollRight}>
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
					{#if option.key === 'oauth'}
						<p>
							For clients that support OAuth authentication (browser-based login). Use the configuration below and authenticate through your browser.
						</p>
						{@render codeSnippet(`
	{
		"mcpServers": {
${servers
	.map(
		(server) => `			"${server.name}": {
				"url": "${server.url}"
			}`
	)
	.join(',\n')}
		}
	}

`)}
					{:else if option.key === 'apikey'}
						<p>
							For clients that require API key/bearer token authentication. Add your API key in the Authorization header.
						</p>
						{@render codeSnippet(`
	{
		"mcpServers": {
${servers
	.map(
		(server) => `			"${server.name}": {
				"url": "${server.url}",
				"headers": {
					"Authorization": "Bearer YOUR_API_KEY"
				}
			}`
	)
	.join(',\n')}
		}
	}

`)}
					{:else if option.key === 'vscode'}
						<p>
							To add this MCP server to VSCode, update your <span class="snippet"
								>.vscode/mcp.json</span
							>
						</p>
						{@render codeSnippet(`
	{
		"servers": {
${servers
	.map(
		(server) => `			"${server.name}": {
				"url": "${server.url}"
			}`
	)
	.join(',\n')}
		}
	}

`)}
					{/if}
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
				class="text-white"
				classes={{ button: 'flex gap-1 flex-shrink-0 items-center text-white' }}
			/>
		</div>
		<pre><code>{code}</code></pre>
	</div>
{/snippet}

<style lang="postcss">
	.snippet {
		background-color: var(--surface1);
		border-radius: 0.375rem;
		padding: 0.125rem 0.5rem;
		font-size: 13px;
		font-weight: 300;

		.dark & {
			background-color: var(--surface3);
		}
	}
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
