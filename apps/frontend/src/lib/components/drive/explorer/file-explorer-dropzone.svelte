<script lang="ts">
	import { Upload } from '@lucide/svelte';
	import { cn } from '$lib/utils.js';
	import type { Snippet } from 'svelte';
	import { locale } from '$lib/locale.svelte.js';

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
	aria-label={locale.language === 'id' ? 'Zona unggah berkas' : 'File upload drop zone'}
	class={cn('relative rounded-xl', active && 'border border-primary')}
	{ondragenter}
	{ondragover}
	{ondragleave}
	{ondrop}
>
	{@render children()}

	{#if active}
		<div
			class="pointer-events-none fixed inset-0 top-16 z-40 flex flex-col items-center justify-center gap-3 bg-primary/10 backdrop-blur-[1px] md:start-64"
			aria-hidden="true"
		>
			<div
				class="m-6 flex h-[calc(100%-3rem)] w-[calc(100%-3rem)] flex-col items-center justify-center gap-4 rounded-2xl border-2 border-dashed border-primary bg-surface/95 p-8 text-primary shadow-none"
			>
				<div
					class="flex size-16 items-center justify-center rounded-2xl border border-dashed border-primary bg-primary/10 text-primary"
				>
					<Upload class="size-8 stroke-2" />
				</div>
				<div class="text-center">
					<p class="text-base font-semibold text-primary">
						{locale.language === 'id'
							? 'Lepas berkas di mana saja untuk mengunggah'
							: 'Drop files anywhere to upload'}
					</p>
					<p class="mt-1 text-xs text-muted">
						{locale.language === 'id'
							? 'Berkas akan diunggah ke lokasi folder saat ini'
							: 'Files will be uploaded to the current folder'}
					</p>
				</div>
			</div>
		</div>
	{/if}
</div>
