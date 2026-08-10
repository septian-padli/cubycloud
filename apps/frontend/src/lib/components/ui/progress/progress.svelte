<script lang="ts">
	import { Progress as ProgressPrimitive } from 'bits-ui';
	import { cn, type WithoutChildrenOrChild } from '$lib/utils.js';

	let {
		ref = $bindable(null),
		class: className,
		indicatorClass,
		max = 100,
		value,
		...restProps
	}: WithoutChildrenOrChild<ProgressPrimitive.RootProps> & {
		/** Override fill color (e.g. `bg-primary` / `bg-secondary-accent`). */
		indicatorClass?: string;
	} = $props();

	const pct = $derived(
		max && max > 0 ? Math.min(100, Math.max(0, (100 * (value ?? 0)) / max)) : 0
	);
</script>

<ProgressPrimitive.Root
	bind:ref
	data-slot="progress"
	class={cn(
		'relative flex h-2 w-full items-center overflow-x-hidden rounded-full bg-black/10 shadow-none dark:bg-white/10',
		className
	)}
	{value}
	{max}
	{...restProps}
>
	<!-- Empty track acts as placeholder; indicator slides in as value grows -->
	<div
		data-slot="progress-indicator"
		class={cn('size-full flex-1 bg-primary transition-all', indicatorClass)}
		style="transform: translateX(-{100 - pct}%)"
	></div>
</ProgressPrimitive.Root>
