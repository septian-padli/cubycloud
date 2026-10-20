<script lang="ts">
	import { goto } from '$app/navigation';
	import type { QuickAccessItem } from '$lib/types/drive.js';
	import { driveView } from '$lib/drive-view.svelte.js';
	import { recordItemAccess } from '$lib/api/drive-home.js';
	import QuickAccessCard from './quick-access-card.svelte';
	import FileKindIcon from './explorer/file-kind-icon.svelte';
	import DriveListRow from './drive-list-row.svelte';
	import DriveFileGrid from './drive-file-grid.svelte';

	let {
		items
	}: {
		items: QuickAccessItem[];
	} = $props();

	function openItem(item: QuickAccessItem) {
		const source = item.source === 'organization' ? 'organization' : 'my-storage';
		void recordItemAccess(item.id, source);
		if (item.kind === 'folder') {
			void goto(`/drive/${source}?folder=${encodeURIComponent(item.id)}`);
		} else {
			void goto(`/drive/${source}?preview=${encodeURIComponent(item.id)}`);
		}
	}
</script>

<section class="space-y-3">
	<h2 class="text-sm font-semibold text-main">Akses Cepat</h2>
	{#if items.length === 0}
		<div
			class="rounded-xl border border-subtle bg-surface px-4 py-8 text-center text-sm text-muted"
			data-testid="quick-access-empty"
		>
			Belum ada berkas atau folder yang dibuka.
		</div>
	{:else if driveView.mode === 'list'}
		<div
			class="overflow-hidden rounded-xl border border-subtle bg-surface"
			role="listbox"
			aria-label="Akses cepat"
		>
			{#each items as item (item.id)}
				<DriveListRow
					onOpen={() => openItem(item)}
					onSelect={() => openItem(item)}
				>
					{#snippet leading()}
						<FileKindIcon kind={item.kind} />
					{/snippet}
					{#snippet meta()}
						<p class="truncate text-sm font-medium text-main">{item.name}</p>
						<p class="text-xs text-muted">{item.sizeLabel} · {item.openedAgo}</p>
					{/snippet}
				</DriveListRow>
			{/each}
		</div>
	{:else}
		<DriveFileGrid label="Akses cepat">
			{#each items as item (item.id)}
				<QuickAccessCard
					variant={item.kind}
					name={item.name}
					sizeLabel={item.sizeLabel}
					openedAgo={item.openedAgo}
					onOpen={() => openItem(item)}
					onSelect={() => openItem(item)}
				/>
			{/each}
		</DriveFileGrid>
	{/if}
</section>
