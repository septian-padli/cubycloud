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
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

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
				toast.error(
					`${tx('Gagal memuat statistik penyimpanan', 'Failed to load storage statistics')}: ${res.error}`
				);
			}
		} catch {
			toast.error(tx('Gagal menghubungi server.', 'Failed to reach the server.'));
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	async function handleRefresh() {
		refreshing = true;
		await fetchStats();
		toast.success(tx('Statistik penyimpanan diperbarui.', 'Storage statistics refreshed.'));
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
	<title>{tx('Penyimpanan Sistem', 'System Storage')} — CubyCloud</title>
	<meta
		name="description"
		content={tx(
			'Ringkasan kapasitas seluruh storage node dan konsumsi penyimpanan per pengguna.',
			'Overview of capacity across all storage nodes and storage usage by user.'
		)}
	/>
</svelte:head>

<div class="flex flex-col gap-8 pb-12">
	<DrivePageHeader
		title={tx('Penyimpanan Sistem', 'System Storage')}
		description={tx(
			'Ringkasan kapasitas seluruh storage node dan konsumsi penyimpanan per pengguna.',
			'Overview of capacity across all storage nodes and storage usage by user.'
		)}
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
				<span>{tx('Segarkan', 'Refresh')}</span>
			</Button>
		{/snippet}
	</DrivePageHeader>

	<AdminStorageOverview {stats} {loading} />

	<AdminTopStorageUsers totalSystemUsedBytes={stats?.total_used_bytes ?? 0} />
</div>
