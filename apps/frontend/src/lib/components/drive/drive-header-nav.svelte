<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
	import { Menu, Search, Upload } from '@lucide/svelte';

	let {
		onMenuClick
	}: {
		onMenuClick: () => void;
	} = $props();

	let search = $state('');
</script>

<header
	class="z-30 flex shrink-0 items-center gap-3 border-b border-subtle bg-surface px-4 py-4 md:px-6"
>
	<Button
		type="button"
		variant="ghost"
		size="icon-sm"
		class="md:hidden"
		aria-label="Buka menu navigasi"
		onclick={onMenuClick}
	>
		<Menu class="size-4 stroke-2" />
	</Button>

	<div class="relative min-w-0 w-full max-w-sm">
		<Search
			class="pointer-events-none absolute start-3 top-1/2 size-4 -translate-y-1/2 stroke-2 text-muted"
			aria-hidden="true"
		/>
		<Input
			class="ps-9"
			placeholder="Cari berkas…"
			bind:value={search}
			aria-label="Cari berkas"
		/>
	</div>

	<div class="ms-auto flex shrink-0 items-center gap-3">
		<!-- ponytail: Quick Upload drawer — iterasi berikutnya -->
		<Button type="button" class="gap-1.5">
			<Upload class="size-4 stroke-2" />
			<span class="hidden sm:inline">Unggah</span>
		</Button>

		<ThemeToggle />
	</div>
</header>
