<script lang="ts">
	import pdfWorkerUrl from 'pdfjs-dist/build/pdf.worker.min.mjs?url';
	import { locale } from '$lib/locale.svelte.js';
	let { url }: { url: string } = $props();
	let canvas: HTMLCanvasElement;
	let document: import('pdfjs-dist').PDFDocumentProxy | null = $state(null);
	let pageNumber = $state(1);
	let loading = $state(true);
	let error = $state('');
	let previousRender: Promise<void> = Promise.resolve();

	$effect(() => {
		const source = url;
		let disposed = false;
		let task: import('pdfjs-dist').PDFDocumentLoadingTask | undefined;
		let loaded: import('pdfjs-dist').PDFDocumentProxy | undefined;
		pageNumber = 1;
		document = null;
		loading = true;
		error = '';
		void (async () => {
			try {
				const pdfjs = await import('pdfjs-dist');
				if (disposed) return;
				pdfjs.GlobalWorkerOptions.workerSrc = pdfWorkerUrl;
				task = pdfjs.getDocument({ url: source, withCredentials: true });
				loaded = await task.promise;
				if (disposed) return;
				document = loaded;
			} catch {
				if (!disposed) {
					loading = false;
					error =
						locale.language === 'id'
							? 'Gagal memuat PDF. Unduh berkas untuk membukanya di perangkat Anda.'
							: 'Unable to load the PDF. Download the file to open it on your device.';
				}
			}
		})();
		return () => {
			disposed = true;
			document = null;
			void task?.destroy();
		};
	});

	$effect(() => {
		const pdf = document;
		const number = pageNumber;
		if (!pdf || !canvas) return;
		let disposed = false;
		let render: import('pdfjs-dist').RenderTask | undefined;
		let loadedPage: import('pdfjs-dist').PDFPageProxy | undefined;
		loading = true;
		error = '';
		void (async () => {
			try {
				await previousRender;
				if (disposed) return;
				loadedPage = await pdf.getPage(number);
				if (disposed) return;
				const natural = loadedPage.getViewport({ scale: 1 });
				const scale = Math.min(2, Math.sqrt(3_000_000 / (natural.width * natural.height)));
				const viewport = loadedPage.getViewport({ scale });
				canvas.width = Math.floor(viewport.width);
				canvas.height = Math.floor(viewport.height);
				const context = canvas.getContext('2d');
				if (!context)
					throw new Error(
						locale.language === 'id' ? 'Canvas tidak tersedia' : 'Canvas is unavailable'
					);
				render = loadedPage.render({ canvasContext: context, canvas, viewport });
				previousRender = render.promise.catch(() => {});
				await render.promise;
				if (!disposed) loading = false;
			} catch {
				if (!disposed) {
					loading = false;
					error =
						locale.language === 'id'
							? 'Gagal menampilkan halaman PDF. Unduh berkas untuk membukanya.'
							: 'Unable to display the PDF page. Download the file to open it.';
				}
			}
		})();
		return () => {
			disposed = true;
			render?.cancel();
			void previousRender.then(() => loadedPage?.cleanup());
		};
	});
</script>

<div class="flex w-full flex-col items-center gap-3" data-testid="pdf-preview">
	{#if document}
		<nav
			aria-label={locale.language === 'id' ? 'Navigasi halaman PDF' : 'PDF page navigation'}
			class="flex items-center gap-3 text-sm"
		>
			<button
				type="button"
				class="rounded-lg border border-subtle px-3 py-1 disabled:opacity-50"
				disabled={pageNumber <= 1}
				onclick={() => pageNumber--}>{locale.language === 'id' ? 'Sebelumnya' : 'Previous'}</button
			>
			<span
				>{locale.language === 'id'
					? `Halaman ${pageNumber} dari ${document.numPages}`
					: `Page ${pageNumber} of ${document.numPages}`}</span
			>
			<button
				type="button"
				class="rounded-lg border border-subtle px-3 py-1 disabled:opacity-50"
				disabled={pageNumber >= document.numPages}
				onclick={() => pageNumber++}>{locale.language === 'id' ? 'Berikutnya' : 'Next'}</button
			>
		</nav>
	{/if}
	{#if loading}<p role="status" class="text-sm text-muted">
			{locale.language === 'id' ? 'Memuat PDF…' : 'Loading PDF…'}
		</p>{/if}
	{#if error}<p role="alert" class="text-sm text-muted">{error}</p>{/if}
	<canvas
		bind:this={canvas}
		aria-label={locale.language === 'id' ? `Halaman PDF ${pageNumber}` : `PDF page ${pageNumber}`}
		class="max-h-96 max-w-full object-contain"
		class:hidden={loading || !!error}
	></canvas>
</div>
