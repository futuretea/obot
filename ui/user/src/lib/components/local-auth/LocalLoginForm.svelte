<script lang="ts">
	import { AdminService } from '$lib/services';
	import { Eye, EyeOff, LoaderCircle } from 'lucide-svelte';

	interface Props {
		authProviders: { id: string; name: string; namespace?: string; icon?: string }[];
		onSuccess: () => void;
	}

	let { authProviders, onSuccess }: Props = $props();

	let localUsername = $state('');
	let localPassword = $state('');
	let showPassword = $state(false);
	let localError = $state('');
	let localLoading = $state(false);

	// Must-change-password state
	let mustChangePassword = $state(false);
	let newPassword = $state('');
	let newPasswordConfirm = $state('');
	let showNewPassword = $state(false);
	let changePasswordError = $state('');
	let changePasswordLoading = $state(false);

	// When OAuth providers exist, local form is collapsed by default
	let showLocalForm = $state(false);
	let localFormExpanded = $derived(showLocalForm || authProviders.length === 0);

	async function handleLocalLogin() {
		if (!localUsername || !localPassword) {
			localError = 'Username and password are required.';
			return;
		}
		localError = '';
		localLoading = true;
		try {
			const result = await AdminService.localLogin(localUsername, localPassword);
			if (result.mustChangePassword) {
				mustChangePassword = true;
			} else if (result.success) {
				// Use full page navigation so the session cookie is picked up by
				// the layout load (getProfile). A SvelteKit goto() same-page
				// navigation does not re-run the layout load.
				onSuccess();
			}
		} catch (err: unknown) {
			localError = (err as { message?: string })?.message ?? 'Login failed.';
		} finally {
			localLoading = false;
		}
	}

	async function handleChangePassword() {
		if (!newPassword || newPassword !== newPasswordConfirm) {
			changePasswordError = 'Passwords do not match.';
			return;
		}
		changePasswordError = '';
		changePasswordLoading = true;
		try {
			// Server-side must-change flow: re-send credentials with the new password.
			// No session cookie was issued on the first login, so we must authenticate again.
			const result = await AdminService.localLogin(localUsername, localPassword, newPassword);
			if (result.success) {
				onSuccess();
			} else {
				changePasswordError = 'Password changed but login did not complete. Please try logging in again.';
			}
		} catch (err: unknown) {
			changePasswordError =
				(err as { message?: string })?.message ?? 'Failed to change password.';
		} finally {
			changePasswordLoading = false;
		}
	}
</script>

{#if mustChangePassword}
	<!-- Must-change-password form -->
	<div class="flex w-full flex-col gap-3">
		<p class="text-sm font-light">You must set a new password before continuing.</p>
		<button
			type="button"
			class="text-on-surface1 hover:text-on-background w-full text-left text-xs transition-colors"
			onclick={() => {
				mustChangePassword = false;
				newPassword = '';
				newPasswordConfirm = '';
				changePasswordError = '';
			}}
		>
			&#8592; Back to sign in
		</button>
		<div class="relative">
			<input
				class="text-input-filled dark:bg-background pr-10"
				type={showNewPassword ? 'text' : 'password'}
				placeholder="New password"
				bind:value={newPassword}
			/>
			<button
				type="button"
				class="text-on-surface1 hover:text-on-background absolute top-1/2 right-3 -translate-y-1/2"
				onclick={() => (showNewPassword = !showNewPassword)}
			>
				{#if showNewPassword}<EyeOff class="size-4" />{:else}<Eye class="size-4" />{/if}
			</button>
		</div>
		<input
			class="text-input-filled dark:bg-background"
			type="password"
			placeholder="Confirm new password"
			bind:value={newPasswordConfirm}
		/>
		{#if changePasswordError}
			<div class="notification-error p-3 text-sm">{changePasswordError}</div>
		{/if}
		<button
			class="button-primary w-full"
			disabled={changePasswordLoading}
			onclick={handleChangePassword}
		>
			{#if changePasswordLoading}
				<LoaderCircle class="size-4 animate-spin" />
			{:else}
				Set Password & Continue
			{/if}
		</button>
	</div>
{:else}
	<div class="flex w-full flex-col gap-3">
		{#if localFormExpanded || authProviders.length === 0}
			{#if authProviders.length > 0}
				<div class="flex items-center gap-3">
					<hr class="border-surface3 flex-1" />
					<span class="text-on-surface1 text-xs">or</span>
					<hr class="border-surface3 flex-1" />
				</div>
			{/if}
			<form
				class="flex flex-col gap-3"
				onsubmit={(e) => {
					e.preventDefault();
					handleLocalLogin();
				}}
			>
				<input
					class="text-input-filled dark:bg-background"
					type="text"
					autocomplete="username"
					placeholder="Username"
					bind:value={localUsername}
				/>
				<div class="relative">
					<input
						class="text-input-filled dark:bg-background pr-10"
						type={showPassword ? 'text' : 'password'}
						autocomplete="current-password"
						placeholder="Password"
						bind:value={localPassword}
					/>
					<button
						type="button"
						class="text-on-surface1 hover:text-on-background absolute top-1/2 right-3 -translate-y-1/2"
						onclick={() => (showPassword = !showPassword)}
					>
						{#if showPassword}<EyeOff class="size-4" />{:else}<Eye class="size-4" />{/if}
					</button>
				</div>
				{#if localError}
					<div class="notification-error p-3 text-sm">{localError}</div>
				{/if}
				<button class="button-primary w-full" type="submit" disabled={localLoading}>
					{#if localLoading}
						<LoaderCircle class="size-4 animate-spin" />
					{:else}
						Sign in
					{/if}
				</button>
				{#if authProviders.length > 0}
					<button
						type="button"
						class="text-on-surface1 hover:text-on-background w-full text-center text-xs transition-colors"
						onclick={() => (showLocalForm = false)}
					>
						&#8592; Back to other sign-in options
					</button>
				{/if}
			</form>
		{:else}
			<!-- Collapsed: show as text link -->
			<div class="flex items-center gap-3">
				<hr class="border-surface3 flex-1" />
				<span class="text-on-surface1 text-xs">or</span>
				<hr class="border-surface3 flex-1" />
			</div>
			<button
				class="text-primary hover:text-primary/80 w-full text-center text-sm font-medium transition-colors"
				onclick={() => (showLocalForm = true)}
			>
				Sign in with username &amp; password
			</button>
		{/if}
	</div>
{/if}
