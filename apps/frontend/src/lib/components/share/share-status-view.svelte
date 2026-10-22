<script lang="ts">
	import errorIllustration from '$lib/assets/illustration/404.webp';
	import { Button } from '$lib/components/ui/button/index.js';
	import { locale } from '$lib/locale.svelte.js';

	let {
		title,
		description,
		ctaHref = '/',
		ctaLabel
	}: {
		title: string;
		description: string;
		ctaHref?: string;
		ctaLabel?: string;
	} = $props();
</script>

<div class="flex flex-1 flex-col items-center justify-center gap-8 py-8 text-center">
	<img
		src={errorIllustration}
		alt=""
		class="w-4/5 object-contain sm:w-3/5 md:w-1/2"
		decoding="async"
	/>
	<div class="flex max-w-lg flex-col items-center gap-3">
		<h1 class="text-3xl leading-tight font-bold text-main md:text-4xl">{title}</h1>
		<p class="text-base leading-relaxed text-muted">{description}</p>
	</div>
	<Button href={ctaHref} size="lg"
		>{ctaLabel ?? (locale.language === 'id' ? 'Kembali ke beranda' : 'Back to home')}</Button
	>
</div>
