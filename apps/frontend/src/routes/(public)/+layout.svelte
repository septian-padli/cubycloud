<script lang="ts">
	let { children } = $props();
</script>

<div class="flex min-h-dvh flex-col bg-app text-main">
	<main class="flex-1">
		{@render children()}
	</main>
	<footer class="mx-auto w-full max-w-5xl px-6 py-4 text-xs text-muted md:px-10">
		Copyright (C) 2026 CubyCloud contributors. You may convey this work under AGPL-3.0-or-later. No
		warranty.
		<a
			class="underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			href="https://github.com/septian-padli/cubycloud/blob/main/LICENSE"
			target="_blank"
			rel="noreferrer">License</a
		>
		·
		<a
			class="underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			href="https://github.com/septian-padli/cubycloud"
			target="_blank"
			rel="noreferrer">Source</a
		>
	</footer>
</div>
