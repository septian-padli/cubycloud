<script lang="ts">
	import type { FileEntry } from '$lib/types/drive.js';
	import type { PreviewSource } from '$lib/api/preview.js';
	import { previewUrl } from '$lib/api/preview.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import FileKindIcon from './file-kind-icon.svelte';
	import PdfPreview from './pdf-preview.svelte';
	import TextPreview from './text-preview.svelte';
	import VideoPreview from './video-preview.svelte';

	let {
		open = $bindable(false),
		entry = null,
		source,
		token,
		rootFile = false,
		onDownload,
		onClose
	}: {
		open?: boolean;
		entry?: FileEntry | null;
		source?: PreviewSource;
		token?: string;
		rootFile?: boolean;
		onDownload?: (entry: FileEntry) => void;
		onClose?: () => void;
	} = $props();

	const extension = $derived(entry?.name.split('.').pop()?.toLowerCase() ?? '');
	const format = $derived.by(() => {
		if (!entry || entry.kind === 'folder') return 'unsupported';
		if (['png', 'jpg', 'jpeg', 'webp', 'gif', 'svg'].includes(extension)) return 'image';
		if (['mp4', 'webm'].includes(extension)) return 'video';
		if (['mp3', 'wav', 'ogg'].includes(extension)) return 'audio';
		if (extension === 'pdf') return 'pdf';
		if (['txt', 'md', 'json', 'csv', 'log', 'sql', 'js', 'ts', 'html', 'css'].includes(extension)) return 'text';
		return 'unsupported';
	});
	const url = $derived(entry && source ? previewUrl(source, entry.id, token, rootFile) : '');

	function setOpen(next: boolean) {
		open = next;
		if (!next) onClose?.();
	}
</script>

<Dialog.Root bind:open={() => open, (v) => setOpen(v)}>
	<Dialog.Content class="max-h-screen max-w-4xl overflow-y-auto border-subtle bg-surface text-main shadow-none sm:max-w-4xl">
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

			<div class="flex min-h-48 items-center justify-center rounded-xl border border-subtle bg-app p-4 text-center">
				{#if open && url && format === 'image'}
					<img src={url} alt={entry.name} class="max-h-96 max-w-full object-contain" data-testid="image-preview" onerror={(e) => {
						e.currentTarget.setAttribute('hidden', '');
						e.currentTarget.nextElementSibling?.classList.remove('hidden');
					}} />
					<p role="alert" class="hidden text-sm text-muted">Gambar gagal dimuat. Unduh berkas untuk membukanya.</p>
				{:else if open && url && format === 'video'}
					{#key url}<VideoPreview {url} name={entry.name} />{/key}
				{:else if open && url && format === 'audio'}
					<audio src={url} controls preload="metadata" class="w-full" aria-label={`Pemutar audio ${entry.name}`} onerror={(e) => {
						e.currentTarget.setAttribute('hidden', '');
						e.currentTarget.nextElementSibling?.classList.remove('hidden');
					}}></audio>
					<p role="alert" class="hidden text-sm text-muted">Audio gagal dimuat. Unduh berkas untuk membukanya.</p>
				{:else if open && url && format === 'pdf'}
					{#key url}<PdfPreview {url} />{/key}
				{:else if open && url && format === 'text'}
					{#key url}<TextPreview {url} {extension} />{/key}
				{:else}
					<p class="text-sm text-muted">Tidak ada pratinjau untuk jenis berkas ini. Unduh berkas untuk membukanya.</p>
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
