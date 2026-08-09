<script lang="ts">
	import { Eye, EyeOff } from '@lucide/svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { cn } from '$lib/utils.js';
	import type { HTMLInputAttributes } from 'svelte/elements';

	let {
		value = $bindable(''),
		id,
		class: className,
		disabled = false,
		autocomplete,
		placeholder,
		name,
		'aria-invalid': ariaInvalid,
		oninput
	}: {
		value?: string;
		id?: string;
		class?: string;
		disabled?: boolean;
		autocomplete?: HTMLInputAttributes['autocomplete'];
		placeholder?: string;
		name?: string;
		'aria-invalid'?: HTMLInputAttributes['aria-invalid'];
		oninput?: HTMLInputAttributes['oninput'];
	} = $props();

	let visible = $state(false);
</script>

<div class="relative">
	<Input
		{id}
		{name}
		type={visible ? 'text' : 'password'}
		class={cn('pe-10', className)}
		bind:value
		{disabled}
		{autocomplete}
		{placeholder}
		aria-invalid={ariaInvalid}
		{oninput}
	/>
	<Button
		type="button"
		variant="ghost"
		size="icon-sm"
		class="absolute end-1 top-1/2 -translate-y-1/2 text-muted"
		{disabled}
		aria-label={visible ? 'Sembunyikan password' : 'Tampilkan password'}
		aria-pressed={visible}
		onclick={() => {
			visible = !visible;
		}}
	>
		{#if visible}
			<EyeOff class="size-4 stroke-2" />
		{:else}
			<Eye class="size-4 stroke-2" />
		{/if}
	</Button>
</div>
