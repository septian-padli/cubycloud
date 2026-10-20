<script lang="ts">
	import { goto } from '$app/navigation';
	import type { ActivityAction, ActivityItem } from '$lib/types/drive.js';
	import { driveView } from '$lib/drive-view.svelte.js';
	import { recordItemAccess } from '$lib/api/drive-home.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Pencil, Share2, Upload } from '@lucide/svelte';
	import type { Component } from 'svelte';
	import DriveFileGrid from './drive-file-grid.svelte';

	let {
		items
	}: {
		items: ActivityItem[];
	} = $props();

	const icons: Record<ActivityAction, Component> = {
		upload: Upload,
		edit: Pencil,
		share: Share2
	};

	const labels: Record<ActivityAction, string> = {
		upload: 'Mengunggah',
		edit: 'Mengubah',
		share: 'Membagikan'
	};

	function openActivity(item: ActivityItem) {
		if (!item.itemId) return;
		const source = item.source === 'organization' ? 'organization' : 'my-storage';
		void recordItemAccess(item.itemId, source);
		if (item.itemKind === 'folder') {
			void goto(`/drive/${source}?folder=${encodeURIComponent(item.itemId)}`);
		} else {
			void goto(`/drive/${source}?preview=${encodeURIComponent(item.itemId)}`);
		}
	}
</script>

<section class="space-y-3">
	<h2 class="text-sm font-semibold text-main">Aktivitas Terbaru</h2>
	{#if items.length === 0}
		<div
			class="rounded-xl border border-subtle bg-surface px-4 py-8 text-center text-sm text-muted"
			data-testid="recent-activities-empty"
		>
			Belum ada aktivitas terbaru.
		</div>
	{:else if driveView.mode === 'grid'}
		<DriveFileGrid label="Aktivitas terbaru">
			{#each items as item (item.id)}
				{@const Icon = icons[item.action]}
				<Card.Root
					role={item.itemId ? 'button' : undefined}
					tabindex={item.itemId ? 0 : undefined}
					onclick={() => openActivity(item)}
					onkeydown={(e) => {
						if (e.key === 'Enter') openActivity(item);
					}}
					class="rounded-xl border border-subtle bg-surface shadow-none transition-colors hover:bg-surface/80 {item.itemId ? 'cursor-pointer' : ''}"
				>
					<Card.Content class="flex flex-col gap-3">
						<div
							class="flex size-16 items-center justify-center rounded-xl bg-primary/10 text-primary"
						>
							<Icon class="size-10 stroke-2" aria-hidden="true" />
						</div>
						<div class="min-w-0">
							<p class="truncate text-sm font-medium text-main">{labels[item.action]}</p>
							<p class="mt-0.5 truncate text-xs text-muted">{item.fileName} · {item.at}</p>
						</div>
					</Card.Content>
				</Card.Root>
			{/each}
		</DriveFileGrid>
	{:else}
		<ul class="overflow-hidden rounded-xl border border-subtle bg-surface">
			{#each items as item, i (item.id)}
				{@const Icon = icons[item.action]}
				<li class={i > 0 ? 'border-t border-subtle' : ''}>
					{#if item.itemId}
						<button
							type="button"
							onclick={() => openActivity(item)}
							class="flex w-full cursor-pointer items-center gap-3 px-4 py-3 text-start transition-colors hover:bg-primary/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
						>
							<div
								class="flex size-8 shrink-0 items-center justify-center rounded-lg bg-app text-primary"
							>
								<Icon class="size-4 stroke-2" aria-hidden="true" />
							</div>
							<div class="min-w-0 flex-1">
								<p class="truncate text-sm text-main">
									<span class="font-medium">{labels[item.action]}</span>
									{' '}
									<span class="text-muted">{item.fileName}</span>
								</p>
								<p class="text-xs text-muted">{item.at}</p>
							</div>
						</button>
					{:else}
						<div class="flex items-center gap-3 px-4 py-3">
							<div
								class="flex size-8 shrink-0 items-center justify-center rounded-lg bg-app text-primary"
							>
								<Icon class="size-4 stroke-2" aria-hidden="true" />
							</div>
							<div class="min-w-0 flex-1">
								<p class="truncate text-sm text-main">
									<span class="font-medium">{labels[item.action]}</span>
									{' '}
									<span class="text-muted">{item.fileName}</span>
								</p>
								<p class="text-xs text-muted">{item.at}</p>
							</div>
						</div>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</section>
