<script lang="ts">
	import { AdminService } from '$lib/services';
	import { Role, type OrgUser } from '$lib/services/admin/types';
	import ResponsiveDialog from '$lib/components/ResponsiveDialog.svelte';
	import { LoaderCircle } from 'lucide-svelte';

	interface Props {
		onCreated: (users: OrgUser[]) => void;
	}

	let { onCreated }: Props = $props();

	let dialog = $state<ReturnType<typeof ResponsiveDialog>>();
	let username = $state('');
	let email = $state('');
	let password = $state('');
	let role = $state(Role.BASIC);
	let error = $state('');
	let loading = $state(false);

	export function open() {
		dialog?.open();
	}

	function reset() {
		username = '';
		email = '';
		password = '';
		role = Role.BASIC;
		error = '';
	}

	async function handleSubmit() {
		if (!username || !password) {
			error = 'Username and password are required.';
			return;
		}
		error = '';
		loading = true;
		try {
			await AdminService.createLocalUser({
				username,
				email: email || undefined,
				password,
				role
			});
			const users = await AdminService.listUsers();
			dialog?.close();
			reset();
			onCreated(users);
		} catch (err: unknown) {
			error = (err as { message?: string })?.message ?? 'Failed to create local user.';
		} finally {
			loading = false;
		}
	}
</script>

<ResponsiveDialog
	bind:this={dialog}
	class="w-full overflow-visible p-4 md:max-w-md"
	title="Create Local User"
>
	<div class="flex flex-col gap-4 p-4">
		<div class="flex flex-col gap-1">
			<label for="new-username" class="input-label"
				>Username <span class="text-red-500">*</span></label
			>
			<input
				id="new-username"
				class="text-input-filled dark:bg-background"
				type="text"
				autocomplete="off"
				placeholder="e.g. john.doe"
				bind:value={username}
			/>
		</div>
		<div class="flex flex-col gap-1">
			<label for="new-email" class="input-label">Email</label>
			<input
				id="new-email"
				class="text-input-filled dark:bg-background"
				type="email"
				autocomplete="off"
				placeholder="e.g. john@example.com"
				bind:value={email}
			/>
		</div>
		<div class="flex flex-col gap-1">
			<label for="new-password" class="input-label"
				>Initial Password <span class="text-red-500">*</span></label
			>
			<input
				id="new-password"
				class="text-input-filled dark:bg-background"
				type="password"
				autocomplete="new-password"
				placeholder="Temporary password"
				bind:value={password}
			/>
			<span class="text-on-surface1 text-xs"
				>User will be required to change this on first login.</span
			>
		</div>
		<div class="flex flex-col gap-1">
			<label for="new-role" class="input-label">Role</label>
			<select id="new-role" class="text-input-filled dark:bg-background" bind:value={role}>
				<option value={Role.BASIC}>Basic User</option>
				<option value={Role.POWERUSER}>Power User</option>
				<option value={Role.ADMIN}>Admin</option>
			</select>
		</div>
		{#if error}
			<div class="notification-error p-3 text-sm">{error}</div>
		{/if}
		<div class="flex justify-end gap-2 pt-2">
			<button
				class="button"
				onclick={() => {
					dialog?.close();
					error = '';
				}}>Cancel</button
			>
			<button class="button-primary" disabled={loading} onclick={handleSubmit}>
				{#if loading}
					<LoaderCircle class="size-4 animate-spin" />
				{:else}
					Create
				{/if}
			</button>
		</div>
	</div>
</ResponsiveDialog>
