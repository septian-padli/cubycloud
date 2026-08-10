<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import type { LargestFileItem } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import { driveView } from '$lib/drive-view.svelte.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import LargestFileRow from './largest-file-row.svelte';
	import FilePreviewDialog from './explorer/file-preview-dialog.svelte';
	import CleanupAdvisorDialog from './cleanup-advisor-dialog.svelte';
	import DriveFileGrid from './drive-file-grid.svelte';
	import FileGridCard from './file-grid-card.svelte';
	import DriveViewToggle from './drive-view-toggle.svelte';
	import { Download, Eye, MoreVertical, Sparkles, Trash2 } from '@lucide/svelte';

	let {
		files,
		trashCount = 0
	}: {
		files: LargestFileItem[];
		trashCount?: number;
	} = $props();

	let selectedId = $state<string | null>(null);
	let cleanupOpen = $state(false);

	const previewId = $derived(page.url.searchParams.get('preview'));
	const previewEntry = $derived(
		previewId ? (files.find((e) => e.id === previewId) ?? null) : null
	);
	const previewOpen = $derived(!!previewEntry);

	function syncPreview(id: string | null) {
		const params = new URLSearchParams(page.url.searchParams);
		if (id) params.set('preview', id);
		else params.delete('preview');
		const qs = params.toString();
		void goto(`${page.url.pathname}${qs ? `?${qs}` : ''}`, {
			replaceState: true,
			keepFocus: true,
			noScroll: true
		});
	}

	function onDownload(entry: LargestFileItem) {
		toast.info(`Mengunduh “${entry.name}” (mock)`, {
			description: formatSize(entry.sizeBytes)
		});
	}
</script>

<section class="space-y-3">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<h2 class="text-sm font-semibold text-main">Berkas terbesar</h2>
		<div class="flex flex-wrap items-center gap-2">
			<Button
				type="button"
				variant="secondary"
				size="sm"
				class="gap-1.5"
				onclick={() => (cleanupOpen = true)}
			>
				<Sparkles class="size-4 stroke-2" />
				Saran Pembersihan
			</Button>
			<Button type="button" variant="secondary" size="sm" class="gap-1.5" href="/drive/trash">
				<Trash2 class="size-4 stroke-2" />
				Kelola Sampah
			</Button>
			<DriveViewToggle />
		</div>
	</div>

	{#if driveView.mode === 'grid'}
		<DriveFileGrid label="Berkas terbesar">
			{#each files as entry (entry.id)}
				<FileGridCard
					kind={entry.kind}
					name={entry.name}
					subtitle="{formatSize(entry.sizeBytes)} · {entry.modifiedAt}"
					selected={selectedId === entry.id}
					onSelect={() => (selectedId = entry.id)}
					onOpen={() => syncPreview(entry.id)}
				>
					{#snippet menu()}
						<DropdownMenu.Root>
							<DropdownMenu.Trigger>
								{#snippet child({ props })}
									<Button
										{...props}
										type="button"
										variant="ghost"
										size="icon-sm"
										class="bg-surface text-muted hover:bg-primary hover:text-white aria-expanded:bg-primary aria-expanded:text-white"
										aria-label="Aksi berkas"
										onclick={(e: MouseEvent) => e.stopPropagation()}
									>
										<MoreVertical class="size-4 stroke-2" />
									</Button>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="end" class="min-w-40">
								<DropdownMenu.Item class="gap-2" onclick={() => syncPreview(entry.id)}>
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
				</FileGridCard>
			{/each}
		</DriveFileGrid>
	{:else}
		<Card.Root class="rounded-xl border-subtle bg-surface py-0 shadow-none">
			<Card.Content class="overflow-hidden p-0" role="listbox" aria-label="Berkas terbesar">
				{#each files as entry (entry.id)}
					<LargestFileRow
						{entry}
						selected={selectedId === entry.id}
						onSelect={(e) => (selectedId = e.id)}
						onPreview={(e) => syncPreview(e.id)}
						onDownload={onDownload}
					/>
				{/each}
			</Card.Content>
		</Card.Root>
	{/if}
</section>

<FilePreviewDialog
	open={previewOpen}
	entry={previewEntry
		? {
				...previewEntry,
				parentId: null
			}
		: null}
	onDownload={(e) => onDownload(e as LargestFileItem)}
	onClose={() => syncPreview(null)}
/>
<CleanupAdvisorDialog
	bind:open={cleanupOpen}
	suggestions={files}
	{trashCount}
	onGoTrash={() => {
		cleanupOpen = false;
		void goto('/drive/trash');
	}}
/>
