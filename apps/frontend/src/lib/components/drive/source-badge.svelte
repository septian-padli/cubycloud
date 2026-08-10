<script lang="ts">
	import type { FileSource } from '$lib/types/drive.js';
	import { cn } from '$lib/utils.js';

	let {
		source,
		label
	}: {
		source: FileSource;
		label: string;
	} = $props();
</script>

<span
	class={cn(
		'max-w-40 truncate rounded-full px-2.5 py-1 text-xs font-medium',
		source === 'organization'
			? 'bg-primary/15 text-primary'
			: source === 'shared'
				? 'bg-secondary text-main'
				: 'border border-subtle bg-app text-muted'
	)}
	title={label}
>
	{label}
</span>
