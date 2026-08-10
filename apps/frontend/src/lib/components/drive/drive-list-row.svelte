<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cn } from '$lib/utils.js';

	let {
		selected = false,
		onSelect,
		onOpen,
		leading,
		meta,
		trailing
	}: {
		selected?: boolean;
		onSelect?: () => void;
		onOpen?: () => void;
		leading: Snippet;
		meta: Snippet;
		trailing?: Snippet;
	} = $props();
</script>

<div
	role="option"
	tabindex="0"
	aria-selected={selected}
	class={cn(
		'flex cursor-pointer items-center justify-between border-b border-subtle px-4 py-3 transition-colors hover:bg-black/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 dark:hover:bg-white/5',
		selected && 'bg-primary/5'
	)}
	onclick={() => onSelect?.()}
	ondblclick={() => onOpen?.()}
	onkeydown={(e) => {
		if (e.key === 'Enter') onOpen?.();
	}}
>
	<div class="flex min-w-0 items-center gap-3">
		{@render leading()}
		<div class="min-w-0">
			{@render meta()}
		</div>
	</div>

	{#if trailing}
		<div class="flex shrink-0 items-center gap-2">
			{@render trailing()}
		</div>
	{/if}
</div>
