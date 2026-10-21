<script lang="ts">
	import { Tabs as TabsPrimitive } from 'bits-ui';
	import { cn, type WithoutChild } from '$lib/utils.js';

	type TabsListProps = WithoutChild<TabsPrimitive.ListProps> & {
		variant: 'line';
	};

	let { ref = $bindable(null), class: className, variant, ...restProps }: TabsListProps = $props();
</script>

<TabsPrimitive.List
	bind:ref
	data-slot="tabs-list"
	data-variant={variant}
	class={cn(
		'inline-flex h-10 w-fit items-center justify-center border-b border-subtle bg-transparent p-0 text-muted',
		className
	)}
	{...restProps}
/>
