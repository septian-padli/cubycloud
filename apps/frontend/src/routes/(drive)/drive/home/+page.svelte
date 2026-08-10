<script lang="ts">
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import HomeQuickAccess from '$lib/components/drive/home-quick-access.svelte';
	import HomeRecentActivities from '$lib/components/drive/home-recent-activities.svelte';
	import HomeStorageSummary from '$lib/components/drive/home-storage-summary.svelte';
	import { getDriveChrome } from '$lib/drive-context.svelte.js';
	import { MOCK_ACTIVITIES, MOCK_QUICK_ACCESS } from '$lib/mocks/drive.js';

	const { quota } = getDriveChrome();
</script>

<svelte:head>
	<title>Beranda — CubyCloud</title>
	<meta name="description" content="Beranda CubyCloud: akses cepat, aktivitas, dan ringkasan kuota." />
</svelte:head>

<DrivePageHeader
	eyebrow="Selamat datang"
	title="Beranda"
	description="Akses cepat berkas, aktivitas terbaru, dan ringkasan My Storage."
/>

<HomeQuickAccess items={MOCK_QUICK_ACCESS} />
<HomeRecentActivities items={MOCK_ACTIVITIES} />
<HomeStorageSummary {quota} />
