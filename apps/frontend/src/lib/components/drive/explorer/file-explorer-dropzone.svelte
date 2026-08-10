<script lang="ts">
	import { Upload } from '@lucide/svelte';
	import { cn } from '$lib/utils.js';
	import type { Snippet } from 'svelte';

	let {
		active = false,
		children,
		ondragenter,
		ondragover,
		ondragleave,
		ondrop
	}: {
		active?: boolean;
		children: Snippet;
		ondragenter?: (e: DragEvent) => void;
		ondragover?: (e: DragEvent) => void;
		ondragleave?: (e: DragEvent) => void;
		ondrop?: (e: DragEvent) => void;
	} = $props();
</script>

<div
	role="region"
	aria-label="Zona unggah berkas"
	class={cn('relative rounded-xl', active && 'border border-primary')}
	{ondragenter}
	{ondragover}
	{ondragleave}
	{ondrop}
>
	{@render children()}

	{#if active}
		<div
			class="pointer-events-none absolute inset-0 z-10 flex flex-col items-center justify-center gap-2 rounded-xl bg-primary/10"
			aria-hidden="true"
		>
			<div
				class="flex size-12 items-center justify-center rounded-xl border border-dashed border-primary bg-surface text-primary"
			>
				<Upload class="size-6 stroke-2" />
			</div>
			<p class="text-sm font-medium text-primary">Lepas berkas untuk mengunggah</p>
		</div>
	{/if}
</div>
