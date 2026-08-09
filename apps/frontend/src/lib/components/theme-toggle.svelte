<script lang="ts">
	import { Moon, Sun } from '@lucide/svelte';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { theme } from '$lib/theme.svelte.js';

	const checked = $derived(theme.isDark);
</script>

<div class="flex items-center gap-2">
	<Sun class="size-4 stroke-2 text-muted" aria-hidden="true" />
	<Switch
		{checked}
		aria-label="Mode terang/gelap"
		onCheckedChange={(value) => theme.setMode(value ? 'dark' : 'light')}
	/>
	<Moon class="size-4 stroke-2 text-muted" aria-hidden="true" />
</div>
