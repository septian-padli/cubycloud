<script lang="ts">
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Cloud, HardDrive, Shield } from '@lucide/svelte';
</script>

<svelte:head>
	<title>CubyCloud — Coming Soon</title>
	<meta
		name="description"
		content="Self-hosted modular cloud storage that's ultra-light on your home server. Launching October 2026."
	/>
</svelte:head>

<div class="min-h-dvh bg-cc-canvas text-cc-ink">
	<header class="mx-auto flex max-w-5xl items-center justify-between px-6 py-4 md:px-10">
		<p class="text-base tracking-tight">CubyCloud</p>
		<Badge
			class="rounded-cc-pill border border-cc-ink bg-cc-secondary px-4 py-1.5 text-xs font-normal text-cc-ink hover:bg-cc-secondary"
		>
			Coming soon
		</Badge>
	</header>

	<main class="mx-auto max-w-5xl px-6 pt-8 pb-20 md:px-10 md:pt-16">
		<section
			class="relative overflow-hidden rounded-cc-tile bg-cc-surface-dark px-6 py-16 text-cc-ink-inverse md:px-12 md:py-24"
		>
			<div class="pointer-events-none absolute inset-0 opacity-40">
				<div
					class="absolute top-10 -left-8 flex size-14 items-center justify-center rounded-full bg-cc-secondary text-cc-ink"
				>
					<Cloud class="size-6" />
				</div>
				<div
					class="absolute top-20 right-8 flex size-12 items-center justify-center rounded-full bg-white/10 text-cc-ink-inverse"
				>
					<HardDrive class="size-5" />
				</div>
				<div
					class="absolute bottom-10 left-1/3 flex size-10 items-center justify-center rounded-full bg-cc-primary text-cc-ink"
				>
					<Shield class="size-4" />
				</div>
			</div>

			<div class="relative max-w-2xl">
				<p class="mb-4 text-sm text-white/70">Self-hosted cloud storage</p>
				<h1 class="text-4xl leading-[1.05] tracking-tight md:text-6xl md:leading-[1.05]">
					Store privately.<br />Build calmly.
				</h1>
				<p class="mt-6 max-w-xl text-base leading-relaxed text-white/80 md:text-lg">
					CubyCloud is a modular cloud workspace for homelabbers and small teams — unified storage,
					zero-CLI setup, and ultra-light on legacy hardware. We are preparing the first release for
					<strong class="font-normal text-white">October 2026</strong>.
				</p>
			</div>
		</section>

		<section class="mt-10 grid gap-4 md:grid-cols-3">
			<article class="rounded-cc-tile border border-cc-ink/10 bg-white px-5 py-6">
				<p class="text-sm text-cc-ink-muted">Performance target</p>
				<p class="mt-2 text-lg">Idle RAM under 200 MB</p>
			</article>
			<article class="rounded-cc-tile border border-cc-ink/10 bg-white px-5 py-6">
				<p class="text-sm text-cc-ink-muted">Deployment</p>
				<p class="mt-2 text-lg">Single Docker container</p>
			</article>
			<article class="rounded-cc-tile border border-cc-ink/10 bg-white px-5 py-6">
				<p class="text-sm text-cc-ink-muted">Storage</p>
				<p class="mt-2 text-lg">Local disks + S3 / R2</p>
			</article>
		</section>
	</main>

	<footer class="mx-auto max-w-5xl px-6 pb-10 text-sm text-cc-ink-muted md:px-10">
		<p>© {new Date().getFullYear()} CubyCloud. All rights reserved.</p>
	</footer>
</div>
