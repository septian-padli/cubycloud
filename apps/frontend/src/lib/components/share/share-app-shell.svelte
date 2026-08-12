<script lang="ts">
	import { page } from '$app/state';
	import type { Snippet } from 'svelte';
	import DriveShell from '$lib/components/drive/drive-shell.svelte';
	import SharePublicShell from '$lib/components/share/share-public-shell.svelte';
	import { setDriveChrome } from '$lib/drive-context.svelte.js';
	import { deriveDriveActiveId } from '$lib/drive-nav.js';
	import { MOCK_DRIVE_USER, MOCK_QUOTA } from '$lib/mocks/drive.js';

	let {
		loggedIn = false,
		children
	}: {
		loggedIn?: boolean;
		children: Snippet;
	} = $props();

	setDriveChrome({ user: MOCK_DRIVE_USER, quota: MOCK_QUOTA });

	const activeId = $derived(deriveDriveActiveId(page.url.pathname));
</script>

{#if loggedIn}
	<DriveShell {activeId}>
		{#snippet content()}
			{@render children()}
		{/snippet}
	</DriveShell>
{:else}
	<SharePublicShell>
		{@render children()}
	</SharePublicShell>
{/if}
