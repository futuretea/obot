import { handleRouteError } from '$lib/errors';
import { AdminService } from '$lib/services';
import type { OrgUser } from '$lib/services/admin/types';
import { profile } from '$lib/stores';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	let users: OrgUser[] = [];
	let localAuthEnabled = false;
	try {
		users = await AdminService.listUsers({ fetch });
	} catch (err) {
		handleRouteError(err, `/users`, profile.current);
	}
	try {
		const localStatus = await AdminService.getLocalAuthStatus();
		localAuthEnabled = localStatus.enabled;
	} catch (err: unknown) {
		// Ignore 404 (feature not enabled); surface any other error
		if (!String((err as Error)?.message).startsWith('404')) {
			console.warn('Failed to check local auth status:', err);
		}
	}

	return {
		users,
		localAuthEnabled
	};
};
