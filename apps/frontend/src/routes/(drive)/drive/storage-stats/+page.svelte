<script lang="ts">
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import StorageQuotaDetail from '$lib/components/drive/storage-quota-detail.svelte';
	import StorageCategoryChart from '$lib/components/drive/storage-category-chart.svelte';
	import StorageLargestFiles from '$lib/components/drive/storage-largest-files.svelte';
	import { getDriveChrome } from '$lib/drive-context.svelte.js';
	import { MOCK_CATEGORY_USAGE, MOCK_LARGEST_FILES } from '$lib/mocks/storage-stats.js';
	import { MOCK_TRASH_FILES } from '$lib/mocks/trash-files.js';

	const { quota } = getDriveChrome();
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

	<StorageQuotaDetail {quota} />
	<StorageCategoryChart categories={MOCK_CATEGORY_USAGE} />
	<StorageLargestFiles files={MOCK_LARGEST_FILES} trashCount={MOCK_TRASH_FILES.length} />
</div>
