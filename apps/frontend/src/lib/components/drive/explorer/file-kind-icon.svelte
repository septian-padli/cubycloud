<script lang="ts">
	import type { FileKind } from '$lib/types/drive.js';
	import { Archive, File, FileAudio, FileImage, FileText, FileVideo, Folder } from '@lucide/svelte';
	import type { Component } from 'svelte';
	import { cn } from '$lib/utils.js';

	let {
		kind,
		class: className
	}: {
		kind: FileKind;
		class?: string;
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
	class={cn(
		'flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary',
		className
	)}
>
	<Icon class="size-5 stroke-2" aria-hidden="true" />
</div>
