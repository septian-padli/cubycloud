<script lang="ts">
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
	import { getDriveChrome } from '$lib/drive-context.svelte.js';
	import { Menu, Search, Upload } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	let {
		onMenuClick
	}: {
		onMenuClick: () => void;
	} = $props();

	const chrome = getDriveChrome();
	let search = $state('');

	const onExplorerPage = $derived(
		page.url.pathname === '/drive/my-storage' || page.url.pathname === '/drive/organization'
	);

	function onUploadClick() {
		if (onExplorerPage) {
			chrome.requestUpload();
			return;
		}
		toast.info('Buka My Storage atau Organization untuk mengunggah berkas.');
	}

	function toggleRole() {
		const next = chrome.user.role === 'super_admin' ? 'regular' : 'super_admin';
		chrome.setRole(next);
		toast.message(
			next === 'super_admin' ? 'Mode demo: Super Admin' : 'Mode demo: Regular User',
			{ description: 'Badge storage node mengikuti role.' }
		);
	}
</script>

<header
	class="z-30 flex shrink-0 items-center gap-3 border-b border-subtle bg-surface px-4 py-4 md:px-6"
>
	<Button
		type="button"
		variant="ghost"
		size="icon"
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

	<div class="ms-auto flex shrink-0 items-center gap-2 sm:gap-3">
		<!-- ponytail: role toggle demo — hapus saat auth nyata -->
		<Button
			type="button"
			variant="outline"
			class="hidden text-xs sm:inline-flex"
			onclick={toggleRole}
			aria-label="Ganti role demo"
		>
			{chrome.user.role === 'super_admin' ? 'Super Admin' : 'Regular'}
		</Button>

		<Button type="button" class="gap-1.5" onclick={onUploadClick}>
			<Upload class="size-4 stroke-2" />
			<span class="hidden sm:inline">Unggah</span>
		</Button>

		<ThemeToggle />
	</div>
</header>
