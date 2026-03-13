<script lang="ts">
	import { type PageProps } from './$types';
	import { browser } from '$app/environment';
	import Logo from '$lib/components/Logo.svelte';
	import LocalLoginForm from '$lib/components/local-auth/LocalLoginForm.svelte';

	let { data }: PageProps = $props();
	let { authProviders, loggedIn, localAuthEnabled, bootstrapEnabled } = $derived(data);
	let overrideRedirect = $state<string | null>(null);

	let rd = $derived.by(() => {
		if (browser) {
			const rd = new URL(window.location.href).searchParams.get('rd');
			if (rd) {
				return rd;
			}
		}
		if (overrideRedirect !== null) {
			return overrideRedirect;
		}
		return '/';
	});
</script>

<svelte:head>
	<title>Build AI agents with MCP</title>
</svelte:head>

{#if !loggedIn}
	{@render unauthorizedContent()}
{:else}
	<div class="flex h-svh w-svw flex-col items-center justify-center">
		<div class="flex items-center justify-center gap-2">
			<div class="animate-bounce">
				<Logo />
			</div>
			<p class="text-base font-semibold">Logging in...</p>
		</div>
	</div>
{/if}

{#snippet unauthorizedContent()}
	<div class="text-on-background relative flex h-dvh w-full flex-col">
		<main
			class="dark:from-surface2 to-surface1 mx-auto flex h-full w-full flex-col items-center justify-center bg-radial-[at_50%_50%] from-gray-50 dark:to-black"
		>
			<div class="flex w-full max-w-sm flex-col items-center gap-6 px-4">
				<Logo class="h-16" />
				<div class="flex flex-col items-center gap-1">
					<h1 class="text-2xl font-semibold">Welcome</h1>
					<p class="text-on-surface1 text-sm font-light">Log in to continue</p>
				</div>

					<div class="flex w-full flex-col gap-3">
					<!-- OAuth providers -->
					{#each authProviders as provider (provider.id)}
						<button
							class="bg-surface2 hover:bg-surface3 dark:bg-surface2 dark:hover:bg-surface3 flex w-full items-center justify-center gap-2 rounded-lg px-4 py-3 text-sm font-medium transition-colors duration-150"
							onclick={() => {
								localStorage.setItem('preAuthRedirect', window.location.href);
								window.location.href = `/oauth2/start?rd=${encodeURIComponent(
									overrideRedirect !== null ? overrideRedirect : rd
								)}&obot-auth-provider=${provider.namespace}/${provider.id}`;
							}}
						>
							{#if provider.icon}
								<img
									class="h-5 w-5 rounded-full"
									src={provider.icon}
									alt={provider.name}
								/>
							{/if}
							<span>Continue with {provider.name}</span>
						</button>
					{/each}

					<!-- Local auth section -->
					{#if localAuthEnabled}
						<LocalLoginForm
							{authProviders}
							onSuccess={() => {
								// Full page navigation so the layout load re-runs and picks up
								// the newly issued session cookie via getProfile().
								window.location.href = rd;
							}}
						/>
					{/if}

					<!-- Bootstrap token login (when bootstrap is still active) -->
					{#if bootstrapEnabled}
						<a
							href="/admin"
							class="bg-surface2 hover:bg-surface3 dark:bg-surface2 dark:hover:bg-surface3 flex w-full items-center justify-center gap-2 rounded-lg px-4 py-3 text-sm font-medium transition-colors duration-150"
						>
							Sign in with Bootstrap Token
						</a>
					{/if}

					{#if authProviders.length === 0 && !localAuthEnabled && !bootstrapEnabled}
						<p class="text-on-surface1 text-center text-sm">
							No auth providers configured. Please configure at least one auth provider in the
							admin panel.
						</p>
					{/if}
				</div>
			</div>
		</main>
	</div>
{/snippet}

<style lang="postcss">
	:global {
		.well {
			padding-left: 1rem;
			padding-right: 1rem;
			@media (min-width: 1024px) {
				padding-left: 4rem;
				padding-right: 4rem;
			}
			@media (min-width: 768px) {
				padding-left: 2rem;
				padding-right: 2rem;
			}
		}
	}
</style>
