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
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

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
	<title>{tx('Penyimpanan', 'Storage')} — CubyCloud</title>
	<meta
		name="description"
		content={tx(
			'Detail kuota My Storage dan analisis pemakaian.',
			'My Storage quota details and usage analysis.'
		)}
	/>
</svelte:head>

<div class="flex flex-col gap-8">
	<DrivePageHeader
		title={tx('Penyimpanan', 'Storage')}
		description={tx(
			'Ringkasan kuota My Storage, pemakaian per kategori, dan berkas terbesar.',
			'My Storage quota summary, usage by category, and largest files.'
		)}
	/>

	<section class="space-y-3">
		<h2 class="text-sm font-semibold text-main">
			{tx('Detail kuota & pemakaian My Storage', 'My Storage quota and usage details')}
		</h2>
		<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
			<Card.Content class="space-y-5 p-5">
				<SegmentedStorageBar usedGb={chrome.quota.usedGb} limitGb={quotaLimitGb} {categories} />
				<div class="border-t border-subtle pt-3">
					<p class="text-xs text-muted">
						{tx(
							'Kuota My Storage tidak menghitung berkas Organization. Item di Trash tetap dihitung sampai dihapus permanen.',
							'My Storage quota excludes Organization files. Items in Trash count until they are permanently deleted.'
						)}
					</p>
				</div>
			</Card.Content>
		</Card.Root>
	</section>
	<StorageLargestFiles trashCount={MOCK_TRASH_FILES.length} />
</div>
