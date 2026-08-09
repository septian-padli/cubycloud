<script lang="ts">
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
	import authHero from '$lib/assets/auth-hero-placeholder.svg';
	import type { Snippet } from 'svelte';

	let {
		children
	}: {
		children: Snippet;
	} = $props();

	// ponytail: placeholder 9:16 — swap import when final auth hero asset is ready
</script>

<div class="relative min-h-dvh bg-app text-main md:grid md:grid-cols-2">
	<!-- Mobile background -->
	<div class="pointer-events-none absolute inset-0 md:hidden" aria-hidden="true">
		<img src={authHero} alt="" class="h-full w-full object-cover object-center" />
		<div class="absolute inset-0 bg-app/80"></div>
	</div>

	<!-- Desktop image panel -->
	<aside class="relative hidden overflow-hidden md:block">
		<img
			src={authHero}
			alt=""
			class="absolute inset-0 h-full w-full object-cover object-center"
		/>
	</aside>

	<!-- Form panel -->
	<section class="relative flex min-h-dvh flex-col">
		<header class="flex items-center justify-between px-6 py-4 md:px-10">
			<a href="/" class="text-sm font-semibold text-main">CubyCloud</a>
			<ThemeToggle />
		</header>

		<div class="flex flex-1 items-center justify-center px-6 py-8 md:px-10 md:py-10">
			<div class="w-full max-w-md">
				{@render children()}
			</div>
		</div>
	</section>
</div>
