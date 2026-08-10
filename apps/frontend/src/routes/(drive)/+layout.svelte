<script lang="ts">
	import { page } from '$app/state';
	import DriveShell from '$lib/components/drive/drive-shell.svelte';
	import { setDriveChrome } from '$lib/drive-context.svelte.js';
	import { deriveDriveActiveId } from '$lib/drive-nav.js';
	import { MOCK_DRIVE_USER, MOCK_QUOTA } from '$lib/mocks/drive.js';

	let { children } = $props();

	setDriveChrome({ user: MOCK_DRIVE_USER, quota: MOCK_QUOTA });

	const activeId = $derived(deriveDriveActiveId(page.url.pathname));
</script>

<DriveShell {activeId}>
	{#snippet content()}
		{@render children()}
	{/snippet}
</DriveShell>
