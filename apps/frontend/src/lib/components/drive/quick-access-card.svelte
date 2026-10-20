<script lang="ts">
	import type { FileKind } from '$lib/types/drive.js';
	import FileGridCard from './file-grid-card.svelte';

	let {
		variant,
		name,
		sizeLabel,
		openedAgo,
		onOpen,
		onSelect
	}: {
		variant: FileKind;
		name: string;
		sizeLabel: string;
		openedAgo: string;
		onOpen?: () => void;
		onSelect?: () => void;
	} = $props();
</script>

<FileGridCard kind={variant} {name} subtitle="{sizeLabel} · {openedAgo}" {onOpen} {onSelect} />
