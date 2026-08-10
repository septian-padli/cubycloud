<script lang="ts">
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import DriveViewToggle from '$lib/components/drive/drive-view-toggle.svelte';
	import StarredFileList from '$lib/components/drive/starred-file-list.svelte';
	import { MOCK_STARRED_FILES } from '$lib/mocks/starred-files.js';
</script>

<svelte:head>
	<title>Berbintang — CubyCloud</title>
	<meta
		name="description"
		content="Berkas berbintang dari My Storage, Organization, dan Shared."
	/>
</svelte:head>

<div class="flex flex-col gap-6">
	<DrivePageHeader
		title="Berbintang"
		description="Akses cepat ke berkas yang Anda tandai dari My Storage, Organization, atau Shared."
	>
		{#snippet actions()}
			<DriveViewToggle />
		{/snippet}
	</DrivePageHeader>

	<StarredFileList initialItems={MOCK_STARRED_FILES} />
</div>
