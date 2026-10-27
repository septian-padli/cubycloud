<script lang="ts">
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
	import LanguagePicker from '$lib/components/language-picker.svelte';
	let { children } = $props();
</script>

<div class="flex min-h-dvh flex-col bg-app text-main">
	<header
		class="mx-auto flex w-full max-w-5xl items-center justify-between border-b border-subtle px-6 py-4 md:px-10"
	>
		<p class="text-sm font-semibold">CubyCloud</p>
		<div class="flex items-center gap-3">
			<ThemeToggle />
			<LanguagePicker />
		</div>
	</header>
	<footer class="mx-auto w-full max-w-5xl px-6 pb-4 text-xs text-muted md:px-10">
		Copyright (C) 2026 CubyCloud contributors. You may convey this work under AGPL-3.0-or-later. No warranty.
		<a class="underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary" href="https://github.com/septian-padli/cubycloud/blob/main/LICENSE" target="_blank" rel="noreferrer">License</a>
		·
		<a class="underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary" href="https://github.com/septian-padli/cubycloud" target="_blank" rel="noreferrer">Source</a>
	</footer>

	<main class="mx-auto w-full max-w-5xl flex-1 px-6 py-8 md:px-10 md:py-10">
		{@render children()}
	</main>
</div>
