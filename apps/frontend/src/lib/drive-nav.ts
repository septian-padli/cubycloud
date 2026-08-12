const PATH_TO_NAV_ID: Record<string, string> = {
	'/drive/home': 'home',
	'/drive/my-storage': 'my-storage',
	'/drive/organization': 'organization',
	'/drive/shared': 'shared',
	'/drive/starred': 'starred',
	'/drive/trash': 'trash',
	'/drive/storage-stats': 'storage-stats',
	'/admin/settings': 'admin-settings',
	'/admin/users': 'admin-users',
	'/admin/addons': 'admin-addons'
};

/** Map current pathname to sidebar `activeId`. */
export function deriveDriveActiveId(pathname: string): string {
	if (pathname.startsWith('/s/')) return '';
	return PATH_TO_NAV_ID[pathname] ?? 'home';
}
