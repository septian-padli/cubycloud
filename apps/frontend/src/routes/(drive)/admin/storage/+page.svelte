<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { toast } from 'svelte-sonner';
	import { RefreshCw } from '@lucide/svelte';
	import { getDriveChrome } from '$lib/drive-context.svelte.js';
	import { adminAPI } from '$lib/api/admin.js';
	import type { AdminStorageStats } from '$lib/types/admin.js';
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import AdminStorageOverview from '$lib/components/admin/admin-storage-overview.svelte';
	import AdminTopStorageUsers from '$lib/components/admin/admin-top-storage-users.svelte';
	import { Button } from '$lib/components/ui/button/index.js';

	const chrome = getDriveChrome();
	let stats = $state<AdminStorageStats | null>(null);
	let loading = $state(true);
	let refreshing = $state(false);

	async function fetchStats() {
		try {
			const res = await adminAPI.getStorageStats();
			if (res.data) {
				stats = res.data;
			} else if (res.error) {
				toast.error(`Gagal memuat statistik penyimpanan: ${res.error}`);
			}
		} catch {
			toast.error('Gagal menghubungi server.');
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	async function handleRefresh() {
		refreshing = true;
		await fetchStats();
		toast.success('Statistik penyimpanan diperbarui.');
	}

	onMount(() => {
		if (browser && chrome?.user?.role && chrome.user.role !== 'super_admin') {
			void goto('/drive/home');
			return;
		}
		void fetchStats();
	});
</script>

<svelte:head>
	<title>Penyimpanan Sistem — CubyCloud</title>
	<meta
		name="description"
		content="Ringkasan kapasitas seluruh storage node dan konsumsi penyimpanan per pengguna."
	/>
</svelte:head>

<div class="flex flex-col gap-8 pb-12">
	<DrivePageHeader
		title="Penyimpanan Sistem"
		description="Ringkasan kapasitas seluruh storage node dan konsumsi penyimpanan per pengguna."
	>
		{#snippet actions()}
			<Button
				variant="outline"
				size="sm"
				class="gap-1.5"
				disabled={refreshing || loading}
				onclick={handleRefresh}
			>
				<RefreshCw class="size-3.5 {refreshing ? 'animate-spin' : ''}" />
				<span>Segarkan</span>
			</Button>
		{/snippet}
	</DrivePageHeader>

	<AdminStorageOverview {stats} {loading} />

	<AdminTopStorageUsers totalSystemUsedBytes={stats?.total_used_bytes ?? 0} />
</div>
