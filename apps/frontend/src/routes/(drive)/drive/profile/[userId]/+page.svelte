<script lang="ts">
	import { toast } from 'svelte-sonner';
	import UserProfileView from '$lib/components/drive/user-profile-view.svelte';
	import { getDriveChrome } from '$lib/drive-context.svelte.js';
	import { categoryUsageForUser, quotaForAdminUser } from '$lib/mocks/user-profiles.js';
	import type { PageData } from './$types.js';

	let { data }: { data: PageData } = $props();

	const chrome = getDriveChrome();
	const quota = $derived(quotaForAdminUser(data.user));
	const categories = $derived(categoryUsageForUser(data.user.usedGb));
	const isSelf = $derived(chrome.user.id === data.user.id);

	const viewUser = $derived(
		isSelf
			? {
					...data.user,
					displayName: chrome.user.displayName,
					email: chrome.user.email
				}
			: data.user
	);

	function onSaveProfile(next: { displayName: string; email: string }) {
		chrome.updateProfile(next);
		toast.success('Profil diperbarui (mock)');
	}
</script>

<svelte:head>
	<title>{viewUser.displayName} — Profil — CubyCloud</title>
	<meta name="description" content="Profil pengguna dan statistik penyimpanan My Storage." />
</svelte:head>

<UserProfileView
	user={viewUser}
	{quota}
	{categories}
	editable={isSelf}
	onSaveProfile={isSelf ? onSaveProfile : undefined}
/>
