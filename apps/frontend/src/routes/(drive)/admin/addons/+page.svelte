<script lang="ts">
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import AddonToggleCard from '$lib/components/admin/addon-toggle-card.svelte';
	import WorkerQueueDrawer from '$lib/components/admin/worker-queue-drawer.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { cloneMockAddons } from '$lib/mocks/addons.js';
	import { cloneMockJobQueue } from '$lib/mocks/job-queue.js';
	import type { AddonConfig } from '$lib/types/admin.js';
	import { ListOrdered } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	// ponytail: add +layout.server role check when API exists
	let addons = $state(cloneMockAddons());
	const jobs = cloneMockJobQueue();
	let queueOpen = $state(false);

	function handleToggle(addon: AddonConfig, enabled: boolean) {
		toast.success(enabled ? `${addon.name} diaktifkan` : `${addon.name} dinonaktifkan`);
	}
</script>

<svelte:head>
	<title>Add-ons Manager — CubyCloud</title>
	<meta name="description" content="Kelola add-on Phase-1 dan pantau antrean job worker." />
</svelte:head>

<div class="flex flex-col gap-8">
	<DrivePageHeader
		title="Add-ons Manager"
		description="Aktifkan atau nonaktifkan add-on Phase-1, lalu pantau antrean job worker (SQLite)."
	>
		{#snippet actions()}
			<Button type="button" variant="secondary" onclick={() => (queueOpen = true)}>
				<ListOrdered class="size-4 stroke-2" aria-hidden="true" />
				Lihat Antrean Job (SQLite Queue)
			</Button>
		{/snippet}
	</DrivePageHeader>

	<div class="grid gap-4 sm:grid-cols-2">
		{#each addons as addon, i (addon.id)}
			<AddonToggleCard
				bind:addon={addons[i]}
				onToggle={(enabled) => handleToggle(addon, enabled)}
			/>
		{/each}
	</div>
</div>

<WorkerQueueDrawer bind:open={queueOpen} {jobs} />
