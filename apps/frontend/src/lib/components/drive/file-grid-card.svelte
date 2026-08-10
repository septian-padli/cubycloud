<script lang="ts">
	import type { FileKind } from '$lib/types/drive.js';
	import type { Snippet } from 'svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import {
		Archive,
		File,
		FileAudio,
		FileImage,
		FileText,
		FileVideo,
		Folder
	} from '@lucide/svelte';
	import type { Component } from 'svelte';
	import { cn } from '$lib/utils.js';

	let {
		kind,
		name,
		subtitle,
		selected = false,
		onSelect,
		onOpen,
		badges,
		menu
	}: {
		kind: FileKind;
		name: string;
		subtitle: string;
		selected?: boolean;
		onSelect?: () => void;
		onOpen?: () => void;
		badges?: Snippet;
		menu?: Snippet;
	} = $props();

	const icons: Record<FileKind, Component> = {
		folder: Folder,
		text: FileText,
		image: FileImage,
		video: FileVideo,
		audio: FileAudio,
		document: File,
		archive: Archive
	};

	const Icon = $derived(icons[kind]);
</script>

<div
	role="option"
	tabindex="0"
	aria-selected={selected}
	class="group relative cursor-pointer outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
	onclick={() => onSelect?.()}
	ondblclick={() => onOpen?.()}
	onkeydown={(e) => {
		if (e.key === 'Enter') onOpen?.();
	}}
>
	<Card.Root
		class={cn(
			'overflow-hidden rounded-xl border border-subtle bg-surface shadow-none transition-colors hover:bg-surface/80',
			selected && 'bg-surface/80'
		)}
	>
		<Card.Content class="flex flex-col gap-3">
				{#if menu}
					<div
						role="presentation"
						class="absolute end-2 top-2 z-10"
						onclick={(e) => e.stopPropagation()}
						onkeydown={(e) => e.stopPropagation()}
					>
						{@render menu()}
					</div>
				{/if}
			<div
				class="relative flex aspect-video w-full items-center justify-center rounded-lg bg-primary/10 text-primary"
			>
				<Icon class="size-12 stroke-2" aria-hidden="true" />
			</div>

			<div class="min-w-0">
				<p class="truncate text-sm font-medium text-main">{name}</p>
				<p class="mt-0.5 truncate text-xs text-muted">{subtitle}</p>
				{#if badges}
					<div class="mt-2 flex flex-wrap gap-1.5">
						{@render badges()}
					</div>
				{/if}
			</div>
		</Card.Content>
	</Card.Root>
</div>
