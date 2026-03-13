import { AdminService, ChatService, getProfile, type AuthProvider } from '$lib/services';
import { Group, type BootstrapStatus } from '$lib/services/admin/types';
import type { PageLoad } from './$types';
import { redirect } from '@sveltejs/kit';

export const load: PageLoad = async ({ fetch, url }) => {
	let bootstrapStatus: BootstrapStatus | undefined;
	let authProviders: AuthProvider[] = [];
	let localAuthEnabled = false;
	let profile;

	try {
		profile = await getProfile({ fetch });
	} catch (_err) {
		[bootstrapStatus, authProviders] = await Promise.all([
			AdminService.getBootstrapStatus(),
			ChatService.listAuthProviders({ fetch })
		]);
		try {
			const localStatus = await AdminService.getLocalAuthStatus();
			localAuthEnabled = localStatus.enabled;
		} catch (err: unknown) {
			// Ignore 404 (feature not enabled); surface any other error
			if (!String((err as Error)?.message).startsWith('404')) {
				console.warn('Failed to check local auth status:', err);
			}
		}
	}

	const loggedIn = profile?.loaded ?? false;
	const isAdmin = profile?.groups.includes(Group.ADMIN);

	if (loggedIn) {
		const redirectRoute = url.searchParams.get('rd');
		if (redirectRoute) {
			throw redirect(302, redirectRoute);
		}

		// Redirect to appropriate dashboard
		throw redirect(302, isAdmin ? '/admin/mcp-servers' : '/mcp-servers');
	}

	if (bootstrapStatus?.enabled && authProviders.length === 0 && !localAuthEnabled) {
		// If no auth providers are configured, redirect to admin page for bootstrap login
		throw redirect(302, '/admin');
	}

	return {
		loggedIn,
		authProviders,
		localAuthEnabled,
		bootstrapEnabled: bootstrapStatus?.enabled ?? false
	};
};
