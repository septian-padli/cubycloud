<script lang="ts">
	import { onMount } from 'svelte';
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import DriveViewToggle from '$lib/components/drive/drive-view-toggle.svelte';
	import HomeQuickAccess from '$lib/components/drive/home-quick-access.svelte';
	import HomeRecentActivities from '$lib/components/drive/home-recent-activities.svelte';
	import HomeStorageSummary from '$lib/components/drive/home-storage-summary.svelte';
	import { getDriveChrome } from '$lib/drive-context.svelte.js';
	import { fetchHomeData } from '$lib/api/drive-home.js';
	import type { ActivityItem, QuickAccessItem } from '$lib/types/drive.js';

	const chrome = getDriveChrome();
	let quickAccess = $state<QuickAccessItem[]>([]);
	let activities = $state<ActivityItem[]>([]);
	let loading = $state(true);

	onMount(async () => {
		try {
			const data = await fetchHomeData();
			quickAccess = data.quickAccess;
			activities = data.activities;
		} catch (e) {
			console.warn('[Home] Failed to fetch home data:', e);
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>Beranda — CubyCloud</title>
	<meta name="description" content="Beranda CubyCloud: akses cepat, aktivitas, dan ringkasan kuota." />
</svelte:head>

<div class="flex flex-col gap-6">
	<DrivePageHeader
		eyebrow="Selamat datang"
		title="Beranda"
		description="Akses cepat berkas, aktivitas terbaru, dan ringkasan My Storage."
	>
		{#snippet actions()}
			<DriveViewToggle />
		{/snippet}
	</DrivePageHeader>

	{#if loading}
		<div class="space-y-6 animate-pulse" data-testid="home-loading">
			<div class="h-40 rounded-xl border border-subtle bg-surface/50"></div>
			<div class="h-40 rounded-xl border border-subtle bg-surface/50"></div>
		</div>
	{:else}
		<HomeQuickAccess items={quickAccess} />
		<HomeRecentActivities items={activities} />
	{/if}
	<HomeStorageSummary quota={chrome.quota} />
</div>
