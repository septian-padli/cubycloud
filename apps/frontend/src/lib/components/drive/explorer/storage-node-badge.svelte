<script lang="ts">
	import type { StorageNodeKind } from '$lib/types/drive.js';
	import { cn } from '$lib/utils.js';

	let {
		node
	}: {
		node: StorageNodeKind;
	} = $props();

	const labels: Record<StorageNodeKind, string> = {
		r2: 'R2',
		s3: 'S3',
		ssd: 'SSD-1',
		hdd: 'HDD-2'
	};

	const cloud = $derived(node === 'r2' || node === 's3');
</script>

<span
	class={cn(
		'rounded-full px-2.5 py-1 text-xs font-medium',
		cloud
			? 'bg-secondary-accent text-white dark:text-app'
			: 'bg-secondary text-main dark:text-app'
	)}
>
	{labels[node]}
</span>
