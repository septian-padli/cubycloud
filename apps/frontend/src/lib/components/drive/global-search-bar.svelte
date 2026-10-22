<script lang="ts">
	import { locale } from '$lib/locale.svelte.js';
	import { goto } from '$app/navigation';
	import { Input } from '$lib/components/ui/input/index.js';
	import { searchFiles, type SearchItem } from '$lib/api/search.js';
	import {
		Search,
		Loader2,
		Folder,
		FileText,
		FileImage,
		FileVideo,
		FileAudio,
		File,
		ArrowRight,
		X
	} from '@lucide/svelte';
	import { onMount } from 'svelte';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let searchQuery = $state('');
	let debouncedQuery = $state('');
	let isOpen = $state(false);
	let isLoading = $state(false);
	let previewItems = $state<SearchItem[]>([]);
	let selectedIndex = $state(-1);

	let inputEl = $state<HTMLInputElement | null>(null);
	let containerEl = $state<HTMLDivElement | null>(null);
	let abortController: AbortController | null = null;

	const trimmedQuery = $derived(searchQuery.trim());
	const totalOptions = $derived(previewItems.length + (trimmedQuery ? 1 : 0));

	$effect(() => {
		const q = searchQuery.trim();
		if (!q) {
			debouncedQuery = '';
			previewItems = [];
			isLoading = false;
			isOpen = false;
			selectedIndex = -1;
			return;
		}

		const timer = setTimeout(() => {
			debouncedQuery = q;
		}, 300);

		return () => clearTimeout(timer);
	});

	$effect(() => {
		if (!debouncedQuery) return;

		abortController?.abort();
		const ac = new AbortController();
		abortController = ac;

		isLoading = true;
		isOpen = true;
		selectedIndex = -1;

		searchFiles(debouncedQuery, { limit: 5, signal: ac.signal })
			.then((res) => {
				if (!ac.signal.aborted) {
					previewItems = res.items;
					isLoading = false;
				}
			})
			.catch((err) => {
				if (!ac.signal.aborted) {
					previewItems = [];
					isLoading = false;
				}
			});

		return () => ac.abort();
	});

	function getItemHref(item: SearchItem): string {
		if (item.source === 'my-storage') {
			if (item.type === 'folder') {
				return `/drive/my-storage?folder=${encodeURIComponent(item.id)}`;
			}
			return item.parent_id
				? `/drive/my-storage?folder=${encodeURIComponent(item.parent_id)}&preview=${encodeURIComponent(item.id)}`
				: `/drive/my-storage?preview=${encodeURIComponent(item.id)}`;
		} else if (item.source === 'organization') {
			if (item.type === 'folder') {
				return `/drive/organization?folder=${encodeURIComponent(item.id)}`;
			}
			return item.parent_id
				? `/drive/organization?folder=${encodeURIComponent(item.parent_id)}&preview=${encodeURIComponent(item.id)}`
				: `/drive/organization?preview=${encodeURIComponent(item.id)}`;
		} else {
			// shared
			if (item.type === 'folder') {
				return `/drive/shared?folder=${encodeURIComponent(item.id)}`;
			}
			return item.parent_id
				? `/drive/shared?folder=${encodeURIComponent(item.parent_id)}&preview=${encodeURIComponent(item.id)}`
				: `/drive/shared?preview=${encodeURIComponent(item.id)}`;
		}
	}

	function goToAllResults() {
		if (!trimmedQuery) return;
		isOpen = false;
		selectedIndex = -1;
		inputEl?.blur();
		void goto(`/drive/search?q=${encodeURIComponent(trimmedQuery)}`);
	}

	function selectItem(item: SearchItem) {
		isOpen = false;
		selectedIndex = -1;
		inputEl?.blur();
		void goto(getItemHref(item));
	}

	function handleKeyDown(e: KeyboardEvent) {
		if (!isOpen && e.key === 'ArrowDown' && trimmedQuery) {
			isOpen = true;
			e.preventDefault();
			return;
		}

		if (e.key === 'Escape') {
			isOpen = false;
			selectedIndex = -1;
			e.preventDefault();
			return;
		}

		if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (totalOptions > 0) {
				selectedIndex = (selectedIndex + 1) % totalOptions;
			}
			return;
		}

		if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (totalOptions > 0) {
				selectedIndex = (selectedIndex - 1 + totalOptions) % totalOptions;
			}
			return;
		}

		if (e.key === 'Enter') {
			e.preventDefault();
			if (selectedIndex >= 0 && selectedIndex < previewItems.length) {
				selectItem(previewItems[selectedIndex]);
			} else {
				goToAllResults();
			}
		}
	}

	function clearInput() {
		searchQuery = '';
		debouncedQuery = '';
		previewItems = [];
		isOpen = false;
		selectedIndex = -1;
		inputEl?.focus();
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

	onMount(() => {
		function handleClickOutside(e: MouseEvent) {
			if (containerEl && !containerEl.contains(e.target as Node)) {
				isOpen = false;
				selectedIndex = -1;
			}
		}

		document.addEventListener('mousedown', handleClickOutside);
		return () => {
			document.removeEventListener('mousedown', handleClickOutside);
		};
	});
</script>

<div
	bind:this={containerEl}
	class="relative w-full max-w-sm min-w-0"
	data-testid="global-search-container"
>
	<div class="relative flex items-center">
		<Search
			class="pointer-events-none absolute start-3 top-1/2 size-4 -translate-y-1/2 stroke-2 text-muted"
			aria-hidden="true"
		/>
		<Input
			bind:ref={inputEl}
			type="search"
			class="ps-9 pe-8"
			placeholder={tx('Cari berkas…', 'Search files…')}
			aria-label={tx('Cari berkas', 'Search files')}
			role="combobox"
			aria-expanded={isOpen}
			aria-controls="search-preview-menu"
			aria-autocomplete="list"
			bind:value={searchQuery}
			onfocus={() => {
				if (trimmedQuery && (previewItems.length > 0 || isLoading)) {
					isOpen = true;
				}
			}}
			onkeydown={handleKeyDown}
			data-testid="global-search-input"
		/>
		{#if isLoading}
			<Loader2
				class="pointer-events-none absolute end-3 top-1/2 size-4 -translate-y-1/2 animate-spin text-muted"
				aria-hidden="true"
			/>
		{:else if searchQuery}
			<button
				type="button"
				class="absolute end-2 top-1/2 -translate-y-1/2 rounded-full p-1 text-muted hover:text-foreground focus:outline-none"
				onclick={clearInput}
				aria-label={tx('Bersihkan pencarian', 'Clear search')}
			>
				<X class="size-3.5 stroke-2" />
			</button>
		{/if}
	</div>

	<!-- Live preview dropdown -->
	{#if isOpen && trimmedQuery}
		<div
			id="search-preview-menu"
			role="listbox"
			aria-label={tx('Hasil pratinjau pencarian', 'Search preview results')}
			class="absolute inset-x-0 top-full z-50 mt-1 max-h-96 overflow-hidden rounded-xl border border-subtle bg-surface shadow-xl"
			data-testid="global-search-preview"
		>
			{#if isLoading && previewItems.length === 0}
				<div class="flex items-center justify-center gap-2 px-4 py-6 text-sm text-muted">
					<Loader2 class="size-4 animate-spin text-primary" />
					<span>{tx('Mencari berkas…', 'Searching files…')}</span>
				</div>
			{:else}
				{#if previewItems.length === 0}
					<div class="px-4 py-4 text-center text-sm text-muted">
						{tx('Tidak ada berkas yang cocok', 'No matching files')}
					</div>
				{:else}
					<ul class="max-h-72 divide-y divide-subtle/50 overflow-y-auto">
						{#each previewItems as item, idx (item.id)}
							{@const Icon = getIcon(item)}
							{@const isSelected = selectedIndex === idx}
							<li>
								<button
									type="button"
									role="option"
									aria-selected={isSelected}
									class={`flex w-full items-center gap-3 px-3.5 py-2.5 text-start text-sm transition-colors ${
										isSelected ? 'bg-primary/10 text-primary' : 'text-foreground hover:bg-accent/50'
									}`}
									onclick={() => selectItem(item)}
									onmouseenter={() => (selectedIndex = idx)}
									data-testid="search-result-item"
								>
									<div
										class="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary"
									>
										<Icon class="size-4 stroke-2" />
									</div>

									<div class="min-w-0 flex-1">
										<div class="flex items-center gap-2">
											<span class="truncate font-medium">{item.name}</span>
											{#if item.source === 'my-storage'}
												<span
													class="shrink-0 rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium text-primary"
												>
													My Storage
												</span>
											{:else if item.source === 'organization'}
												<span
													class="shrink-0 rounded bg-violet-500/10 px-1.5 py-0.5 text-[10px] font-medium text-violet-600 dark:text-violet-400"
												>
													{tx('Organisasi', 'Organization')}
												</span>
											{:else}
												<span
													class="shrink-0 rounded bg-emerald-500/10 px-1.5 py-0.5 text-[10px] font-medium text-emerald-600 dark:text-emerald-400"
												>
													{tx('Dibagikan', 'Shared')}
												</span>
											{/if}
										</div>
										<p class="truncate text-xs text-muted">
											{item.location}
										</p>
									</div>
								</button>
							</li>
						{/each}
					</ul>
				{/if}

				<!-- "Lihat semua hasil untuk '...'" option -->
				<div class="border-t border-subtle bg-accent/20 p-1.5">
					<button
						type="button"
						role="option"
						aria-selected={selectedIndex === previewItems.length}
						class={`flex w-full items-center justify-between rounded-lg px-3 py-2 text-start text-xs font-medium transition-colors ${
							selectedIndex === previewItems.length
								? 'bg-primary text-primary-foreground'
								: 'text-primary hover:bg-primary/10'
						}`}
						onclick={goToAllResults}
						onmouseenter={() => (selectedIndex = previewItems.length)}
						data-testid="search-view-all"
					>
						<span class="truncate">
							{tx('Lihat semua hasil untuk', 'View all results for')} “<span class="font-semibold"
								>{trimmedQuery}</span
							>”
						</span>
						<ArrowRight class="size-3.5 shrink-0 stroke-2" />
					</button>
				</div>
			{/if}
		</div>
	{/if}
</div>
