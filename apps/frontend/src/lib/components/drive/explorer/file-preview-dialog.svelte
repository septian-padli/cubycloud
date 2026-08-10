<script lang="ts">
	import type { FileEntry } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import FileKindIcon from './file-kind-icon.svelte';

	let {
		open = $bindable(false),
		entry = null,
		onDownload,
		onClose
	}: {
		open?: boolean;
		entry?: FileEntry | null;
		onDownload?: (entry: FileEntry) => void;
		onClose?: () => void;
	} = $props();

	function setOpen(next: boolean) {
		open = next;
		if (!next) onClose?.();
	}
</script>

<Dialog.Root bind:open={() => open, (v) => setOpen(v)}>
	<Dialog.Content class="max-w-lg bg-surface text-main sm:max-w-lg">
		{#if entry}
			<Dialog.Header>
				<Dialog.Title class="flex items-center gap-3">
					<FileKindIcon kind={entry.kind} />
					<span class="truncate">{entry.name}</span>
				</Dialog.Title>
				<Dialog.Description>
					{formatSize(entry.sizeBytes)} · Diubah {entry.modifiedAt}
				</Dialog.Description>
			</Dialog.Header>

			<div
				class="flex min-h-48 items-center justify-center rounded-xl border border-dashed border-subtle bg-app p-6 text-center"
			>
				{#if entry.kind === 'image'}
					<p class="text-sm text-muted">Pratinjau gambar (placeholder mock)</p>
				{:else if entry.kind === 'video'}
					<p class="text-sm text-muted">Pratinjau video (placeholder mock)</p>
				{:else if entry.kind === 'audio'}
					<p class="text-sm text-muted">Pemutar audio (placeholder mock)</p>
				{:else if entry.kind === 'document' || entry.kind === 'text'}
					<p class="text-sm text-muted">Pratinjau dokumen (placeholder mock)</p>
				{:else}
					<p class="text-sm text-muted">Tidak ada pratinjau untuk jenis berkas ini</p>
				{/if}
			</div>

			<Dialog.Footer>
				<Button type="button" variant="secondary" onclick={() => setOpen(false)}>Tutup</Button>
				{#if onDownload}
					<Button type="button" onclick={() => onDownload(entry)}>Unduh</Button>
				{/if}
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>
