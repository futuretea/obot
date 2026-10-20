import { Group, type McpAuthLogUsageStatItem } from '$lib/services';
import { createMockProfile, preparePageData } from '../../tests/helpers/pageData';
import { worker } from '../../tests/mocks/worker';
import type { PageData } from './$types';
import DashboardPage from './+page.svelte';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';

function usage(mcpID: string, name: string): McpAuthLogUsageStatItem {
	return {
		mcpID,
		mcpServerInstanceName: mcpID,
		mcpServerName: mcpID,
		mcpServerDisplayName: name,
		toolCalls: [
			{
				toolName: 'lookup',
				callCount: 1,
				items: [
					{
						createdAt: new Date().toISOString(),
						userID: '1',
						processingTimeMs: 42,
						responseStatus: 200,
						error: ''
					}
				]
			}
		]
	};
}

async function renderUsage(items: McpAuthLogUsageStatItem[]) {
	worker.use(
		http.get('/api/mcp-stats', () =>
			HttpResponse.json({
				items,
				timeStart: '',
				timeEnd: '',
				totalCalls: items.length,
				uniqueUsers: 1
			})
		)
	);
	const data = await preparePageData<PageData>({
		profile: createMockProfile([Group.POWERUSER]),
		hasDeviceScans: false
	});
	render(DashboardPage, { data });
	await expect.element(page.getByText('Finance', { exact: true }).first()).toBeVisible();
}

function statisticCards() {
	return [
		page
			.getByCSS('.paper')
			.filter({ has: page.getByRole('heading', { name: /Top Servers Used/ }) }),
		page
			.getByCSS('.paper')
			.filter({ has: page.getByRole('heading', { name: /Recently Popular Tools/ }) }),
		page
			.getByCSS('.paper')
			.filter({ has: page.getByRole('heading', { name: /Tool Call Average Response Time/ }) })
	];
}

describe('Dashboard usage statistics', () => {
	it('keeps user-defined uni names in every usage statistic', async () => {
		await renderUsage([usage('vmcp1finance', 'uni Finance'), usage('vmcp1plain', 'Finance')]);

		for (const card of statisticCards()) {
			await expect.element(card.getByText(/uni Finance/).first()).toBeVisible();
		}
	});

	it('keeps ordinary names while excluding existing internal server prefixes', async () => {
		await renderUsage([
			usage('vmcp1plain', 'Finance'),
			usage('sms1system', 'System Server'),
			usage('vmcp1agent', 'nba1agent'),
			usage('vmcp1internal', 'Obot Search')
		]);

		for (const card of statisticCards()) {
			await expect.element(card.getByText(/Finance/).first()).toBeVisible();
			await expect
				.element(card.getByText(/System Server|nba1agent|Obot Search/))
				.not.toBeInTheDocument();
		}
	});
});
