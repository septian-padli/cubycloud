<script lang="ts">
	import { errorPageContent } from '$lib/error-pages.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
	import { ArrowLeft } from '@lucide/svelte';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);
	let {
		status,
		message,
		showHeader = true,
		homeHref = '/drive/my-storage'
	}: {
		status: number;
		message?: string;
		showHeader?: boolean;
		homeHref?: string;
	} = $props();

	const content = $derived(errorPageContent(status, message));
	const title = $derived(
		locale.language === 'id'
			? content.title
			: ({
					401: 'Sign in required',
					403: 'Access denied',
					404: 'Page not found',
					500: 'Server error',
					503: 'Service unavailable'
				}[status] ?? 'An error occurred')
	);
	const description = $derived(
		locale.language === 'id'
			? message === 'Setup sistem belum diselesaikan. Silakan selesaikan setup terlebih dahulu.'
				? message
				: content.description
			: message === 'Setup awal telah selesai. Halaman setup tidak dapat diakses kembali.' ||
				  message === 'Initial setup is complete. This setup page can no longer be accessed.'
				? 'Initial setup is complete. The setup page can no longer be accessed.'
				: message === 'Setup sistem belum diselesaikan. Silakan selesaikan setup terlebih dahulu.' ||
					  message === 'System setup is incomplete. Please complete setup first.'
					? 'System setup is incomplete. Please complete setup first.'
					: ({
							401: 'Sign in to your CubyCloud account to continue.',
							403: 'You do not have permission to view this page.',
							404: 'The link may be invalid or the page may have moved.',
							500: 'A temporary issue occurred. Please try again shortly.',
							503: 'The server is busy or under maintenance. Please try again later.'
						}[status] ?? 'An error occurred while loading the page. Please try again.')
	);
</script>

<svelte:head>
	<title>{status} — {title} | CubyCloud</title>
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
			<img
				src={content.illustration}
				alt=""
				class="w-4/5 object-contain sm:w-3/5 md:w-1/2"
				decoding="async"
			/>

			<div class="flex max-w-lg flex-col items-center gap-3 px-2">
				<h1 class="text-3xl leading-tight font-bold text-main md:text-4xl">{title}</h1>
				<p class="text-base leading-relaxed text-muted">{description}</p>
			</div>

			<Button href={homeHref} size="lg" class="gap-1.5">
				<ArrowLeft class="size-4 stroke-2" aria-hidden="true" />
				{tx('Kembali ke beranda', 'Back to home')}
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
			<h1 class="text-3xl leading-tight font-bold text-main md:text-4xl">{title}</h1>
			<p class="text-base leading-relaxed text-muted">{description}</p>
		</div>

		<Button href={homeHref} size="lg" class="gap-1.5">
			<ArrowLeft class="size-4 stroke-2" aria-hidden="true" />
			{tx('Kembali ke beranda', 'Back to home')}
		</Button>
	</div>
{/if}
