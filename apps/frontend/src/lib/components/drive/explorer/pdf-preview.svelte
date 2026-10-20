<script lang="ts">
	import pdfWorkerUrl from 'pdfjs-dist/build/pdf.worker.min.mjs?url';
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
					error = 'Gagal memuat PDF. Unduh berkas untuk membukanya di perangkat Anda.';
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
				if (!context) throw new Error('Canvas tidak tersedia');
				render = loadedPage.render({ canvasContext: context, canvas, viewport });
				previousRender = render.promise.catch(() => {});
				await render.promise;
				if (!disposed) loading = false;
			} catch {
				if (!disposed) {
					loading = false;
					error = 'Gagal menampilkan halaman PDF. Unduh berkas untuk membukanya.';
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
		<nav aria-label="Navigasi halaman PDF" class="flex items-center gap-3 text-sm">
			<button type="button" class="rounded-lg border border-subtle px-3 py-1 disabled:opacity-50" disabled={pageNumber <= 1} onclick={() => pageNumber--}>Sebelumnya</button>
			<span>Halaman {pageNumber} dari {document.numPages}</span>
			<button type="button" class="rounded-lg border border-subtle px-3 py-1 disabled:opacity-50" disabled={pageNumber >= document.numPages} onclick={() => pageNumber++}>Berikutnya</button>
		</nav>
	{/if}
	{#if loading}<p role="status" class="text-sm text-muted">Memuat PDF…</p>{/if}
	{#if error}<p role="alert" class="text-sm text-muted">{error}</p>{/if}
	<canvas bind:this={canvas} aria-label="Halaman PDF {pageNumber}" class="max-h-96 max-w-full object-contain" class:hidden={loading || !!error}></canvas>
</div>
