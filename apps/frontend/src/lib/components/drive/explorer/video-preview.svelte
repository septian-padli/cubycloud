<script lang="ts">
	import 'vidstack/player/styles/default/theme.css';
	import 'vidstack/player/styles/default/layouts/video.css';
	import type {} from 'vidstack/svelte';
	import { onMount } from 'svelte';
	import { locale } from '$lib/locale.svelte.js';

	let { url, name }: { url: string; name: string } = $props();
	let player = $state<HTMLElement>();
	let ready = $state(false);
	let error = $state(false);

	onMount(() => {
		let disposed = false;
		void Promise.all([
			import('vidstack/player'),
			import('vidstack/player/layouts/default'),
			import('vidstack/player/ui')
		])
			.then(() => {
				if (!disposed) ready = true;
			})
			.catch(() => {
				if (!disposed) error = true;
			});
		return () => {
			disposed = true;
		};
	});

	$effect(() => {
		const target = player;
		if (!target) return;
		const onError = () => {
			error = true;
		};
		target.addEventListener('media-error', onError);
		return () => target.removeEventListener('media-error', onError);
	});
</script>

{#if error}
	<p role="alert" class="text-sm text-muted">
		{locale.language === 'id'
			? 'Video gagal diputar. Unduh berkas untuk membukanya di perangkat Anda.'
			: 'Unable to play this video. Download the file to open it on your device.'}
	</p>
{:else if ready}
	<media-player
		bind:this={player}
		title={name}
		src={{ src: url, type: name.toLowerCase().endsWith('.webm') ? 'video/webm' : 'video/mp4' }}
		load="eager"
		preload="metadata"
		playsinline
		class="w-full overflow-hidden rounded-xl bg-black"
		data-testid="video-preview"
	>
		<media-provider></media-provider>
		<media-video-layout></media-video-layout>
	</media-player>
{:else}
	<p role="status" class="text-sm text-muted">
		{locale.language === 'id' ? 'Memuat video…' : 'Loading video…'}
	</p>
{/if}
