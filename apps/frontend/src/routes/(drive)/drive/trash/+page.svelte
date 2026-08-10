<script lang="ts">
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import TrashFileList from '$lib/components/drive/trash-file-list.svelte';
	import { MOCK_TRASH_FILES } from '$lib/mocks/trash-files.js';
</script>

<svelte:head>
	<title>Trash — CubyCloud</title>
	<meta name="description" content="Sampah CubyCloud: pulihkan atau hapus permanen." />
</svelte:head>

<div class="flex flex-col gap-6">
	<DrivePageHeader title="Trash" />

	<TrashFileList initialItems={MOCK_TRASH_FILES} />
</div>
