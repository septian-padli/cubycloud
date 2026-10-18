<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { getSetupStatus } from '$lib/api/setup';
	import { goto } from '$app/navigation';
	import { authAPI } from '$lib/api/auth.js';
	import DriveShell from '$lib/components/drive/drive-shell.svelte';
	import SetupRequiredPage from '$lib/components/setup/setup-required-page.svelte';
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
			if (!isCompleted) {
				checking = false;
				return;
			}

			// Verify authentication with backend
			const meResp = await authAPI.getMe();
			if (meResp.error || !meResp.data) {
				const currentPath = page.url.pathname + page.url.search;
				void goto(`/login?redirect=${encodeURIComponent(currentPath)}`);
				return;
			}

			setMockLoggedIn(true);
			driveView.init();
		} catch (e) {
			console.warn('[Drive Guard] Could not verify session:', e);
		} finally {
			checking = false;
		}
	});
</script>

{#if !checking}
	{#if !isCompleted}
		<SetupRequiredPage />
	{:else}
		<DriveShell {activeId}>
			{#snippet content()}
				{@render children()}
			{/snippet}
		</DriveShell>
	{/if}
{/if}
