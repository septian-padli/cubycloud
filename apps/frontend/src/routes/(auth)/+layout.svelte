<script lang="ts">
	import { onMount } from 'svelte';
	import { getSetupStatus } from '$lib/api/setup';
	import AuthSplitShell from '$lib/components/auth/auth-split-shell.svelte';
	import ErrorPage from '$lib/components/error/error-page.svelte';
	import { locale } from '$lib/locale.svelte.js';

	let { children } = $props();

	let checking = $state(true);
	let isCompleted = $state(true);

	onMount(async () => {
		try {
			const status = await getSetupStatus();
			isCompleted = status.is_setup_completed;
		} catch (e) {
			console.warn('[Auth Guard] Could not verify setup status:', e);
		} finally {
			checking = false;
		}
	});
</script>

{#if !checking}
	{#if !isCompleted}
		<ErrorPage
			status={403}
			message={locale.language === 'id'
				? 'Setup sistem belum diselesaikan. Silakan selesaikan setup terlebih dahulu.'
				: 'System setup is incomplete. Please complete setup first.'}
		/>
	{:else}
		<AuthSplitShell>
			{@render children()}
		</AuthSplitShell>
	{/if}
{/if}
