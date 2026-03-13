<script lang="ts">
	import { AdminService } from '$lib/services';
	import ResponsiveDialog from '$lib/components/ResponsiveDialog.svelte';
	import { LoaderCircle } from 'lucide-svelte';

	interface UserInfo {
		id: string;
		name?: string;
		username?: string;
		email?: string;
	}

	interface Props {
		user?: UserInfo;
	}

	let { user = $bindable() }: Props = $props();
	let dialog = $state<ReturnType<typeof ResponsiveDialog>>();
	let newPassword = $state('');
	let mustChange = $state(true);
	let error = $state('');
	let loading = $state(false);

	export function open() {
		newPassword = '';
		mustChange = true;
		error = '';
		dialog?.open();
	}

	function displayName(u?: UserInfo) {
		return u?.name ?? u?.username ?? u?.email ?? 'Unknown';
	}

	async function handleSubmit() {
		if (!newPassword) {
			error = 'New password is required.';
			return;
		}
		if (!user) return;
		error = '';
		loading = true;
		try {
			await AdminService.updateLocalPassword(user.id, newPassword, mustChange);
			dialog?.close();
			user = undefined;
		} catch (err: unknown) {
			error = (err as { message?: string })?.message ?? 'Failed to reset password.';
		} finally {
			loading = false;
		}
	}
</script>

<ResponsiveDialog
	bind:this={dialog}
	class="w-full overflow-visible p-4 md:max-w-md"
	title={`Reset Password for ${displayName(user)}`}
>
	<div class="flex flex-col gap-4 p-4">
		<div class="flex flex-col gap-1">
			<label for="reset-password" class="input-label">New Password</label>
			<input
				id="reset-password"
				class="text-input-filled dark:bg-background"
				type="password"
				placeholder="New password"
				bind:value={newPassword}
			/>
		</div>
		<label class="flex items-center gap-2 text-sm font-light">
			<input type="checkbox" bind:checked={mustChange} />
			Force password change on next login
		</label>
		{#if error}
			<div class="notification-error p-3 text-sm">{error}</div>
		{/if}
		<div class="flex justify-end gap-2 pt-2">
			<button
				class="button"
				onclick={() => {
					dialog?.close();
					user = undefined;
					error = '';
				}}>Cancel</button
			>
			<button class="button-primary" disabled={loading} onclick={handleSubmit}>
				{#if loading}
					<LoaderCircle class="size-4 animate-spin" />
				{:else}
					Reset Password
				{/if}
			</button>
		</div>
	</div>
</ResponsiveDialog>
