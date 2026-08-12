<script lang="ts">
	import type { FileCategory } from '$lib/types/drive.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import DriveViewToggle from '$lib/components/drive/drive-view-toggle.svelte';
	import { FolderPlus } from '@lucide/svelte';
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
		onNewFolder
	}: {
		category?: FileCategory;
		onNewFolder: () => void;
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
		<Button type="button" variant="secondary" class="gap-1.5" onclick={onNewFolder}>
			<FolderPlus class="size-4 stroke-2" />
			Folder Baru
		</Button>
	</div>
</div>
