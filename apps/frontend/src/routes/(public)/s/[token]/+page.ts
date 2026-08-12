import { findShareLink } from '$lib/mocks/share-links.js';
import type { EntryGenerator, PageLoad } from './$types.js';

export const prerender = true;

export const entries: EntryGenerator = () => [
	{ token: 'demo-public' },
	{ token: 'demo-folder' },
	{ token: 'demo-password' },
	{ token: 'demo-restricted' },
	{ token: 'demo-expired' },
	{ token: 'demo-missing' }
];

export const load: PageLoad = ({ params }) => {
	return {
		token: params.token,
		link: findShareLink(params.token) ?? null
	};
};
