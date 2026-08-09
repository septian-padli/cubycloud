<script lang="ts">
	import type { FileKind } from '$lib/types/drive.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { File, FileAudio, FileImage, FileText, FileVideo, Folder } from '@lucide/svelte';
	import type { Component } from 'svelte';

	let {
		variant,
		name,
		sizeLabel,
		openedAgo
	}: {
		variant: FileKind;
		name: string;
		sizeLabel: string;
		openedAgo: string;
	} = $props();

	const icons: Record<FileKind, Component> = {
		folder: Folder,
		text: FileText,
		image: FileImage,
		video: FileVideo,
		audio: FileAudio,
		document: File
	};

	const Icon = $derived(icons[variant]);
</script>

<Card.Root
	class="rounded-xl border border-subtle bg-surface shadow-none transition-colors hover:bg-surface/10"
>
	<Card.Content class="flex flex-col gap-3 p-4">
		<div
			class="flex size-14 items-center justify-center rounded-xl bg-primary/10 text-primary"
		>
			<Icon class="size-8 stroke-2" aria-hidden="true" />
		</div>
		<div class="min-w-0">
			<p class="truncate text-sm font-medium text-main">{name}</p>
			<p class="mt-0.5 text-xs text-muted">
				{sizeLabel} · {openedAgo}
			</p>
		</div>
	</Card.Content>
</Card.Root>
