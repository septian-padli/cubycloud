<script lang="ts">
	import type { FileCategory } from '$lib/types/drive.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import DriveViewToggle from '$lib/components/drive/drive-view-toggle.svelte';
	import { Plus, FolderPlus, Upload, ChevronDown, FolderUp } from '@lucide/svelte';
	import { cn } from '$lib/utils.js';

	const FILTERS: { id: FileCategory; label: string }[] = [
		{ id: 'all', label: 'Semua' },
		{ id: 'folder', label: 'Folder' },
		{ id: 'image', label: 'Gambar' },
		{ id: 'video', label: 'Video' },
		{ id: 'audio', label: 'Audio' },
		{ id: 'document', label: 'Dokumen' },
		{ id: 'text', label: 'Teks' },
		{ id: 'archive', label: 'Arsip' }
	];

	let {
		category = $bindable('all' as FileCategory),
		onNewFolder,
		onUpload,
		onUploadFolder = () => {},
		showFolderUpload = false
	}: {
		category?: FileCategory;
		onNewFolder: () => void;
		onUpload: () => void;
		onUploadFolder?: () => void;
		showFolderUpload?: boolean;
	} = $props();
</script>

<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
	<div class="flex flex-wrap gap-1.5" role="group" aria-label="Filter kategori">
		{#each FILTERS as filter (filter.id)}
			<button
				type="button"
				class={cn(
					'rounded-full px-3 py-1.5 text-xs font-medium transition-colors',
					category === filter.id
						? 'bg-primary text-primary-foreground'
						: 'border border-subtle bg-surface text-muted hover:bg-primary/10 hover:text-main'
				)}
				aria-pressed={category === filter.id}
				onclick={() => {
					category = filter.id;
				}}
			>
				{filter.label}
			</button>
		{/each}
	</div>

	<div class="flex flex-wrap items-center gap-2 self-start">
		<DriveViewToggle />
		<DropdownMenu.Root>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<Button
						{...props}
						type="button"
						variant="default"
						class="gap-1.5"
					>
						<Plus class="size-4 stroke-2" />
						Tambah
						<ChevronDown class="size-3.5 stroke-2" />
					</Button>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="end" class="w-48">
				<DropdownMenu.Item class="gap-2" onclick={onNewFolder}>
					<FolderPlus class="size-4 stroke-2" />
					Folder Baru
				</DropdownMenu.Item>
				<DropdownMenu.Item class="gap-2" onclick={onUpload}>
					<Upload class="size-4 stroke-2" />
					Upload File
				</DropdownMenu.Item>
				{#if showFolderUpload}
					<DropdownMenu.Item class="gap-2" onclick={onUploadFolder}>
						<FolderUp class="size-4 stroke-2" />
						Upload Folder
					</DropdownMenu.Item>
				{/if}
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	</div>
</div>
