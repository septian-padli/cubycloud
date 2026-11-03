<script lang="ts">
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
	import LanguagePicker from '$lib/components/language-picker.svelte';
	import BrandLogo from '$lib/components/brand-logo.svelte';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();
</script>

<div class="flex min-h-dvh flex-col bg-app text-main">
	<header
		class="flex w-full items-center justify-between border-b border-subtle px-6 py-4 md:px-10"
	>
		<a
			href="/"
			class="inline-flex items-center focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			aria-label="CubyCrate"
		>
			<BrandLogo variant="horizontal" imgClass="h-6 w-auto" />
		</a>
		<div class="flex items-center gap-3">
			<ThemeToggle />
			<LanguagePicker />
		</div>
	</header>
	<main class="mx-auto flex w-full max-w-5xl flex-1 flex-col px-6 py-8 md:px-10 md:py-10">
		{@render children()}
	</main>
</div>
