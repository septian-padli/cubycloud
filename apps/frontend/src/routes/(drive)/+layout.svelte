<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { getSetupStatus } from '$lib/api/setup';
	import DriveShell from '$lib/components/drive/drive-shell.svelte';
	import ErrorPage from '$lib/components/error/error-page.svelte';
	import { setDriveChrome } from '$lib/drive-context.svelte.js';
	import { deriveDriveActiveId } from '$lib/drive-nav.js';
	import { driveView } from '$lib/drive-view.svelte.js';
	import { MOCK_DRIVE_USER, MOCK_QUOTA } from '$lib/mocks/drive.js';
	import { setMockLoggedIn } from '$lib/mocks/auth-session.js';

	let { children } = $props();

	let checking = $state(true);
	let isCompleted = $state(true);

	setDriveChrome({ user: MOCK_DRIVE_USER, quota: MOCK_QUOTA });

	const activeId = $derived(deriveDriveActiveId(page.url.pathname));

	onMount(async () => {
		try {
			const status = await getSetupStatus();
			isCompleted = status.is_setup_completed;
		} catch (e) {
			console.warn('[Drive Guard] Could not verify setup status:', e);
		} finally {
			checking = false;
		}
		if (isCompleted) {
			setMockLoggedIn(true);
			driveView.init();
		}
	});
</script>

{#if !checking}
	{#if !isCompleted}
		<ErrorPage
			status={403}
			message="Setup sistem belum diselesaikan. Silakan selesaikan setup terlebih dahulu."
		/>
	{:else}
		<DriveShell {activeId}>
			{#snippet content()}
				{@render children()}
			{/snippet}
		</DriveShell>
	{/if}
{/if}
