<script lang="ts">
	import type { BreadcrumbSegment } from '$lib/types/drive.js';
	import { ChevronRight } from '@lucide/svelte';
	import { locale } from '$lib/locale.svelte.js';

	let {
		segments,
		onNavigate
	}: {
		segments: BreadcrumbSegment[];
		onNavigate: (folderId: string | null) => void;
	} = $props();
</script>

<nav
	class="flex flex-wrap items-center gap-1 text-sm"
	aria-label={locale.language === 'id' ? 'Jejak navigasi' : 'Breadcrumb'}
>
	<ol class="flex flex-wrap items-center gap-1">
		{#each segments as segment, i (segment.id ?? 'root')}
			{@const isLast = i === segments.length - 1}
			<li class="flex items-center gap-1">
				{#if i > 0}
					<ChevronRight class="size-3.5 shrink-0 stroke-2 text-muted" aria-hidden="true" />
				{/if}
				{#if isLast}
					<span class="font-medium text-main" aria-current="page">{segment.label}</span>
				{:else}
					<button
						type="button"
						class="rounded-full px-1.5 py-0.5 text-muted transition-colors hover:bg-primary/10 hover:text-main"
						onclick={() => onNavigate(segment.id)}
					>
						{segment.label}
					</button>
				{/if}
			</li>
		{/each}
	</ol>
</nav>
