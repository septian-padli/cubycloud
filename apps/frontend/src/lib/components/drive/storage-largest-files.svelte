<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import type { LargestFileItem } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import { driveView } from '$lib/drive-view.svelte.js';
	import { myStorageAPI } from '$lib/api/my-storage.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import LargestFileRow from './largest-file-row.svelte';
	import FilePreviewDialog from './explorer/file-preview-dialog.svelte';
	import CleanupAdvisorDialog from './cleanup-advisor-dialog.svelte';
	import DriveFileGrid from './drive-file-grid.svelte';
	import FileGridCard from './file-grid-card.svelte';
	import DriveViewToggle from './drive-view-toggle.svelte';
	import { Download, Eye, Loader2, MoreVertical, Sparkles, Trash2 } from '@lucide/svelte';

	let {
		files: initialFiles,
		trashCount = 0,
		fetchFromApi = true
	}: {
		files?: LargestFileItem[];
		trashCount?: number;
		fetchFromApi?: boolean;
	} = $props();
	let files = $state<LargestFileItem[]>([]);
	let loading = $state(true);
	let loadingMore = $state(false);
	let hasMore = $state(false);
	let nextOffset = $state(0);
	let selectedId = $state<string | null>(null);
	let cleanupOpen = $state(false);
	let sentinelEl = $state<HTMLDivElement | null>(null);

	const previewId = $derived(page.url.searchParams.get('preview'));
	const previewEntry = $derived(previewId ? (files.find((e) => e.id === previewId) ?? null) : null);
	const previewOpen = $derived(!!previewEntry);

	async function loadInitial() {
		if (!fetchFromApi && initialFiles) {
			files = initialFiles;
			loading = false;
			return;
		}
		try {
			loading = true;
			const res = await myStorageAPI.largestFiles(20, 0);
			files = res.items.map((item) => myStorageAPI.toLargestFileItem(item));
			hasMore = res.has_more;
			nextOffset = res.next_offset;
		} catch (error) {
			toast.error(error instanceof Error ? error.message : 'Gagal memuat berkas terbesar');
		} finally {
			loading = false;
		}
	}

	async function loadMore() {
		if (loadingMore || !hasMore) return;
		try {
			loadingMore = true;
			const res = await myStorageAPI.largestFiles(20, nextOffset);
			const nextItems = res.items.map((item) => myStorageAPI.toLargestFileItem(item));
			files = [...files, ...nextItems];
			hasMore = res.has_more;
			nextOffset = res.next_offset;
		} catch (error) {
			toast.error(error instanceof Error ? error.message : 'Gagal memuat halaman berikutnya');
		} finally {
			loadingMore = false;
		}
	}

	onMount(() => {
		void loadInitial();
	});

	$effect(() => {
		if (!sentinelEl || !hasMore) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0]?.isIntersecting) {
					void loadMore();
				}
			},
			{ rootMargin: '200px' }
		);
		observer.observe(sentinelEl);
		return () => observer.disconnect();
	});

	function syncPreview(id: string | null) {
		const params = new URLSearchParams(page.url.searchParams);
		if (id) params.set('preview', id);
		else params.delete('preview');
		const qs = params.toString();
		void goto(`${page.url.pathname}${qs ? `?${qs}` : ''}`, {
			replaceState: true,
			keepFocus: true,
			noScroll: true
		});
	}

	function onDownload(entry: LargestFileItem) {
		const link = document.createElement('a');
		link.href = myStorageAPI.downloadUrl(entry.id);
		link.download = entry.name;
		document.body.appendChild(link);
		link.click();
		link.remove();
		toast.info(`Mengunduh “${entry.name}”`, {
			description: formatSize(entry.sizeBytes)
		});
	}
</script>

<section class="space-y-3">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<h2 class="text-sm font-semibold text-main">Berkas terbesar</h2>
		<div class="flex flex-wrap items-center gap-2">
			<Button
				type="button"
				variant="secondary"
				class="gap-1.5"
				onclick={() => (cleanupOpen = true)}
			>
				<Sparkles class="size-4 stroke-2" />
				Saran Pembersihan
			</Button>
			<Button type="button" variant="secondary" class="gap-1.5" href="/drive/trash">
				<Trash2 class="size-4 stroke-2" />
				Kelola Sampah
			</Button>
			<DriveViewToggle />
		</div>
	</div>

	{#if loading && files.length === 0}
		<div
			class="flex items-center justify-center rounded-xl border border-subtle bg-surface px-4 py-16 text-muted"
			data-testid="loading-largest-files"
		>
			<Loader2 class="size-6 animate-spin text-primary" />
			<span class="ms-2 text-sm font-medium">Memuat berkas terbesar...</span>
		</div>
	{:else if files.length === 0}
		<Card.Root
			class="rounded-xl border-subtle bg-surface shadow-none"
			data-testid="empty-largest-files"
		>
			<Card.Content class="p-8 text-center text-muted">
				<p class="text-sm font-medium text-main">Tidak ada berkas di My Storage</p>
				<p class="mt-1 text-xs text-muted">
					Berkas yang Anda simpan di My Storage akan muncul di sini.
				</p>
			</Card.Content>
		</Card.Root>
	{:else if driveView.mode === 'grid'}
		<DriveFileGrid label="Berkas terbesar">
			{#each files as entry (entry.id)}
				<FileGridCard
					kind={entry.kind}
					name={entry.name}
					subtitle="{formatSize(entry.sizeBytes)} · {entry.location} · {entry.modifiedAt}"
					selected={selectedId === entry.id}
					onSelect={() => (selectedId = entry.id)}
					onOpen={() => syncPreview(entry.id)}
				>
					{#snippet menu()}
						<DropdownMenu.Root>
							<DropdownMenu.Trigger>
								{#snippet child({ props })}
									<Button
										{...props}
										type="button"
										variant="ghost"
										size="icon"
										class="bg-surface text-muted hover:bg-primary hover:text-white aria-expanded:bg-primary aria-expanded:text-white"
										aria-label="Aksi berkas"
										onclick={(e: MouseEvent) => e.stopPropagation()}
									>
										<MoreVertical class="size-4 stroke-2" />
									</Button>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="end" class="min-w-40">
								<DropdownMenu.Item class="gap-2" onclick={() => syncPreview(entry.id)}>
									<Eye class="size-4 stroke-2" />
									Pratinjau
								</DropdownMenu.Item>
								<DropdownMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
									<Download class="size-4 stroke-2" />
									Unduh
								</DropdownMenu.Item>
							</DropdownMenu.Content>
						</DropdownMenu.Root>
					{/snippet}
				</FileGridCard>
			{/each}
		</DriveFileGrid>
	{:else}
		<Card.Root class="rounded-xl border-subtle bg-surface py-0 shadow-none">
			<Card.Content class="overflow-hidden p-0" role="listbox" aria-label="Berkas terbesar">
				{#each files as entry (entry.id)}
					<LargestFileRow
						{entry}
						selected={selectedId === entry.id}
						onSelect={(e) => (selectedId = e.id)}
						onPreview={(e) => syncPreview(e.id)}
						{onDownload}
					/>
				{/each}
			</Card.Content>
		</Card.Root>
	{/if}

	{#if hasMore}
		<div
			bind:this={sentinelEl}
			class="flex items-center justify-center py-4"
			data-testid="infinite-scroll-sentinel"
		>
			{#if loadingMore}
				<div class="flex items-center gap-2 text-xs text-muted">
					<Loader2 class="size-4 animate-spin text-primary" />
					<span>Memuat lebih banyak berkas...</span>
				</div>
			{:else}
				<Button
					type="button"
					variant="ghost"
					size="sm"
					class="text-xs text-muted hover:text-main"
					onclick={() => void loadMore()}
				>
					Muat lebih banyak
				</Button>
			{/if}
		</div>
	{/if}
</section>

<FilePreviewDialog
	open={previewOpen}
	entry={previewEntry
		? {
				...previewEntry,
				parentId: null
			}
		: null}
	source="my-storage"
	onDownload={(e) => onDownload(e as unknown as LargestFileItem)}
	onClose={() => syncPreview(null)}
/>
<CleanupAdvisorDialog
	bind:open={cleanupOpen}
	suggestions={files}
	{trashCount}
	onGoTrash={() => {
		cleanupOpen = false;
		void goto('/drive/trash');
	}}
/>
