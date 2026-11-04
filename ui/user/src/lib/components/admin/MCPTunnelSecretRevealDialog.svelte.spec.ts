import type { MCPTunnel } from '$lib/services';
import MCPTunnelSecretRevealDialog from './MCPTunnelSecretRevealDialog.svelte';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';

function tunnelWithSecret(): MCPTunnel {
	return {
		name: 'mt1example',
		token: 'secret-token-123'
	} as unknown as MCPTunnel;
}

describe('MCPTunnelSecretRevealDialog', () => {
	it('shows only the uni CLI command with no Docker tab', async () => {
		const { container } = await render(MCPTunnelSecretRevealDialog, {
			props: { action: 'created', onClose: vi.fn(), tunnel: tunnelWithSecret() }
		});

		const text = container.textContent ?? '';
		expect(text).toContain(
			`uni tunnel --uni-base-url ${window.location.origin}/api --token secret-token-123`
		);
		expect(text).not.toContain('ghcr.io');
		expect(text).not.toContain('docker');
		expect(text).not.toContain('Docker');
		expect(text.match(/uni tunnel --uni-base-url/g)?.length).toBe(1);
	});
});
