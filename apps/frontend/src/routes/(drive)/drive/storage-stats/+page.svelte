<script lang="ts">
	import { onMount } from 'svelte';
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import SegmentedStorageBar from '$lib/components/drive/segmented-storage-bar.svelte';
	import StorageLargestFiles from '$lib/components/drive/storage-largest-files.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import { getDriveChrome } from '$lib/drive-context.svelte.js';
	import { myStorageAPI } from '$lib/api/my-storage.js';
	import type { CategoryUsage } from '$lib/types/drive.js';
	import { MOCK_TRASH_FILES } from '$lib/mocks/trash-files.js';

	const chrome = getDriveChrome();

	const quotaLimitGb = $derived(chrome.user.role === 'super_admin' ? null : chrome.quota.limitGb);
	let categories = $state<CategoryUsage[]>([]);
	let loading = $state(true);

	onMount(async () => {
		try {
			const res = await myStorageAPI.categoryUsage();
			if (res && res.categories) {
				categories = res.categories.map((c) => ({
					id: c.id,
					label: c.label,
					usedGb: c.used_gb,
					bytes: c.bytes
				}));
				if (chrome.quota && res.total_gb != null) {
					chrome.quota.usedGb = res.total_gb;
				}
			}
		} catch (e) {
			console.warn('[storage-stats] Failed to fetch category usage:', e);
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>Penyimpanan — CubyCloud</title>
	<meta name="description" content="Detail kuota My Storage dan analisis pemakaian." />
</svelte:head>

<div class="flex flex-col gap-8">
	<DrivePageHeader
		title="Penyimpanan"
		description="Ringkasan kuota My Storage, pemakaian per kategori, dan berkas terbesar."
	/>

	<section class="space-y-3">
		<h2 class="text-sm font-semibold text-main">Detail kuota & pemakaian My Storage</h2>
		<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
			<Card.Content class="space-y-5 p-5">
				<SegmentedStorageBar usedGb={chrome.quota.usedGb} limitGb={quotaLimitGb} {categories} />
				<div class="border-t border-subtle pt-3">
					<p class="text-xs text-muted">
						Kuota My Storage tidak menghitung berkas Organization. Item di Trash tetap dihitung
						sampai dihapus permanen.
					</p>
				</div>
			</Card.Content>
		</Card.Root>
	</section>
	<StorageLargestFiles trashCount={MOCK_TRASH_FILES.length} />
</div>
