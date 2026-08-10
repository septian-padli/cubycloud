<script lang="ts">
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import FileExplorer from '$lib/components/drive/explorer/file-explorer.svelte';
	import { MOCK_MY_STORAGE_TREE } from '$lib/mocks/file-tree.js';
</script>

<svelte:head>
	<title>My Storage — CubyCloud</title>
	<meta name="description" content="Jelajahi dan kelola berkas My Storage." />
</svelte:head>

<div class="flex flex-col gap-6">
	<DrivePageHeader
		title="My Storage"
		description="Berkas pribadi Anda. Navigasi folder lewat breadcrumb atau klik ganda."
	/>

	<FileExplorer
		variant="my-storage"
		initialTree={MOCK_MY_STORAGE_TREE}
		rootLabel="My Storage"
	/>
</div>
