<script lang="ts">
	import type { SharePermission } from '$lib/types/drive.js';
	import { cn } from '$lib/utils.js';
	import { locale } from '$lib/locale.svelte.js';

	let {
		permission
	}: {
		permission: SharePermission;
	} = $props();

	const label = $derived(
		permission === 'editor' ? 'Editor' : locale.language === 'id' ? 'Pemirsa' : 'Viewer'
	);
</script>

<span
	class={cn(
		'rounded-full px-2.5 py-1 text-xs font-medium',
		permission === 'editor' ? 'bg-primary/15 text-primary' : 'bg-secondary text-main'
	)}
>
	{label}
</span>
