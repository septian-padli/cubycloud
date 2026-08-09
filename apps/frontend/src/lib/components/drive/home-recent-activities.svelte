<script lang="ts">
	import type { ActivityAction, ActivityItem } from '$lib/types/drive.js';
	import { Pencil, Share2, Upload } from '@lucide/svelte';
	import type { Component } from 'svelte';

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
		edit: 'Mengedit',
		share: 'Membagikan'
	};
</script>

<section class="space-y-3">
	<h2 class="text-sm font-semibold text-main">Aktivitas Terbaru</h2>
	<ul class="overflow-hidden rounded-xl border border-subtle bg-surface">
		{#each items as item, i (item.id)}
			{@const Icon = icons[item.action]}
			<li
				class="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-primary/5"
				class:border-t={i > 0}
				class:border-subtle={i > 0}
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
			</li>
		{/each}
	</ul>
</section>
