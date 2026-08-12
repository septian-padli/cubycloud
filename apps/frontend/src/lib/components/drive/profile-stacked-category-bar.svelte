<script lang="ts">
	import type { CategoryUsage } from '$lib/types/drive.js';
	import * as Popover from '$lib/components/ui/popover/index.js';
	import { cn } from '$lib/utils.js';

	let {
		categories
	}: {
		categories: CategoryUsage[];
	} = $props();

	const total = $derived(categories.reduce((sum, c) => sum + c.usedGb, 0));

	let openId = $state<string | null>(null);
	let closeTimer: ReturnType<typeof setTimeout> | null = null;

	const SEGMENT_CLASS: Record<CategoryUsage['id'], string> = {
		video: 'bg-primary',
		image: 'bg-secondary-accent',
		document: 'bg-primary/75',
		archive: 'bg-secondary-accent/75',
		trash: 'bg-primary/50'
	};

	function openSegment(id: string) {
		if (closeTimer) {
			clearTimeout(closeTimer);
			closeTimer = null;
		}
		openId = id;
	}

	function scheduleClose(id: string) {
		if (closeTimer) clearTimeout(closeTimer);
		closeTimer = setTimeout(() => {
			if (openId === id) openId = null;
		}, 120);
	}

	function toggleSegment(id: string) {
		if (closeTimer) {
			clearTimeout(closeTimer);
			closeTimer = null;
		}
		openId = openId === id ? null : id;
	}
</script>

<div class="space-y-2">
	<div
		class="flex h-2.5 w-full overflow-hidden rounded-full bg-black/10 dark:bg-white/10"
		role="img"
		aria-label="Pemakaian My Storage per kategori"
	>
		{#each categories as cat (cat.id)}
			{@const pct = total > 0 ? (cat.usedGb / total) * 100 : 0}
			{#if pct > 0}
				<Popover.Root
					open={openId === cat.id}
					onOpenChange={(next) => {
						if (next) openSegment(cat.id);
						else if (openId === cat.id) openId = null;
					}}
				>
					<Popover.Trigger>
						{#snippet child({ props })}
							<button
								{...props}
								type="button"
								class={cn(
									'h-full min-w-0 shrink-0 transition-opacity hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary',
									SEGMENT_CLASS[cat.id]
								)}
								style="width: {pct}%"
								aria-label="{cat.label}: {cat.usedGb.toFixed(1)} GB"
								onmouseenter={() => openSegment(cat.id)}
								onmouseleave={() => scheduleClose(cat.id)}
								onclick={() => toggleSegment(cat.id)}
							></button>
						{/snippet}
					</Popover.Trigger>
					<Popover.Content
						class="w-auto border border-subtle bg-surface p-3 shadow-none"
						side="top"
						onmouseenter={() => openSegment(cat.id)}
						onmouseleave={() => scheduleClose(cat.id)}
					>
						<p class="text-sm font-medium text-main">{cat.label}</p>
						<p class="text-xs text-muted">
							{cat.usedGb.toFixed(1)} GB · {Math.round(pct)}% dari total My Storage
						</p>
					</Popover.Content>
				</Popover.Root>
			{/if}
		{/each}
	</div>
	<p class="text-xs text-muted">
		Arahkan atau klik segmen untuk detail kategori. Total: {total.toFixed(1)} GB (Organization
		dikecualikan).
	</p>
</div>
