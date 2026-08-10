<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		label = 'Daftar berkas',
		children
	}: {
		label?: string;
		children: Snippet;
	} = $props();
</script>

<div
	class="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4"
	role="listbox"
	aria-label={label}
>
	{@render children()}
</div>
