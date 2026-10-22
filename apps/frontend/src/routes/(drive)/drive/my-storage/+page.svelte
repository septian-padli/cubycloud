<script lang="ts">
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import FileExplorer from '$lib/components/drive/explorer/file-explorer.svelte';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);
	// API-backed My Storage data is loaded by FileExplorer.
</script>

<svelte:head>
	<title>{tx('My Storage', 'My Storage')} — CubyCloud</title>
	<meta
		name="description"
		content={tx(
			'Jelajahi dan kelola berkas My Storage.',
			'Browse and manage your My Storage files.'
		)}
	/>
</svelte:head>

<div class="flex flex-col gap-6">
	<DrivePageHeader
		title={tx('My Storage', 'My Storage')}
		description={tx(
			'Berkas pribadi Anda. Navigasi folder lewat breadcrumb atau klik ganda.',
			'Your personal files. Navigate folders using breadcrumbs or double-click.'
		)}
	/>

	<FileExplorer variant="my-storage" initialTree={[]} rootLabel="My Storage" />
</div>
