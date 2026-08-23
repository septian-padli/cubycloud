<script lang="ts">
	import { errorPageContent } from '$lib/error-pages.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
	import { ArrowLeft } from '@lucide/svelte';

	let {
		status,
		message,
		showHeader = true
	}: {
		status: number;
		message?: string;
		showHeader?: boolean;
	} = $props();

	const content = $derived(errorPageContent(status, message));
</script>

<svelte:head>
	<title>{status} — {content.title} | CubyCloud</title>
	<meta name="robots" content="noindex" />
</svelte:head>

{#if showHeader}
	<div class="flex min-h-dvh flex-col bg-app text-main">
		<header class="flex w-full items-center justify-between border-b border-subtle px-6 py-4 md:px-10">
			<a href="/" class="text-sm font-semibold text-main">CubyCloud</a>
			<ThemeToggle />
		</header>

		<main
			class="flex w-full flex-1 flex-col items-center justify-center gap-8 px-6 py-12 text-center md:gap-10 md:px-10 md:py-16"
		>
			<img
				src={content.illustration}
				alt=""
				class="w-4/5 object-contain sm:w-3/5 md:w-1/2"
				decoding="async"
			/>

			<div class="flex max-w-lg flex-col items-center gap-3 px-2">
				<h1 class="text-3xl font-bold leading-tight text-main md:text-4xl">{content.title}</h1>
				<p class="text-base leading-relaxed text-muted">{content.description}</p>
			</div>

			<Button href="/" size="lg" class="gap-1.5">
				<ArrowLeft class="size-4 stroke-2" aria-hidden="true" />
				Kembali ke beranda
			</Button>
		</main>
	</div>
{:else}
	<div
		class="flex w-full flex-1 flex-col items-center justify-center gap-8 py-8 text-center md:gap-10 md:py-12"
	>
		<img
			src={content.illustration}
			alt=""
			class="w-4/5 object-contain sm:w-3/5 md:w-1/2"
			decoding="async"
		/>

		<div class="flex max-w-lg flex-col items-center gap-3 px-2">
			<h1 class="text-3xl font-bold leading-tight text-main md:text-4xl">{content.title}</h1>
			<p class="text-base leading-relaxed text-muted">{content.description}</p>
		</div>

		<Button href="/" size="lg" class="gap-1.5">
			<ArrowLeft class="size-4 stroke-2" aria-hidden="true" />
			Kembali ke beranda
		</Button>
	</div>
{/if}
