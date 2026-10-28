<script lang="ts">
	import { Checkbox as CheckboxPrimitive } from 'bits-ui';
	import CheckIcon from '@lucide/svelte/icons/check';
	import MinusIcon from '@lucide/svelte/icons/minus';
	import { cn, type WithoutChildrenOrChild } from '$lib/utils.js';

	let {
		ref = $bindable(null),
		checked = $bindable(false),
		indeterminate = $bindable(false),
		class: className,
		...restProps
	}: WithoutChildrenOrChild<CheckboxPrimitive.RootProps> = $props();

	$effect(() => {
		if (ref) {
			Object.defineProperty(ref, 'indeterminate', {
				get: () => indeterminate === true,
				configurable: true
			});
			Object.defineProperty(ref, 'checked', {
				get: () => checked === true,
				configurable: true
			});
		}
	});
</script>

<CheckboxPrimitive.Root
	bind:ref
	data-slot="checkbox"
	class={cn(
		'peer relative flex size-4 shrink-0 cursor-pointer items-center justify-center rounded-[4px] border border-primary/40 bg-surface shadow-none transition-all outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 aria-invalid:aria-checked:border-primary data-[state=checked]:border-primary data-[state=checked]:bg-primary data-[state=checked]:text-primary-foreground data-[state=indeterminate]:border-primary data-[state=indeterminate]:bg-primary data-[state=indeterminate]:text-primary-foreground dark:border-primary/40 dark:bg-surface/50 dark:data-[state=checked]:border-primary dark:data-[state=checked]:bg-primary dark:data-[state=checked]:text-primary-foreground dark:data-[state=indeterminate]:border-primary dark:data-[state=indeterminate]:bg-primary dark:data-[state=indeterminate]:text-primary-foreground',
		className
	)}
	bind:checked
	bind:indeterminate
	{...restProps}
>
	{#snippet children({ checked, indeterminate })}
		<div
			data-slot="checkbox-indicator"
			class="grid place-content-center text-current transition-none [&>svg]:size-3.5"
		>
			{#if checked}
				<CheckIcon class="size-3.5 stroke-[3]" />
			{:else if indeterminate}
				<MinusIcon class="size-3.5 stroke-[3]" />
			{/if}
		</div>
	{/snippet}
</CheckboxPrimitive.Root>
