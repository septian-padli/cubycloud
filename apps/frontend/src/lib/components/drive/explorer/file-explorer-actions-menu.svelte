<script lang="ts">
	import type { FileEntry } from '$lib/types/drive.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		Copy,
		Download,
		Eye,
		FolderInput,
		MoreVertical,
		Pencil,
		Share2,
		Star,
		Trash2
	} from '@lucide/svelte';
	import { cn } from '$lib/utils.js';

	let {
		entry,
		onOpen,
		onDownload,
		onRename,
		onMove,
		onCopy,
		onStar,
		onShare,
		onDelete
	}: {
		entry: FileEntry;
		onOpen: (entry: FileEntry) => void;
		onDownload: (entry: FileEntry) => void;
		onRename: (entry: FileEntry) => void;
		onMove: (entry: FileEntry) => void;
		onCopy: (entry: FileEntry) => void;
		onStar: (entry: FileEntry) => void;
		onShare: (entry: FileEntry) => void;
		onDelete: (entry: FileEntry) => void;
	} = $props();
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger>
		{#snippet child({ props })}
			<Button
				{...props}
				type="button"
				variant="ghost"
				size="icon"
				class="bg-surface text-muted hover:bg-primary hover:text-white aria-expanded:bg-primary aria-expanded:text-white"
				aria-label="Aksi berkas"
				onclick={(e: MouseEvent) => e.stopPropagation()}
			>
				<MoreVertical class="size-4 stroke-2" />
			</Button>
		{/snippet}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="end" class="min-w-48">
		{#if entry.kind !== 'folder'}
			<DropdownMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
				<Eye class="size-4 stroke-2" />
				Pratinjau
			</DropdownMenu.Item>
			<DropdownMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
				<Download class="size-4 stroke-2" />
				Unduh
			</DropdownMenu.Item>
		{:else}
			<DropdownMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
				<Eye class="size-4 stroke-2" />
				Buka
			</DropdownMenu.Item>
			<DropdownMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
				<Download class="size-4 stroke-2" />
				Unduh
			</DropdownMenu.Item>
		{/if}
		<DropdownMenu.Item class="gap-2" onclick={() => onRename(entry)}>
			<Pencil class="size-4 stroke-2" />
			Ganti nama
		</DropdownMenu.Item>
		<DropdownMenu.Item class="gap-2" onclick={() => onMove(entry)}>
			<FolderInput class="size-4 stroke-2" />
			Pindahkan ke
		</DropdownMenu.Item>
		<DropdownMenu.Item class="gap-2" onclick={() => onCopy(entry)}>
			<Copy class="size-4 stroke-2" />
			Duplikat
		</DropdownMenu.Item>
		<DropdownMenu.Item class="gap-2" onclick={() => onStar(entry)}>
			<Star class={cn("size-4 stroke-2", entry.starred ? "fill-amber-400 stroke-amber-500 text-amber-500" : "")} />
			{entry.starred ? 'Hapus Bintang' : 'Bintang'}
		</DropdownMenu.Item>
		<DropdownMenu.Item class="gap-2" onclick={() => onShare(entry)}>
			<Share2 class="size-4 stroke-2" />
			Bagikan
		</DropdownMenu.Item>
		<DropdownMenu.Separator />
		<DropdownMenu.Item variant="destructive" class="gap-2" onclick={() => onDelete(entry)}>
			<Trash2 class="size-4 stroke-2" />
			Hapus ke Trash
		</DropdownMenu.Item>
	</DropdownMenu.Content>
</DropdownMenu.Root>
