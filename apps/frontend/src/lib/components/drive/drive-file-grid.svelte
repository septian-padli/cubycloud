<script lang="ts">
	import { locale } from '$lib/locale.svelte.js';
	import type { Snippet } from 'svelte';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let {
		label = tx('Daftar berkas', 'File list'),
		children
	}: {
		label?: string;
		children: Snippet;
	} = $props();
</script>

<div class="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4" role="listbox" aria-label={label}>
	{@render children()}
</div>
