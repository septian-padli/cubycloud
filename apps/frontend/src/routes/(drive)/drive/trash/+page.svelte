<script lang="ts">
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import TrashFileList from '$lib/components/drive/trash-file-list.svelte';
</script>

<svelte:head>
	<title>Trash — CubyCloud</title>
	<meta name="description" content="Sampah CubyCloud: pulihkan atau hapus permanen." />
</svelte:head>

<div class="flex flex-col gap-6">
	<DrivePageHeader
		title="Trash"
		description="Item yang dihapus akan disimpan di sini sebelum dihapus permanen setelah 30 hari."
	/>

	<TrashFileList />
</div>
