<script lang="ts">
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import DriveViewToggle from '$lib/components/drive/drive-view-toggle.svelte';
	import SharedFileList from '$lib/components/drive/shared-file-list.svelte';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);
</script>

<svelte:head>
	<title>{tx('Dibagikan pada saya', 'Shared with me')} — CubyCloud</title>
	<meta
		name="description"
		content={tx(
			'Berkas yang dibagikan ke akun CubyCloud Anda.',
			'Files shared with your CubyCloud account.'
		)}
	/>
</svelte:head>

<div class="flex flex-col gap-6">
	<DrivePageHeader
		title={tx('Dibagikan pada saya', 'Shared with me')}
		description={tx(
			'Berkas dari pengguna lain. Izin Viewer atau Editor menentukan aksi yang tersedia.',
			'Files from other users. Viewer or Editor permissions determine available actions.'
		)}
	>
		{#snippet actions()}
			<DriveViewToggle />
		{/snippet}
	</DrivePageHeader>

	<SharedFileList />
</div>
