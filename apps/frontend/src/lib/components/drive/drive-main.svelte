<script lang="ts">
	import DriveHeaderNav from './drive-header-nav.svelte';
	import type { Snippet } from 'svelte';

	let {
		content,
		onMenuClick
	}: {
		content: Snippet;
		onMenuClick: () => void;
	} = $props();
</script>

<div class="flex h-full flex-col overflow-hidden">
	<DriveHeaderNav {onMenuClick} />

	<div class="min-h-0 flex-1 overflow-y-auto">
		<div class="mx-auto flex w-full max-w-5xl flex-col gap-8 px-4 py-6 md:px-6 md:py-8">
			{@render content()}
		</div>
	</div>
</div>
