import { Group } from '$lib/services';
import type { License } from '$lib/services/admin/types';
import type { Profile, Version } from '$lib/services/user/types';
import {
	defaultModelAliases,
	license as licenseStore,
	profile,
	userDeviceSettings,
	version
} from '$lib/stores';
import { getLicenseResponse, getProfileResponse, getVersionResponse } from '../../tests/mocks/data';
import Layout from './Layout.svelte';
import { createRawSnippet, tick } from 'svelte';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';

const children = createRawSnippet(() => ({ render: () => '<div></div>' }));

const sharedLinks = ['/vmcps', '/skills', '/models', '/audit-logs', '/usage', '/identity-access'];

const adminOnlyLinks = ['/admin/enforcement-events', '/admin/platform'];

function createProfile(groups: string[]): Profile {
	return {
		...getProfileResponse,
		groups,
		iconURL: '',
		loaded: true,
		canImpersonate: () => groups.includes(Group.ADMIN) && groups.includes(Group.USER_IMPERSONATION),
		hasAdminAccess: () => groups.includes(Group.ADMIN) || groups.includes(Group.AUDITOR),
		isAdmin: () => groups.includes(Group.ADMIN),
		isAdminReadonly: () => !groups.includes(Group.ADMIN) && groups.includes(Group.AUDITOR),
		isBootstrapUser: () => false
	};
}

async function renderLayout(
	groups: string[] = [],
	versionOverrides: Partial<Version> = {},
	licenseOverrides: Partial<License> = {},
	profileOverrides: Partial<Profile> = {}
) {
	userDeviceSettings.setShowAllGuides(false);
	profile.initialize({
		...createProfile(groups),
		...profileOverrides
	});
	version.initialize({
		...getVersionResponse,
		agentsEnabled: false,
		engine: 'docker',
		...versionOverrides
	});
	licenseStore.initialize({
		...getLicenseResponse,
		...licenseOverrides
	});
	await defaultModelAliases.initialize([]);

	return render(Layout, { children });
}

async function clickButton(id: string) {
	const locator = page.getByCSS(`#${id}`);
	await expect.element(locator).toBeInTheDocument();
	// Native DOM click: Playwright actionability fails on driver.js overlays / off-viewport sidebar.
	const el = await locator.element();
	if (!(el instanceof HTMLElement)) {
		throw new Error(`Expected #${id} to be an HTMLElement`);
	}
	el.click();
	await tick();
}

async function expandSection(id: string, expectedHref: string) {
	const link = page.getByCSS(`a.sidebar-link[href="${expectedHref}"]`);
	if ((await link.elements()).length === 0) {
		await clickButton(`sidebar-collapse-${id}`);
	}
}

async function expectLink(href: string) {
	await expect.element(page.getByCSS(`a.sidebar-link[href="${href}"]`)).toBeInTheDocument();
}

async function expectNoLink(href: string) {
	await expect.element(page.getByCSS(`a.sidebar-link[href="${href}"]`)).not.toBeInTheDocument();
}

async function expectSharedNavigation() {
	await expandSection('ai-resources', '/vmcps');
	await expandSection('operations', '/audit-logs');

	for (const href of sharedLinks) {
		await expectLink(href);
	}
}

async function expectAdminOnlyNavigation() {
	await expandSection('operations', '/admin/enforcement-events');

	for (const href of adminOnlyLinks) {
		await expectLink(href);
	}
}

async function expectNoAdminOnlyNavigation() {
	for (const href of adminOnlyLinks) {
		await expectNoLink(href);
	}
}

describe('Layout.svelte', () => {
	it('gives all users access to shared sidebar navigation', async () => {
		await renderLayout();
		await expectSharedNavigation();
		await expectNoLink('/dashboard');
		await expectNoAdminOnlyNavigation();
	});

	describe('when Hosted Agents are disabled', () => {
		it('hides Hosted Agents navigation', async () => {
			await renderLayout([Group.ADMIN], { hostedAgentsEnabled: false });

			await expectNoLink('/hosted-agents');
		});
	});

	describe('when Hosted Agents are enabled', () => {
		it('shows Hosted Agents navigation', async () => {
			await renderLayout([Group.ADMIN], { hostedAgentsEnabled: true });
			await expectLink('/hosted-agents');
		});
	});

	describe('based on user role', () => {
		describe('when the user is an administrator', () => {
			it('shows administrator-only navigation', async () => {
				await renderLayout([Group.ADMIN]);
				await expectSharedNavigation();
				await expectLink('/dashboard');
				await expectAdminOnlyNavigation();
				await expectNoLink('/admin/product-analytics');
			});
		});

		describe('when the user is a power user', () => {
			it('does not show administrator-only navigation', async () => {
				await renderLayout([Group.POWERUSER]);
				await expectSharedNavigation();
				await expectLink('/dashboard');
				await expectLink('/mcp-servers');
				await expectNoAdminOnlyNavigation();
			});
		});

		describe('when the user is a power user plus', () => {
			it('does not show administrator-only navigation', async () => {
				await renderLayout([Group.POWERUSER, Group.POWERUSER_PLUS]);
				await expectSharedNavigation();
				await expectLink('/dashboard');
				await expectLink('/mcp-servers');
				await expectNoAdminOnlyNavigation();
			});
		});

		describe('when the user is a basic user', () => {
			it('hides MCP Servers and does not show administrator-only navigation', async () => {
				await renderLayout([Group.USER]);
				await expectSharedNavigation();
				await expectNoLink('/dashboard');
				await expectNoLink('/mcp-servers');
				await expectNoAdminOnlyNavigation();
			});
		});

		describe('when the user is a basic user and auditor', () => {
			it('shows the administrator navigation available to auditors', async () => {
				await renderLayout([Group.USER, Group.AUDITOR]);
				await expectSharedNavigation();
				await expectLink('/dashboard');
				await expectAdminOnlyNavigation();
				await expectNoLink('/admin/product-analytics');
			});
		});
	});

	describe('profile menu', () => {
		it('links a signed-in user to the CLI install page', async () => {
			await renderLayout();
			await clickButton('btn-navbar-profile');
			const link = page.getByRole('link', { name: 'Install uni CLI' });
			await expect.element(link).toBeInTheDocument();
			await expect.element(link).toHaveAttribute('href', '/install-cli');
		});
	});
});
