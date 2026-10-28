<script lang="ts">
	import { locale } from '$lib/locale.svelte.js';
	import type { SharedFileItem } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import PermissionBadge from './permission-badge.svelte';
	import FileKindIcon from './explorer/file-kind-icon.svelte';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let {
		open = $bindable(false),
		entry = null
	}: {
		open?: boolean;
		entry?: SharedFileItem | null;
	} = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main">
		{#if entry}
			<Dialog.Header>
				<Dialog.Title class="flex items-center gap-3">
					<FileKindIcon kind={entry.kind} />
					<span class="truncate">{tx('Detail berbagi', 'Sharing details')}</span>
				</Dialog.Title>
				<Dialog.Description>
					{tx(
						`Informasi pemilik dan izin untuk “${entry.name}”.`,
						`Owner and permission details for “${entry.name}”.`
					)}
				</Dialog.Description>
			</Dialog.Header>

			<div class="grid gap-3 rounded-xl border border-subtle bg-app p-4">
				<div class="flex items-center justify-between gap-3">
					<p class="text-xs font-medium text-muted">{tx('Berkas', 'File')}</p>
					<p class="truncate text-sm font-medium text-main">{entry.name}</p>
				</div>
				<div class="flex items-center justify-between gap-3">
					<p class="text-xs font-medium text-muted">{tx('Ukuran', 'Size')}</p>
					<p class="text-sm text-main">{formatSize(entry.sizeBytes)}</p>
				</div>
				<div class="flex items-center justify-between gap-3">
					<p class="text-xs font-medium text-muted">{tx('Pemilik', 'Owner')}</p>
					<p class="text-sm text-main">{entry.ownerName}</p>
				</div>
				<div class="flex items-center justify-between gap-3">
					<p class="text-xs font-medium text-muted">{tx('Izin Anda', 'Your permission')}</p>
					<PermissionBadge permission={entry.permission} />
				</div>
				<p class="text-xs text-muted">
					{tx(
						'Kuota dihitung ke pemilik berbagi — tidak menambah usage My Storage Anda.',
						'Usage counts toward the owner’s quota and does not add to your My Storage usage.'
					)}
				</p>
			</div>

			<Dialog.Footer>
				<Button type="button" variant="secondary" onclick={() => (open = false)}
					>{tx('Tutup', 'Close')}</Button
				>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>
