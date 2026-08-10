<script lang="ts">
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import DriveViewToggle from '$lib/components/drive/drive-view-toggle.svelte';
	import SharedFileList from '$lib/components/drive/shared-file-list.svelte';
	import { MOCK_SHARED_FILES } from '$lib/mocks/shared-files.js';
</script>

<svelte:head>
	<title>Dibagikan pada saya — CubyCloud</title>
	<meta name="description" content="Berkas yang dibagikan ke akun CubyCloud Anda." />
</svelte:head>

<div class="flex flex-col gap-6">
	<DrivePageHeader
		title="Dibagikan pada saya"
		description="Berkas dari pengguna lain. Izin Viewer atau Editor menentukan aksi yang tersedia."
	>
		{#snippet actions()}
			<DriveViewToggle />
		{/snippet}
	</DrivePageHeader>

	<SharedFileList initialItems={MOCK_SHARED_FILES} />
</div>
