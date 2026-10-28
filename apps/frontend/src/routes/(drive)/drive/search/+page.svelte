<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { searchFiles, type SearchItem } from '$lib/api/search.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import {
		Search,
		Folder,
		FileText,
		FileImage,
		FileVideo,
		FileAudio,
		File,
		Loader2,
		AlertCircle,
		ChevronLeft,
		ChevronRight
	} from '@lucide/svelte';
	import { locale } from '$lib/locale.svelte.js';

	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	const PAGE_SIZE = 25;

	const query = $derived(page.url.searchParams.get('q') ?? '');
	const currentPage = $derived(
		Math.max(1, parseInt(page.url.searchParams.get('page') ?? '1', 10) || 1)
	);
	const offset = $derived((currentPage - 1) * PAGE_SIZE);

	let items = $state<SearchItem[]>([]);
	let total = $state(0);
	let loading = $state(false);
	let errorMessage = $state<string | null>(null);

	const totalPages = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));
	const fromItem = $derived(total === 0 ? 0 : offset + 1);
	const toItem = $derived(Math.min(offset + PAGE_SIZE, total));

	$effect(() => {
		const trimmed = query.trim();
		if (!trimmed) {
			items = [];
			total = 0;
			loading = false;
			errorMessage = null;
			return;
		}

		loading = true;
		errorMessage = null;

		const ac = new AbortController();

		searchFiles(trimmed, { limit: PAGE_SIZE, offset, signal: ac.signal })
			.then((res) => {
				if (!ac.signal.aborted) {
					items = res.items;
					total = res.total;
					loading = false;
				}
			})
			.catch((err: unknown) => {
				if (!ac.signal.aborted) {
					loading = false;
					errorMessage =
						err instanceof Error
							? err.message
							: tx(
									'Terjadi kesalahan saat memuat hasil pencarian',
									'An error occurred while loading search results'
								);
				}
			});

		return () => ac.abort();
	});

	function goToPage(p: number) {
		if (p < 1 || p > totalPages || p === currentPage) return;
		const params = new URLSearchParams(page.url.searchParams);
		params.set('page', String(p));
		void goto(`/drive/search?${params.toString()}`);
	}

	function openItem(item: SearchItem) {
		let href = '';
		if (item.source === 'my-storage') {
			if (item.type === 'folder') {
				href = `/drive/my-storage?folder=${encodeURIComponent(item.id)}`;
			} else {
				href = item.parent_id
					? `/drive/my-storage?folder=${encodeURIComponent(item.parent_id)}&preview=${encodeURIComponent(item.id)}`
					: `/drive/my-storage?preview=${encodeURIComponent(item.id)}`;
			}
		} else if (item.source === 'organization') {
			if (item.type === 'folder') {
				href = `/drive/organization?folder=${encodeURIComponent(item.id)}`;
			} else {
				href = item.parent_id
					? `/drive/organization?folder=${encodeURIComponent(item.parent_id)}&preview=${encodeURIComponent(item.id)}`
					: `/drive/organization?preview=${encodeURIComponent(item.id)}`;
			}
		} else {
			// shared
			if (item.type === 'folder') {
				href = `/drive/shared?folder=${encodeURIComponent(item.id)}`;
			} else {
				href = item.parent_id
					? `/drive/shared?folder=${encodeURIComponent(item.parent_id)}&preview=${encodeURIComponent(item.id)}`
					: `/drive/shared?preview=${encodeURIComponent(item.id)}`;
			}
		}
		void goto(href);
	}

	function getIcon(item: SearchItem) {
		if (item.type === 'folder') return Folder;
		const ext = item.name.split('.').pop()?.toLowerCase() ?? '';
		if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg'].includes(ext)) return FileImage;
		if (['mp4', 'webm', 'mov', 'mkv'].includes(ext)) return FileVideo;
		if (['mp3', 'wav', 'ogg', 'm4a'].includes(ext)) return FileAudio;
		if (['pdf', 'txt', 'md', 'doc', 'docx', 'csv', 'json'].includes(ext)) return FileText;
		return File;
	}

	function formatDate(isoString: string): string {
		return locale.formatDate(isoString);
	}
</script>

<svelte:head>
	<title>
		{query
			? tx(`Hasil pencarian untuk "${query}"`, `Search results for "${query}"`)
			: tx('Pencarian Berkas', 'File Search')} — CubyCloud
	</title>
	<meta
		name="description"
		content={tx(
			'Hasil pencarian berkas dan folder di CubyCloud.',
			'Search results for files and folders in CubyCloud.'
		)}
	/>
</svelte:head>

<div class="flex flex-col gap-6" data-testid="search-results-page">
	<DrivePageHeader
		title={tx('Pencarian Berkas', 'File Search')}
		description={query.trim()
			? tx(
					`Hasil pencarian untuk "${query}" (${new Intl.NumberFormat(locale.language === 'id' ? 'id-ID' : 'en-US').format(total)} ditemukan)`,
					`Search results for "${query}" (${new Intl.NumberFormat(locale.language === 'id' ? 'id-ID' : 'en-US').format(total)} found)`
				)
			: tx(
					'Cari berkas dan folder di My Storage, Organisasi, dan Dibagikan.',
					'Search files and folders in My Storage, Organization, and Shared.'
				)}
	/>

	{#if errorMessage}
		<div
			class="flex items-center gap-3 rounded-xl border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive"
			role="alert"
			data-testid="search-error"
		>
			<AlertCircle class="size-5 shrink-0" />
			<div class="flex-1">
				<p class="font-medium">
					{tx('Gagal memuat hasil pencarian', 'Failed to load search results')}
				</p>
				<p class="text-xs opacity-90">{errorMessage}</p>
			</div>
			<Button
				variant="outline"
				size="sm"
				class="shrink-0"
				onclick={() => {
					const p = currentPage;
					goToPage(p);
				}}
			>
				{tx('Coba Lagi', 'Try Again')}
			</Button>
		</div>
	{:else if !query.trim()}
		<div
			class="flex flex-col items-center justify-center rounded-2xl border border-dashed border-subtle bg-surface/50 py-16 text-center"
			data-testid="search-empty-query"
		>
			<div class="flex size-14 items-center justify-center rounded-2xl bg-muted/20 text-muted">
				<Search class="size-7 stroke-2" />
			</div>
			<h2 class="mt-4 text-base font-semibold text-foreground">
				{tx('Mulai Pencarian', 'Start Searching')}
			</h2>
			<p class="mt-1.5 max-w-sm text-sm text-muted">
				{tx(
					'Ketik nama berkas atau folder pada kotak pencarian di bagian atas untuk mencari di seluruh penyimpanan.',
					'Type a file or folder name in the search box above to search across all your storage.'
				)}
			</p>
		</div>
	{:else if loading}
		<div
			class="flex flex-col items-center justify-center rounded-2xl border border-subtle bg-surface/50 py-16 text-center"
			data-testid="search-loading"
		>
			<Loader2 class="size-8 animate-spin text-primary" />
			<p class="mt-3 text-sm text-muted">
				{tx('Mencari berkas yang cocok…', 'Searching for matching files…')}
			</p>
		</div>
	{:else if items.length === 0}
		<div
			class="flex flex-col items-center justify-center rounded-2xl border border-dashed border-subtle bg-surface/50 py-16 text-center"
			data-testid="search-empty-results"
		>
			<div class="flex size-14 items-center justify-center rounded-2xl bg-muted/20 text-muted">
				<Search class="size-7 stroke-2" />
			</div>
			<h2 class="mt-4 text-base font-semibold text-foreground">
				{tx('Tidak ada berkas yang cocok dengan pencarian Anda', 'No files match your search')}
			</h2>
			<p class="mt-1.5 max-w-sm text-sm text-muted">
				{tx(
					'Tidak ditemukan berkas atau folder dengan nama “',
					'No files or folders found with the name “'
				)}<span class="font-medium text-foreground">{query}</span>{tx(
					'”. Coba gunakan kata kunci lain atau periksa ejaan Anda.',
					'”. Try another keyword or check your spelling.'
				)}
			</p>
		</div>
	{:else}
		<!-- Table of search results -->
		<div class="overflow-hidden rounded-2xl border border-subtle bg-surface shadow-sm">
			<div class="overflow-x-auto">
				<table class="w-full text-start text-sm" data-testid="search-results-table">
					<thead class="border-b border-subtle bg-accent/30 text-xs font-medium text-muted">
						<tr>
							<th scope="col" class="px-4 py-3 text-start">{tx('Nama', 'Name')}</th>
							<th scope="col" class="px-4 py-3 text-start">{tx('Tipe', 'Type')}</th>
							<th scope="col" class="px-4 py-3 text-start">{tx('Sumber', 'Source')}</th>
							<th scope="col" class="px-4 py-3 text-start">{tx('Lokasi', 'Location')}</th>
							<th scope="col" class="px-4 py-3 text-end">{tx('Ukuran', 'Size')}</th>
							<th scope="col" class="px-4 py-3 text-start"
								>{tx('Terakhir Diubah', 'Last Modified')}</th
							>
						</tr>
					</thead>
					<tbody class="divide-y divide-subtle/60">
						{#each items as item (item.id)}
							{@const Icon = getIcon(item)}
							<tr
								tabindex="0"
								role="button"
								class="cursor-pointer transition-colors hover:bg-accent/40 focus:bg-accent/40 focus:outline-none"
								onclick={() => openItem(item)}
								onkeydown={(e) => {
									if (e.key === 'Enter' || e.key === ' ') {
										e.preventDefault();
										openItem(item);
									}
								}}
								data-testid="search-row-{item.id}"
							>
								<!-- Name & Icon -->
								<td class="px-4 py-3 font-medium text-foreground">
									<div class="flex items-center gap-3">
										<div
											class="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary"
										>
											<Icon class="size-4 stroke-2" />
										</div>
										<span
											class="max-w-xs truncate font-medium text-foreground hover:underline md:max-w-md"
										>
											{item.name}
										</span>
									</div>
								</td>

								<!-- Type -->
								<td class="px-4 py-3 text-muted">
									{item.type === 'folder' ? tx('Folder', 'Folder') : tx('Berkas', 'File')}
								</td>

								<!-- Source -->
								<td class="px-4 py-3">
									{#if item.source === 'my-storage'}
										<span
											class="inline-flex rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary"
										>
											My Storage
										</span>
									{:else if item.source === 'organization'}
										<span
											class="inline-flex rounded-full bg-violet-500/10 px-2 py-0.5 text-xs font-medium text-violet-600 dark:text-violet-400"
										>
											{tx('Organisasi', 'Organization')}
										</span>
									{:else}
										<span
											class="inline-flex rounded-full bg-emerald-500/10 px-2 py-0.5 text-xs font-medium text-emerald-600 dark:text-emerald-400"
										>
											{tx('Dibagikan', 'Shared')}
										</span>
									{/if}
								</td>

								<!-- Location -->
								<td class="px-4 py-3 text-muted">
									<span class="block max-w-xs truncate" title={item.location}>
										{item.location}
									</span>
								</td>

								<!-- Size -->
								<td class="px-4 py-3 text-end font-mono text-xs text-muted">
									{item.type === 'folder' ? '—' : formatSize(item.size_bytes)}
								</td>

								<!-- Last Modified -->
								<td class="px-4 py-3 text-xs whitespace-nowrap text-muted">
									{formatDate(item.updated_at)}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			<!-- Pagination Controls -->
			<div
				class="flex flex-col gap-3 border-t border-subtle bg-surface/50 px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
				data-testid="search-pagination"
			>
				<p class="text-xs text-muted">
					{tx('Menampilkan', 'Showing')}
					<span class="font-medium text-foreground"
						>{new Intl.NumberFormat(locale.language === 'id' ? 'id-ID' : 'en-US').format(
							fromItem
						)}</span
					>–<span class="font-medium text-foreground"
						>{new Intl.NumberFormat(locale.language === 'id' ? 'id-ID' : 'en-US').format(
							toItem
						)}</span
					>
					{tx('dari', 'of')}
					<span class="font-medium text-foreground"
						>{new Intl.NumberFormat(locale.language === 'id' ? 'id-ID' : 'en-US').format(
							total
						)}</span
					>
					{tx('hasil', 'results')}
				</p>

				<nav
					class="flex items-center gap-1.5"
					aria-label={tx('Navigasi halaman', 'Page navigation')}
				>
					<Button
						variant="outline"
						size="sm"
						class="gap-1 px-2.5"
						disabled={currentPage <= 1}
						onclick={() => goToPage(currentPage - 1)}
						aria-label={tx('Halaman sebelumnya', 'Previous page')}
						data-testid="search-page-prev"
					>
						<ChevronLeft class="size-4 stroke-2" />
						<span class="hidden sm:inline">{tx('Sebelumnya', 'Previous')}</span>
					</Button>

					<div class="flex items-center gap-1">
						{#each Array.from({ length: totalPages }, (_, i) => i + 1) as p}
							{#if p === 1 || p === totalPages || (p >= currentPage - 2 && p <= currentPage + 2)}
								<Button
									variant={p === currentPage ? 'default' : 'ghost'}
									size="sm"
									class="size-8 p-0 text-xs"
									aria-current={p === currentPage ? 'page' : undefined}
									onclick={() => goToPage(p)}
									data-testid="search-page-{p}"
								>
									{p}
								</Button>
							{:else if p === currentPage - 3 || p === currentPage + 3}
								<span class="px-1 text-xs text-muted">…</span>
							{/if}
						{/each}
					</div>

					<Button
						variant="outline"
						size="sm"
						class="gap-1 px-2.5"
						disabled={currentPage >= totalPages}
						onclick={() => goToPage(currentPage + 1)}
						aria-label={tx('Halaman selanjutnya', 'Next page')}
						data-testid="search-page-next"
					>
						<span class="hidden sm:inline">{tx('Selanjutnya', 'Next')}</span>
						<ChevronRight class="size-4 stroke-2" />
					</Button>
				</nav>
			</div>
		</div>
	{/if}
</div>
