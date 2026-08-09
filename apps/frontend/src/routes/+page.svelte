<script lang="ts">
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
	import { ArrowRight, Cloud, HardDrive, Shield } from '@lucide/svelte';
</script>

<svelte:head>
	<title>CubyCloud — Coming Soon</title>
	<meta
		name="description"
		content="Self-hosted modular cloud storage that's ultra-light on your home server. Launching October 2026."
	/>
</svelte:head>

<div class="flex min-h-dvh flex-col bg-app text-main">
	<header
		class="mx-auto flex w-full max-w-5xl items-center justify-between border-b border-subtle px-6 py-4 md:px-10"
	>
		<p class="text-sm font-semibold">CubyCloud</p>
		<div class="flex items-center gap-3">
			<ThemeToggle />
			<Badge
				class="rounded-full bg-primary/15 px-3 py-1 text-xs font-medium text-primary hover:bg-primary/15"
			>
				Coming soon
			</Badge>
		</div>
	</header>

	<main class="mx-auto w-full max-w-5xl flex-1 px-6 py-10 md:px-10 md:py-16">
		<section class="rounded-xl border border-subtle bg-surface px-6 py-12 md:px-10 md:py-16">
			<div class="flex flex-wrap items-start gap-4">
				<div class="rounded-lg bg-primary/10 p-3 text-primary">
					<Cloud class="size-6 stroke-2" />
				</div>
				<div class="max-w-2xl flex-1">
					<p class="text-xs font-medium text-muted">Self-hosted cloud storage</p>
					<h1 class="mt-3 text-2xl leading-tight font-bold md:text-4xl">
						Store privately.<br />Build calmly.
					</h1>
					<p class="mt-4 text-sm leading-normal text-muted md:text-base">
						CubyCloud is modular cloud storage for homelabbers and small teams — unified local and
						cloud nodes, zero-CLI setup, and ultra-light on legacy hardware. First release targets
						<strong class="font-medium text-main">October 2026</strong>.
					</p>
					<div class="mt-6 flex flex-wrap gap-3">
						<Button href="/setup" size="lg" class="gap-2">
							Mulai Setup
							<ArrowRight class="size-4 stroke-2" />
						</Button>
						<Button href="/login" variant="outline" size="lg">
							Masuk
						</Button>
					</div>
				</div>
			</div>

			<div class="mt-10 grid gap-4 md:grid-cols-3">
				<article class="rounded-xl border border-subtle bg-app px-5 py-5">
					<div
						class="mb-3 flex size-9 items-center justify-center rounded-lg bg-secondary/20 text-secondary-accent"
					>
						<HardDrive class="size-4 stroke-2" />
					</div>
					<p class="text-xs font-medium text-muted">Performance target</p>
					<p class="mt-1 text-sm font-medium">Idle RAM under 200 MB</p>
				</article>
				<article class="rounded-xl border border-subtle bg-app px-5 py-5">
					<div
						class="mb-3 flex size-9 items-center justify-center rounded-lg bg-primary/10 text-primary"
					>
						<Shield class="size-4 stroke-2" />
					</div>
					<p class="text-xs font-medium text-muted">Deployment</p>
					<p class="mt-1 text-sm font-medium">Single Docker container</p>
				</article>
				<article class="rounded-xl border border-subtle bg-app px-5 py-5">
					<div
						class="mb-3 flex size-9 items-center justify-center rounded-lg bg-secondary/20 text-secondary-accent"
					>
						<Cloud class="size-4 stroke-2" />
					</div>
					<p class="text-xs font-medium text-muted">Storage</p>
					<p class="mt-1 text-sm font-medium">Local disks + S3 / R2</p>
				</article>
			</div>
		</section>
	</main>

	<section class="border-t border-subtle bg-primary-accent px-6 py-10 text-app md:px-10">
		<div
			class="mx-auto flex w-full max-w-5xl flex-col gap-2 md:flex-row md:items-center md:justify-between"
		>
			<p class="text-lg leading-snug font-semibold">Your files. Your server. Your rules.</p>
			<p class="text-xs text-app/70">© {new Date().getFullYear()} CubyCloud</p>
		</div>
	</section>
</div>
