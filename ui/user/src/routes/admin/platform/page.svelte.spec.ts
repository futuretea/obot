import { page as appPage } from '$app/state';
import { Group, type ModelProxySettings, type ProductTelemetryConsent } from '$lib/services';
import type {
	ImagePullSecret,
	ImagePullSecretCapability,
	License
} from '$lib/services/admin/types';
import type { Version } from '$lib/services/user/types';
import { createMockProfile, preparePageData } from '../../../tests/helpers/pageData';
import { getLicenseResponse, getVersionResponse } from '../../../tests/mocks/data';
import type { PageData } from './$types';
import PlatformPage from './+page.svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';

vi.mock('$lib/url', async (importOriginal) => ({
	...(await importOriginal<typeof import('$lib/url')>()),
	setUrlParamAndUpdateUrl: vi.fn()
}));

async function renderPlatformPage({
	license = getLicenseResponse,
	versionOverrides = {},
	view = 'settings',
	create = false,
	capability = { available: false },
	imagePullSecrets = [],
	groups = [Group.ADMIN],
	productAnalyticsAvailable = true,
	productAnalyticsConsent = { consent: false },
	modelProxySettings
}: {
	license?: License;
	versionOverrides?: Partial<Version>;
	view?: string;
	create?: boolean;
	capability?: ImagePullSecretCapability;
	imagePullSecrets?: ImagePullSecret[];
	groups?: string[];
	productAnalyticsAvailable?: boolean;
	productAnalyticsConsent?: ProductTelemetryConsent;
	modelProxySettings?: ModelProxySettings;
} = {}) {
	appPage.url.searchParams.set('view', view);
	if (create) {
		appPage.url.searchParams.set('create', 'true');
	} else {
		appPage.url.searchParams.delete('create');
	}

	const data = await preparePageData<PageData>({
		profile: createMockProfile(groups),
		license,
		appNotification: undefined,
		k8sSettings: undefined,
		capability,
		imagePullSecrets,
		gitCredentials: [],
		productTelemetryConsentAvailable: productAnalyticsAvailable,
		productTelemetryConsent: productAnalyticsConsent,
		modelProxySettings,
		version: {
			...getVersionResponse,
			...versionOverrides
		}
	});

	return render(PlatformPage, { data });
}

afterEach(() => {
	appPage.url.searchParams.delete('view');
	appPage.url.searchParams.delete('create');
	appPage.url.searchParams.delete('id');
});

describe('Platform Page', () => {
	describe('settings tab', () => {
		it('shows product analytics inside Settings instead of its own tab', async () => {
			await renderPlatformPage({ view: 'settings' });

			await expect
				.element(page.getByRole('button', { name: 'Settings', exact: true }))
				.toBeVisible();
			await expect
				.element(page.getByRole('heading', { name: 'Product Analytics', exact: true }))
				.toBeVisible();
			await expect
				.element(page.getByText(/Share product usage data to help improve uni\./))
				.toBeVisible();
			await expect
				.element(page.getByRole('button', { name: 'Product Analytics', exact: true }))
				.not.toBeInTheDocument();
			await expect.element(page.getByRole('button', { name: 'Save', exact: true })).toBeVisible();
			await expect.element(page.getByRole('button', { name: 'Cancel', exact: true })).toBeVisible();
		});

		it('hides the product analytics section when consent controls are unavailable', async () => {
			await renderPlatformPage({ view: 'settings', productAnalyticsAvailable: false });

			await expect
				.element(page.getByRole('heading', { name: 'Product Analytics', exact: true }))
				.not.toBeInTheDocument();
		});

		it('hides product analytics and the shared actions from read-only administrators', async () => {
			await renderPlatformPage({ view: 'settings', groups: [Group.AUDITOR] });

			await expect
				.element(page.getByRole('heading', { name: 'Product Analytics', exact: true }))
				.not.toBeInTheDocument();
			await expect
				.element(page.getByRole('button', { name: 'Save', exact: true }))
				.not.toBeInTheDocument();
		});
	});

	describe('registry connections section', () => {
		it('hides the section when the engine is not kubernetes', async () => {
			await renderPlatformPage({
				view: 'settings',
				capability: { available: true }
			});

			await expect
				.element(page.getByRole('heading', { name: 'Registry Connections', exact: true }))
				.not.toBeInTheDocument();
		});

		it('renders image pull secrets inside Settings', async () => {
			await renderPlatformPage({
				view: 'settings',
				capability: { available: true },
				versionOverrides: { engine: 'kubernetes' }
			});

			await expect
				.element(page.getByRole('button', { name: 'Settings', exact: true }))
				.toBeVisible();
			await expect
				.element(page.getByRole('heading', { name: 'Registry Connections', exact: true }))
				.toBeVisible();
			await expect
				.element(page.getByText("Click '+' to add a basic secret.", { exact: true }))
				.toBeVisible();
			await expect
				.element(page.getByRole('button', { name: 'Add Basic Secret', exact: true }))
				.toBeVisible();
			await expect
				.element(page.getByRole('button', { name: 'Add ECR Secret', exact: true }))
				.toBeVisible();
		});

		it('opens the create form inside Settings', async () => {
			await renderPlatformPage({
				view: 'settings',
				create: true,
				capability: { available: true },
				versionOverrides: { engine: 'kubernetes' }
			});

			await expect
				.element(page.getByRole('button', { name: 'Settings', exact: true }))
				.toBeVisible();
			await expect
				.element(page.getByRole('heading', { name: 'Registry Connections', exact: true }))
				.toBeVisible();
			await expect
				.element(page.getByRole('heading', { name: 'Add Basic Secret', exact: true }))
				.toBeVisible();
			await expect.element(page.getByText('Registry Server', { exact: true })).toBeVisible();
			await expect
				.element(page.getByText("Click '+' to add a basic secret.", { exact: true }))
				.toBeVisible();
		});
	});
});
