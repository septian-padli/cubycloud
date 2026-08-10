<script lang="ts">
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import FileExplorer from '$lib/components/drive/explorer/file-explorer.svelte';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { MOCK_ORG_TREE } from '$lib/mocks/file-tree.js';
	import { Building2 } from '@lucide/svelte';

	const orgBadgeClass = 'bg-primary/15 text-primary border-transparent font-medium';
</script>

<svelte:head>
	<title>Organization — CubyCloud</title>
	<meta name="description" content="Drive bersama organisasi CubyCloud." />
</svelte:head>

<div class="flex flex-col gap-6">
	<DrivePageHeader
		title="Organization"
		description="Drive bersama tim. Tidak dihitung ke kuota My Storage."
	>
		{#snippet actions()}
			<Badge class={orgBadgeClass}>🏢 Organization</Badge>
		{/snippet}
	</DrivePageHeader>

	<Alert.Root class="rounded-xl border-subtle bg-surface text-main shadow-none">
		<Building2 class="size-4 stroke-2 text-primary" />
		<Alert.Title>Drive bersama tim</Alert.Title>
		<Alert.Description>
			Semua anggota dapat membaca, menulis, mengunduh, dan menghapus di pohon Organization.
			Berkas di sini tidak mengurangi kuota My Storage Anda.
		</Alert.Description>
	</Alert.Root>

	<FileExplorer
		variant="organization"
		initialTree={MOCK_ORG_TREE}
		rootLabel="Organization"
	/>
</div>
