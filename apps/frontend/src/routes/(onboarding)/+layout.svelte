<script lang="ts">
	import ThemeToggle from '$lib/components/theme-toggle.svelte';

	let { children } = $props();
</script>

<div class="flex min-h-dvh flex-col bg-app text-main">
	<header
		class="mx-auto flex w-full max-w-5xl items-center justify-between border-b border-subtle px-6 py-4 md:px-10"
	>
		<p class="text-sm font-semibold">CubyCloud</p>
		<ThemeToggle />
	</header>

	<main class="mx-auto w-full max-w-5xl flex-1 px-6 py-8 md:px-10 md:py-10">
		{@render children()}
	</main>
</div>
