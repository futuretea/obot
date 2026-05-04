<script lang="ts">
	import { initLayout } from '$lib/context/chatLayout.svelte';
	import { DEFAULT_PROJECT_NAME } from '$lib/constants';
	import { profile } from '$lib/stores';
	import { goto } from '$lib/url';
	import { type PageProps } from './$types';

	let { data }: PageProps = $props();
	let title = $derived(data.project?.name || DEFAULT_PROJECT_NAME);

	initLayout({
		items: []
	});

	$effect(() => {
		if (profile.current.unauthorized) {
			// Redirect to the main page to log in.
			window.location.href = `/?rd=${window.location.pathname}`;
		} else if (data.project?.id) {
			goto(`/o/${data.project.id}`, { replaceState: true });
		}
	});
</script>

<svelte:head>
	{#if title}
		<title>{title}</title>
	{/if}
</svelte:head>
