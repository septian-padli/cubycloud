import { error } from '@sveltejs/kit';
import { MOCK_ADMIN_USERS } from '$lib/mocks/admin-users.js';
import { findAdminUserById } from '$lib/mocks/user-profiles.js';
import type { EntryGenerator, PageLoad } from './$types.js';

export const entries: EntryGenerator = () => MOCK_ADMIN_USERS.map((u) => ({ userId: u.id }));

export const load: PageLoad = ({ params }) => {
	const user = findAdminUserById(params.userId);
	if (!user) {
		error(404, 'Pengguna tidak ditemukan');
	}
	return { user };
};
