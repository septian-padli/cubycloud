<script lang="ts">
	import { Select as SelectPrimitive } from "bits-ui";
	import { Check } from "@lucide/svelte";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		value,
		label,
		children,
		...restProps
	}: SelectPrimitive.ItemProps = $props();
</script>

<SelectPrimitive.Item
	bind:ref
	data-slot="select-item"
	{value}
	{label}
	class={cn(
		"relative flex w-full cursor-pointer items-center gap-2 rounded-lg py-1.5 ps-8 pe-2 text-sm text-main outline-none select-none data-disabled:pointer-events-none data-disabled:opacity-50 hover:bg-muted/15 focus:bg-muted/15 data-highlighted:bg-muted/15",
		className
	)}
	{...restProps}
>
	{#snippet children(itemProps)}
		<span class="absolute start-2 flex size-3.5 items-center justify-center">
			{#if itemProps?.selected}
				<Check class="size-4 stroke-2" />
			{/if}
		</span>
		{#if children}
			{@render children(itemProps)}
		{:else}
			{label ?? value}
		{/if}
	{/snippet}
</SelectPrimitive.Item>
