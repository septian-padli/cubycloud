<script lang="ts">
	import type { Component } from 'svelte';
	import type { FileKind } from '$lib/types/drive.js';
	import pdfWorkerUrl from 'pdfjs-dist/build/pdf.worker.min.mjs?url';

	const pdfCache = new Map<string, string>();

	let {
		url,
		name,
		kind,
		fallbackIcon: FallbackIcon
	}: {
		url?: string;
		name: string;
		kind: FileKind;
		fallbackIcon: Component;
	} = $props();

	let hasError = $state(false);
	let isLoaded = $state(false);
	let pdfDataUrl = $state<string | null>(null);

	const extension = $derived(name.split('.').pop()?.toLowerCase() ?? '');
	const mediaType = $derived.by(() => {
		if (!url || kind === 'folder') return 'none';
		if (['png', 'jpg', 'jpeg', 'webp', 'gif', 'svg'].includes(extension)) return 'image';
		if (['mp4', 'webm'].includes(extension)) return 'video';
		if (extension === 'pdf') return 'pdf';
		return 'none';
	});

	let videoEl = $state<HTMLVideoElement>();

	$effect(() => {
		hasError = false;
		isLoaded = false;
		pdfDataUrl = null;

		if (mediaType !== 'pdf' || !url) return;
		if (pdfCache.has(url)) {
			pdfDataUrl = pdfCache.get(url)!;
			isLoaded = true;
			return;
		}

		let disposed = false;
		let task: any;

		void (async () => {
			try {
				const pdfjs = await import('pdfjs-dist');
				if (disposed) return;
				pdfjs.GlobalWorkerOptions.workerSrc = pdfWorkerUrl;
				task = pdfjs.getDocument({ url, withCredentials: true });
				const pdf = await task.promise;
				if (disposed) return;
				const page = await pdf.getPage(1);
				if (disposed) return;

				const natural = page.getViewport({ scale: 1 });
				const scale = Math.min(1.5, 320 / natural.width);
				const viewport = page.getViewport({ scale });

				const canvas = document.createElement('canvas');
				canvas.width = Math.floor(viewport.width);
				canvas.height = Math.floor(viewport.height);
				const ctx = canvas.getContext('2d');
				if (!ctx) throw new Error('No 2d context');

				await page.render({ canvasContext: ctx, canvas, viewport }).promise;
				if (disposed) return;

				const data = canvas.toDataURL('image/jpeg', 0.85);
				pdfCache.set(url, data);
				pdfDataUrl = data;
				isLoaded = true;
			} catch {
				if (!disposed) {
					hasError = true;
				}
			}
		})();

		return () => {
			disposed = true;
			void task?.destroy();
		};
	});
</script>

{#if mediaType === 'image' && !hasError}
	<img
		src={url}
		alt={name}
		loading="lazy"
		class="pointer-events-none absolute inset-0 h-full w-full rounded-lg object-cover select-none"
		data-testid="grid-image-thumbnail"
		onload={() => {
			isLoaded = true;
		}}
		onerror={() => {
			hasError = true;
		}}
	/>
{:else if mediaType === 'video' && !hasError}
	<video
		bind:this={videoEl}
		src={url}
		preload="metadata"
		muted
		playsinline
		tabindex="-1"
		aria-hidden="true"
		class="pointer-events-none absolute inset-0 h-full w-full rounded-lg object-cover select-none"
		data-testid="grid-video-thumbnail"
		onloadedmetadata={() => {
			if (videoEl) {
				videoEl.currentTime = Math.min(1, (videoEl.duration || 1) / 2);
			}
		}}
		onloadeddata={() => {
			isLoaded = true;
		}}
		onseeked={() => {
			isLoaded = true;
		}}
		onerror={() => {
			hasError = true;
		}}
	></video>
{:else if mediaType === 'pdf' && !hasError && pdfDataUrl}
	<img
		src={pdfDataUrl}
		alt={name}
		class="pointer-events-none absolute inset-0 h-full w-full rounded-lg object-cover select-none"
		data-testid="grid-pdf-thumbnail"
	/>
{/if}

{#if hasError || mediaType === 'none' || (!isLoaded && (mediaType === 'pdf' || mediaType === 'image' || mediaType === 'video'))}
	<div class:hidden={isLoaded} class="flex items-center justify-center">
		<FallbackIcon class="size-12 stroke-2" aria-hidden="true" data-testid="grid-fallback-icon" />
	</div>
{/if}
