<script lang="ts">
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import TrashFileList from '$lib/components/drive/trash-file-list.svelte';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);
</script>

<svelte:head>
	<title>{tx('Sampah', 'Trash')} — CubyCloud</title>
	<meta
		name="description"
		content={tx(
			'Sampah CubyCloud: pulihkan atau hapus permanen.',
			'CubyCloud trash: restore items or permanently delete them.'
		)}
	/>
</svelte:head>

<div class="flex flex-col gap-6">
	<DrivePageHeader
		title={tx('Sampah', 'Trash')}
		description={tx(
			'Item yang dihapus akan disimpan di sini sebelum dihapus permanen setelah 30 hari.',
			'Deleted items are kept here before being permanently deleted after 30 days.'
		)}
	/>

	<TrashFileList />
</div>
