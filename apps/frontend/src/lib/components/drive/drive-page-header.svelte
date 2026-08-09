<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		title,
		description,
		eyebrow,
		actions
	}: {
		title: string;
		description?: string;
		eyebrow?: string;
		actions?: Snippet;
	} = $props();
</script>

<header class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
	<div class="min-w-0 space-y-1">
		{#if eyebrow}
			<p class="text-xs font-medium text-muted">{eyebrow}</p>
		{/if}
		<h1 class="text-2xl font-bold leading-tight text-main">{title}</h1>
		{#if description}
			<p class="text-sm text-muted">{description}</p>
		{/if}
	</div>

	{#if actions}
		<div class="flex shrink-0 flex-wrap items-center gap-2">
			{@render actions()}
		</div>
	{/if}
</header>
