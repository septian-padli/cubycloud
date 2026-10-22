<script lang="ts">
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import FileExplorer from '$lib/components/drive/explorer/file-explorer.svelte';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Building2 } from '@lucide/svelte';
	import { locale } from '$lib/locale.svelte.js';

	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	const orgBadgeClass = 'bg-primary/15 text-primary border-transparent font-medium';
</script>

<svelte:head>
	<title>{tx('Organisasi — CubyCloud', 'Organization — CubyCloud')}</title>
	<meta
		name="description"
		content={tx('Drive bersama organisasi CubyCloud.', 'CubyCloud organization shared drive.')}
	/>
</svelte:head>

<div class="flex flex-col gap-6">
	<DrivePageHeader
		title={tx('Organisasi', 'Organization')}
		description={tx(
			'Drive bersama tim. Tidak dihitung ke kuota My Storage.',
			'Shared team drive. It does not count toward My Storage quota.'
		)}
	>
		{#snippet actions()}
			<Badge class={orgBadgeClass}>{tx('🏢 Organisasi', '🏢 Organization')}</Badge>
		{/snippet}
	</DrivePageHeader>

	<Alert.Root variant="default">
		<Building2 class="size-4 stroke-2" />
		<Alert.Title>{tx('Drive bersama tim', 'Shared team drive')}</Alert.Title>
		<Alert.Description>
			{tx(
				'Semua anggota dapat membaca, menulis, mengunduh, dan menghapus di pohon Organisasi. Berkas di sini tidak mengurangi kuota My Storage Anda.',
				'All members can read, write, download, and delete within the Organization tree. Files here do not count toward your My Storage quota.'
			)}
		</Alert.Description>
	</Alert.Root>

	<FileExplorer variant="organization" initialTree={[]} rootLabel={tx('Organisasi', 'Organization')} />
</div>
