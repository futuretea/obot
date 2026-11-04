<script lang="ts">
	import ResponsiveDialog from '$lib/components/ResponsiveDialog.svelte';
	import type { MCPTunnel } from '$lib/services';
	import CopyField from '../CopyField.svelte';
	import { KeyRound, Terminal, TriangleAlert } from '@lucide/svelte';

	interface Props {
		action?: 'created' | 'rotated';
		onClose: () => void;
		tunnel?: MCPTunnel;
	}

	let { action = 'created', onClose, tunnel }: Props = $props();
	let dialog = $state<ReturnType<typeof ResponsiveDialog>>();
	let uniURL = $derived(
		typeof window === 'undefined' ? '<UNI_URL>/api' : `${window.location.origin}/api`
	);
	let command = $derived(
		tunnel ? `uni tunnel --uni-base-url ${uniURL} --token ${tunnel.token}` : ''
	);

	$effect(() => {
		if (tunnel?.token) {
			dialog?.open();
		}
	});
</script>

{#if tunnel?.token}
	<ResponsiveDialog
		bind:this={dialog}
		{onClose}
		title={action === 'created' ? 'MCP Tunnel Created' : 'MCP Tunnel Secret Rotated'}
		class="w-full max-w-2xl"
		disableClickOutside
	>
		<div class="flex flex-col gap-6">
			<div class="notification-alert">
				<div class="flex items-start gap-3">
					<TriangleAlert class="size-5 shrink-0" />
					<div class="flex flex-col gap-1">
						<p class="text-sm font-medium">Save this secret now</p>
						<p class="text-xs">
							This is the only time the complete tunnel secret will be shown. Store it securely
							before closing this dialog.
						</p>
					</div>
				</div>
			</div>

			<div class="flex flex-col gap-2">
				<p class="text-sm font-medium">Tunnel Secret</p>
				<CopyField value={tunnel.token} id="mcp-tunnel-secret">
					{#snippet preContent()}
						<KeyRound class="size-4" />
					{/snippet}
				</CopyField>
			</div>

			<div class="flex flex-col gap-2">
				<p class="text-sm font-medium">Connect this tunnel</p>
				<CopyField value={command} id="mcp-tunnel-command" variant="code">
					{#snippet preContent()}
						<Terminal class="size-4" />
					{/snippet}
				</CopyField>
			</div>
		</div>

		<div class="mt-6 flex justify-end">
			<button class="btn btn-primary" onclick={() => dialog?.close()}>
				I've saved the secret
			</button>
		</div>
	</ResponsiveDialog>
{/if}
