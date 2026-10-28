<script lang="ts">
	import { onMount } from 'svelte';
	import { getSetupStatus } from '$lib/api/setup';
	import SetupWizard from '$lib/components/setup/setup-wizard.svelte';
	import ErrorPage from '$lib/components/error/error-page.svelte';
	import { locale } from '$lib/locale.svelte.js';

	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let checking = $state(true);
	let isCompleted = $state(false);

	onMount(async () => {
		try {
			const status = await getSetupStatus();
			isCompleted = status.is_setup_completed;
		} catch (e) {
			console.warn('[Setup Guard] Could not verify setup status:', e);
		} finally {
			checking = false;
		}
	});
</script>

<svelte:head>
	<title>CubyCloud — {tx('Penyiapan', 'Setup')}</title>
	<meta
		name="description"
		content={tx('Wizard konfigurasi awal CubyCloud.', 'CubyCloud initial configuration wizard.')}
	/>
</svelte:head>

{#if !checking}
	{#if isCompleted}
		<ErrorPage
			showHeader={false}
			status={403}
			message={tx(
				'Setup awal telah selesai. Halaman setup tidak dapat diakses kembali.',
				'Initial setup is complete. This setup page can no longer be accessed.'
			)}
		/>
	{:else}
		<SetupWizard />
	{/if}
{/if}
