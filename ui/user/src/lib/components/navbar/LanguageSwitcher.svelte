<script lang="ts">
	import { locale, setLocale, t } from '$lib/i18n';
	import { Globe } from 'lucide-svelte/icons';
	import { tooltip } from '$lib/actions/tooltip.svelte';

	interface LocaleOption {
		code: string;
		label: string;
		nativeLabel: string;
	}

	const locales: LocaleOption[] = [
		{ code: 'en', label: 'English', nativeLabel: 'English' },
		{ code: 'zh-CN', label: 'Chinese (Simplified)', nativeLabel: '简体中文' }
	];

	let isOpen = $state(false);
	let dropdownRef = $state<HTMLDivElement>();

	const currentLocale = $derived(locales.find((l) => l.code === $locale) ?? locales[0]);

	function handleLocaleChange(code: string) {
		setLocale(code);
		isOpen = false;
	}

	function handleClickOutside(event: MouseEvent) {
		if (dropdownRef && !dropdownRef.contains(event.target as Node)) {
			isOpen = false;
		}
	}

	$effect(() => {
		if (isOpen) {
			document.addEventListener('click', handleClickOutside);
			return () => document.removeEventListener('click', handleClickOutside);
		}
	});
</script>

<div class="relative" bind:this={dropdownRef}>
	<button
		type="button"
		onclick={() => (isOpen = !isOpen)}
		class="icon-button flex items-center gap-1"
		use:tooltip={{ text: $t('language.switchLanguage') || 'Switch Language', disablePortal: true }}
	>
		<Globe class="size-5" />
		<span class="text-xs hidden md:inline">{currentLocale.nativeLabel}</span>
	</button>

	{#if isOpen}
		<div
			class="bg-surface2 border-surface3 absolute right-0 top-full z-50 mt-1 min-w-[140px] rounded-md border shadow-lg"
		>
			{#each locales as loc (loc.code)}
				<button
					type="button"
					onclick={() => handleLocaleChange(loc.code)}
					class="hover:bg-surface3 flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors first:rounded-t-md last:rounded-b-md"
					class:bg-surface3={$locale === loc.code}
				>
					<span>{loc.nativeLabel}</span>
					{#if $locale === loc.code}
						<span class="text-primary ml-auto">✓</span>
					{/if}
				</button>
			{/each}
		</div>
	{/if}
</div>
