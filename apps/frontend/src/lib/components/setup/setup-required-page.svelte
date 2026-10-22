<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
	import { Settings } from '@lucide/svelte';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);
	let { showHeader = true }: { showHeader?: boolean } = $props();
</script>

<svelte:head>
	<title>{tx('Setup Diperlukan', 'Setup Required')} | CubyCloud</title>
	<meta name="robots" content="noindex" />
</svelte:head>

{#if showHeader}
	<div class="flex min-h-dvh flex-col bg-app text-main">
		<header
			class="flex w-full items-center justify-between border-b border-subtle px-6 py-4 md:px-10"
		>
			<a href="/" class="text-sm font-semibold text-main">CubyCloud</a>
			<ThemeToggle />
		</header>

		<main
			class="flex w-full flex-1 flex-col items-center justify-center gap-8 px-6 py-12 text-center md:gap-10 md:px-10 md:py-16"
		>
			<div
				class="flex size-24 items-center justify-center rounded-full bg-primary/10 text-primary md:size-28"
			>
				<Settings class="size-12 stroke-[1.5] md:size-14" aria-hidden="true" />
			</div>

			<div class="flex max-w-lg flex-col items-center gap-3 px-2">
				<h1 class="text-3xl leading-tight font-bold text-main md:text-4xl">
					{tx('Setup Belum Selesai', 'Setup Not Complete')}
				</h1>
				<p class="text-base leading-relaxed text-muted md:text-lg">
					{tx(
						'CubyCloud belum dikonfigurasi. Silakan selesaikan setup awal untuk mulai menggunakan sistem penyimpanan cloud Anda.',
						'CubyCloud is not configured yet. Complete the initial setup to start using your cloud storage system.'
					)}
				</p>
			</div>

			<Button href="/setup" size="lg" class="gap-2">
				<Settings class="size-4 stroke-2" aria-hidden="true" />
				{tx('Mulai Setup', 'Start Setup')}
			</Button>
		</main>
	</div>
{:else}
	<div
		class="flex w-full flex-1 flex-col items-center justify-center gap-8 py-8 text-center md:gap-10 md:py-12"
	>
		<div
			class="flex size-24 items-center justify-center rounded-full bg-primary/10 text-primary md:size-28"
		>
			<Settings class="size-12 stroke-[1.5] md:size-14" aria-hidden="true" />
		</div>

		<div class="flex max-w-lg flex-col items-center gap-3 px-2">
			<h1 class="text-3xl leading-tight font-bold text-main md:text-4xl">
				{tx('Setup Belum Selesai', 'Setup Not Complete')}
			</h1>
			<p class="text-base leading-relaxed text-muted md:text-lg">
				{tx(
					'CubyCloud belum dikonfigurasi. Silakan selesaikan setup awal untuk mulai menggunakan sistem penyimpanan cloud Anda.',
					'CubyCloud is not configured yet. Complete the initial setup to start using your cloud storage system.'
				)}
			</p>
		</div>

		<Button href="/setup" size="lg" class="gap-2">
			<Settings class="size-4 stroke-2" aria-hidden="true" />
			{tx('Mulai Setup', 'Start Setup')}
		</Button>
	</div>
{/if}
