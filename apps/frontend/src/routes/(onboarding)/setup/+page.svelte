<script lang="ts">
	import { onMount } from 'svelte';
	import { getSetupStatus } from '$lib/api/setup';
	import SetupWizard from '$lib/components/setup/setup-wizard.svelte';
	import ErrorPage from '$lib/components/error/error-page.svelte';

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
	<title>CubyCloud — Setup</title>
	<meta name="description" content="Wizard konfigurasi awal CubyCloud." />
</svelte:head>

{#if !checking}
	{#if isCompleted}
		<ErrorPage
			showHeader={false}
			status={403}
			message="Setup awal telah selesai. Halaman setup tidak dapat diakses kembali."
		/>
	{:else}
		<SetupWizard />
	{/if}
{/if}
