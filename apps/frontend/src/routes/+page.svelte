<script lang="ts">
	import { onMount } from 'svelte';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
	import LanguagePicker from '$lib/components/language-picker.svelte';
	import { ArrowRight, Cloud, HardDrive, Shield } from '@lucide/svelte';
	import { getSetupStatus } from '$lib/api/setup';
	import { locale } from '$lib/locale.svelte.js';

	let isSetupCompleted = $state<boolean | null>(null);

	onMount(async () => {
		try {
			const res = await getSetupStatus();
			isSetupCompleted = res.is_setup_completed;
		} catch (e) {
			isSetupCompleted = false;
		}
	});
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);
</script>

<svelte:head>
	<title>CubyCloud — {tx('Segera Hadir', 'Coming Soon')}</title>
	<meta
		name="description"
		content={tx(
			'Penyimpanan cloud modular yang di-host sendiri dan ringan untuk server rumah. Diluncurkan Oktober 2026.',
			'Self-hosted modular cloud storage that is ultra-light on your home server. Launching October 2026.'
		)}
	/>
</svelte:head>

<div class="flex min-h-dvh flex-col bg-app text-main">
	<header
		class="mx-auto flex w-full max-w-5xl flex-wrap items-center justify-between gap-3 border-b border-subtle px-6 py-4 md:px-10"
	>
		<p class="text-sm font-semibold">CubyCloud</p>
		<div class="flex items-center gap-3">
			<ThemeToggle />
			<LanguagePicker />
			<Badge
				class="rounded-full bg-primary/15 px-3 py-1 text-xs font-medium text-primary hover:bg-primary/15"
			>
				{tx('Segera hadir', 'Coming soon')}
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
					<p class="text-xs font-medium text-muted">
						{tx('Penyimpanan cloud yang di-host sendiri', 'Self-hosted cloud storage')}
					</p>
					<h1 class="mt-3 text-2xl leading-tight font-bold md:text-4xl">
						{tx('Simpan dengan privat.', 'Store privately.')}<br />{tx(
							'Bangun dengan tenang.',
							'Build calmly.'
						)}
					</h1>
					<p class="mt-4 text-sm leading-normal text-muted md:text-base">
						{tx(
							'CubyCloud adalah penyimpanan cloud modular untuk homelab dan tim kecil — node lokal dan cloud terpadu, setup tanpa CLI, dan ringan untuk perangkat lama. Rilis pertama ditargetkan ',
							'CubyCloud is modular cloud storage for homelabbers and small teams — unified local and cloud nodes, zero-CLI setup, and ultra-light on legacy hardware. First release targets '
						)}
						<strong class="font-medium text-main">{tx('Oktober 2026', 'October 2026')}</strong>.
					</p>
					<div class="mt-6 flex flex-wrap gap-3">
						{#if isSetupCompleted}
							<Button href="/drive/home" size="lg" class="gap-2">
								{tx('Buka Drive', 'Open Drive')}
							</Button>
							<Button href="/login" variant="outline" size="lg">
								{tx('Masuk', 'Sign in')}
							</Button>
						{:else}
							<Button href="/setup" size="lg" class="gap-2">
								{tx('Mulai Setup', 'Start setup')}
								<ArrowRight class="size-4 stroke-2" />
							</Button>
						{/if}
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
					<p class="text-xs font-medium text-muted">
						{tx('Target performa', 'Performance target')}
					</p>
					<p class="mt-1 text-sm font-medium">
						{tx('RAM idle di bawah 200 MB', 'Idle RAM under 200 MB')}
					</p>
				</article>
				<article class="rounded-xl border border-subtle bg-app px-5 py-5">
					<div
						class="mb-3 flex size-9 items-center justify-center rounded-lg bg-primary/10 text-primary"
					>
						<Shield class="size-4 stroke-2" />
					</div>
					<p class="text-xs font-medium text-muted">{tx('Penerapan', 'Deployment')}</p>
					<p class="mt-1 text-sm font-medium">
						{tx('Satu kontainer Docker', 'Single Docker container')}
					</p>
				</article>
				<article class="rounded-xl border border-subtle bg-app px-5 py-5">
					<div
						class="mb-3 flex size-9 items-center justify-center rounded-lg bg-secondary/20 text-secondary-accent"
					>
						<Cloud class="size-4 stroke-2" />
					</div>
					<p class="text-xs font-medium text-muted">{tx('Penyimpanan', 'Storage')}</p>
					<p class="mt-1 text-sm font-medium">{tx('Disk lokal + S3 / R2', 'Local disk + S3 / R2')}</p>
				</article>
			</div>
		</section>
	</main>

	<section class="border-t border-subtle bg-primary-accent px-6 py-10 text-app md:px-10">
		<div
			class="mx-auto flex w-full max-w-5xl flex-col gap-2 md:flex-row md:items-center md:justify-between"
		>
			<p class="text-lg leading-snug font-semibold">
				{tx('Berkas Anda. Server Anda. Aturan Anda.', 'Your files. Your server. Your rules.')}
			</p>
			<p class="text-xs text-app/70">
				Copyright (C) 2026 CubyCloud contributors. You may convey this work under AGPL-3.0-or-later. No warranty.
				<a class="underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary" href="https://github.com/septian-padli/cubycloud/blob/main/LICENSE" target="_blank" rel="noreferrer">License</a>
				·
				<a class="underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary" href="https://github.com/septian-padli/cubycloud" target="_blank" rel="noreferrer">Source</a>
			</p>
		</div>
	</section>
</div>
