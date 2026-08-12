<script lang="ts">
	import type { LargestFileItem } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Download, Eye, MoreVertical } from '@lucide/svelte';
	import DriveListRow from './drive-list-row.svelte';
	import FileKindIcon from './explorer/file-kind-icon.svelte';

	let {
		entry,
		selected = false,
		onSelect,
		onPreview,
		onDownload
	}: {
		entry: LargestFileItem;
		selected?: boolean;
		onSelect: (entry: LargestFileItem) => void;
		onPreview: (entry: LargestFileItem) => void;
		onDownload: (entry: LargestFileItem) => void;
	} = $props();
</script>

<DriveListRow
	{selected}
	onSelect={() => onSelect(entry)}
	onOpen={() => onPreview(entry)}
>
	{#snippet leading()}
		<FileKindIcon kind={entry.kind} />
	{/snippet}
	{#snippet meta()}
		<p class="truncate text-sm font-medium text-main">{entry.name}</p>
		<p class="text-xs text-muted">
			{formatSize(entry.sizeBytes)} · Diubah {entry.modifiedAt}
		</p>
	{/snippet}
	{#snippet trailing()}
		<span class="text-xs font-medium text-muted">{formatSize(entry.sizeBytes)}</span>
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
			<DropdownMenu.Content align="end" class="min-w-40">
				<DropdownMenu.Item class="gap-2" onclick={() => onPreview(entry)}>
					<Eye class="size-4 stroke-2" />
					Pratinjau
				</DropdownMenu.Item>
				<DropdownMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
					<Download class="size-4 stroke-2" />
					Unduh
				</DropdownMenu.Item>
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	{/snippet}
</DriveListRow>
