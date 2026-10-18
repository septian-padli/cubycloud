<script lang="ts">
	import { Tabs as TabsPrimitive } from "bits-ui";
	import { cn, type WithoutChild } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		...restProps
	}: WithoutChild<TabsPrimitive.ListProps> = $props();
</script>

<TabsPrimitive.List
	bind:ref
	data-slot="tabs-list"
	class={cn(
		"inline-flex h-9 w-fit items-center justify-center rounded-lg bg-secondary p-[3px] text-muted",
		className
	)}
	{...restProps}
/>
