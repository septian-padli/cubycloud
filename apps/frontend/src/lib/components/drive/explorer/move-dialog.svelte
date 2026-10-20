<script lang="ts">
	import type { FileEntry } from '$lib/types/drive.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { isAncestor } from '$lib/mocks/file-tree.js';
	import { cn } from '$lib/utils.js';
	import {
		Folder,
		FolderOpen,
		ChevronRight,
		ArrowLeft,
		Building2,
		HardDrive,
		Search,
		X
	} from '@lucide/svelte';

	let {
		open = $bindable(false),
		entry = null,
		entries = [],
		folders = [],
		currentParentId = null,
		onConfirm
	}: {
		open?: boolean;
		entry?: FileEntry | null;
		entries?: FileEntry[];
		folders?: FileEntry[];
		currentParentId?: string | null;
		onConfirm: (targetFolderId: string | null) => void;
	} = $props();

	let currentFolderId = $state<string | null>(null);
	let searchQuery = $state('');
	let showSearch = $state(false);

	$effect(() => {
		if (open) {
			currentFolderId = null;
			searchQuery = '';
			showSearch = false;
		}
	});

	const targetList = $derived(entries && entries.length > 0 ? entries : (entry ? [entry] : []));
	const effectiveCurrentParentId = $derived(targetList[0]?.parentId ?? currentParentId ?? null);

	const currentLocationName = $derived.by(() => {
		if (!effectiveCurrentParentId) return 'Drive Saya';
		const parentFolder = folders.find((f) => f.id === effectiveCurrentParentId);
		return parentFolder ? parentFolder.name : 'Folder';
	});

	const breadcrumbs = $derived.by(() => {
		if (!currentFolderId) return [{ id: null, name: 'Drive Saya' }];
		const trail: { id: string | null; name: string }[] = [{ id: null, name: 'Drive Saya' }];
		const segments: { id: string; name: string }[] = [];
		let curr = folders.find((f) => f.id === currentFolderId);
		while (curr) {
			segments.unshift({ id: curr.id, name: curr.name });
			curr = curr.parentId ? folders.find((f) => f.id === curr!.parentId) : undefined;
		}
		return [...trail, ...segments];
	});

	const currentLevelFolders = $derived.by(() => {
		const allFoldersOnly = folders.filter((f) => f.kind === 'folder');
		if (searchQuery.trim()) {
			const q = searchQuery.toLowerCase().trim();
			return allFoldersOnly.filter((f) => f.name.toLowerCase().includes(q));
		}
		return allFoldersOnly.filter((f) => (f.parentId ?? null) === currentFolderId);
	});

	function isFolderInvalid(targetFolder: FileEntry): boolean {
		for (const e of targetList) {
			if (e.kind === 'folder') {
				if (targetFolder.id === e.id) return true;
				if (isAncestor(folders, e.id, targetFolder.id)) return true;
			}
		}
		return false;
	}

	function isCurrentLocation(targetFolder: FileEntry): boolean {
		return targetFolder.id === effectiveCurrentParentId;
	}

	const canMoveToCurrentFolder = $derived.by(() => {
		if (targetList.length === 0) return false;
		if (targetList.every((e) => (e.parentId ?? null) === currentFolderId)) return false;
		for (const e of targetList) {
			if (e.kind === 'folder' && currentFolderId) {
				if (currentFolderId === e.id) return false;
				if (isAncestor(folders, e.id, currentFolderId)) return false;
			}
		}
		return true;
	});

	function executeMove(targetId: string | null) {
		onConfirm(targetId);
		open = false;
	}

	function navigateUp() {
		if (!currentFolderId) return;
		const curr = folders.find((f) => f.id === currentFolderId);
		currentFolderId = curr?.parentId ?? null;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content
		class="max-w-[calc(100%-2rem)] sm:max-w-xl w-full p-0 bg-surface text-main rounded-2xl border border-subtle shadow-none overflow-hidden flex flex-col max-h-[85vh] h-[520px]"
		data-testid="move-dialog"
	>
		<!-- Header -->
		<div class="px-6 pt-5 pb-3">
			<div class="flex items-start justify-between gap-4">
				<div class="space-y-1 min-w-0 pr-6">
					<Dialog.Title class="text-xl font-medium tracking-tight text-main">
						Pindahkan
					</Dialog.Title>
					<p class="text-sm text-main/90 font-medium truncate">
						{#if targetList.length > 1}
							{targetList.length} item dipilih
						{:else}
							“{targetList[0]?.name ?? 'Berkas'}”
						{/if}
					</p>
					<div class="flex items-center gap-2 pt-1 text-xs text-muted">
						<span>Lokasi saat ini:</span>
						<span
							class="inline-flex items-center gap-1.5 rounded-full border border-subtle bg-app/80 px-2.5 py-0.5 text-xs font-medium text-main"
						>
							<Folder class="size-3.5 text-primary-accent" />
							<span class="max-w-[200px] truncate">{currentLocationName}</span>
						</span>
					</div>
				</div>
			</div>
		</div>

		<!-- Tabs Bar -->
		<div class="px-6 border-b border-subtle flex items-center justify-between">
			<div class="flex items-center gap-6">
				<button
					type="button"
					class="flex items-center gap-2 pb-2.5 pt-1 text-sm font-medium border-b-2 border-primary text-primary-accent transition-colors"
					data-testid="move-tab-drive-saya"
				>
					<HardDrive class="size-4" />
					Drive Saya
				</button>
				<button
					type="button"
					disabled
					class="flex items-center gap-2 pb-2.5 pt-1 text-sm font-medium border-b-2 border-transparent text-muted/50 cursor-not-allowed opacity-60"
					data-testid="move-tab-organisasi"
					title="Organisasi belum tersedia"
				>
					<Building2 class="size-4" />
					Organisasi
					<span class="text-[10px] rounded bg-muted/15 px-1.5 py-0.2">Segera</span>
				</button>
			</div>

			<div class="flex items-center">
				{#if showSearch}
					<div class="relative flex items-center">
						<Search class="pointer-events-none absolute start-2.5 size-3.5 stroke-2 text-muted" />
						<Input
							type="text"
							placeholder="Cari folder..."
							bind:value={searchQuery}
							class="h-8 w-36 pe-7 ps-8 text-xs sm:w-44"
						/>
						{#if searchQuery}
							<button
								type="button"
								onclick={() => (searchQuery = '')}
								class="absolute end-6 text-muted hover:text-main"
							>
								<X class="size-3" />
							</button>
						{/if}
						<button
							type="button"
							onclick={() => {
								showSearch = false;
								searchQuery = '';
							}}
							class="absolute end-2 text-muted hover:text-main"
						>
							<X class="size-3.5" />
						</button>
					</div>
				{:else}
					<button
						type="button"
						class="p-1.5 text-muted hover:text-main rounded-full hover:bg-app transition-colors"
						onclick={() => (showSearch = true)}
						title="Cari folder"
						aria-label="Cari folder"
					>
						<Search class="size-4" />
					</button>
				{/if}
			</div>
		</div>

		<!-- Breadcrumbs & Navigation Bar inside Drive Saya -->
		{#if currentFolderId && !searchQuery}
			<div class="flex items-center gap-2 px-6 py-2 bg-app/50 border-b border-subtle text-xs">
				<button
					type="button"
					class="p-1 rounded-md hover:bg-surface text-main transition-colors"
					onclick={navigateUp}
					title="Kembali ke folder sebelumnya"
					aria-label="Kembali ke folder sebelumnya"
				>
					<ArrowLeft class="size-4" />
				</button>
				<nav class="flex items-center gap-1 min-w-0 flex-1 overflow-x-auto" aria-label="Breadcrumb">
					{#each breadcrumbs as seg, idx (seg.id ?? 'root')}
						{#if idx > 0}
							<span class="text-muted/60">/</span>
						{/if}
						{#if idx === breadcrumbs.length - 1}
							<span class="font-medium text-main truncate max-w-[150px]">{seg.name}</span>
						{:else}
							<button
								type="button"
								class="text-muted hover:text-main transition-colors truncate max-w-[120px]"
								onclick={() => (currentFolderId = seg.id)}
							>
								{seg.name}
							</button>
						{/if}
					{/each}
				</nav>
			</div>
		{/if}

		<!-- Folder List -->
		<div class="flex-1 overflow-y-auto px-4 py-3 space-y-1">
			{#if currentLevelFolders.length === 0}
				<div class="flex flex-col items-center justify-center py-16 text-center text-muted">
					<FolderOpen class="size-10 mb-2 stroke-1 text-muted/50" />
					<p class="text-sm font-medium text-main/80">Folder ini kosong</p>
					<p class="text-xs text-muted mt-0.5">Tidak ada folder tujuan di lokasi ini</p>
				</div>
			{:else}
				{#each currentLevelFolders as folder (folder.id)}
					{@const invalid = isFolderInvalid(folder)}
					{@const here = isCurrentLocation(folder)}
					<div
						class={cn(
							'flex items-center justify-between px-3 py-2 rounded-xl transition-colors select-none group',
							invalid
								? 'opacity-40 cursor-not-allowed bg-transparent'
								: 'hover:bg-app/70 cursor-pointer'
						)}
						data-testid="move-folder-{folder.id}"
						onclick={() => {
							if (!invalid) currentFolderId = folder.id;
						}}
						role="button"
						tabindex={invalid ? -1 : 0}
						onkeydown={(e) => {
							if (!invalid && (e.key === 'Enter' || e.key === ' ')) {
								e.preventDefault();
								currentFolderId = folder.id;
							}
						}}
					>
						<div class="flex items-center gap-3 min-w-0 flex-1 pr-3">
							<Folder class="size-5 shrink-0 text-primary-accent" />
							<span class="text-sm font-medium text-main truncate">{folder.name}</span>
							{#if here}
								<span
									class="text-[10px] text-muted rounded-full bg-app px-2 py-0.5 border border-subtle shrink-0"
								>
									Lokasi saat ini
								</span>
							{:else if invalid}
								<span
									class="text-[10px] text-muted rounded-full bg-app px-2 py-0.5 border border-subtle shrink-0"
								>
									Tidak valid
								</span>
							{/if}
						</div>

						<div class="flex items-center gap-1 shrink-0">
							{#if !invalid && !here}
								<Button
									size="sm"
									variant="outline"
									class="rounded-full h-7 px-3 text-xs border-primary/30 text-primary hover:bg-primary hover:text-white"
									data-testid="btn-move-to-{folder.id}"
									onclick={(e) => {
										e.stopPropagation();
										executeMove(folder.id);
									}}
								>
									Pindahkan
								</Button>
							{/if}
							<button
								type="button"
								disabled={invalid}
								class="p-1 text-muted group-hover:text-main rounded-md hover:bg-surface disabled:opacity-40 transition-colors"
								title="Buka folder"
								aria-label="Buka folder {folder.name}"
								onclick={(e) => {
									e.stopPropagation();
									if (!invalid) currentFolderId = folder.id;
								}}
							>
								<ChevronRight class="size-4" />
							</button>
						</div>
					</div>
				{/each}
			{/if}
		</div>

		<!-- Footer -->
		<div class="p-4 px-6 border-t border-subtle flex items-center justify-between bg-surface">
			<div class="text-xs text-muted truncate max-w-[200px]">
				{#if currentFolderId}
					{@const activeFolder = folders.find((f) => f.id === currentFolderId)}
					Target: <span class="font-medium text-main">{activeFolder?.name ?? 'Folder'}</span>
				{:else}
					Target: <span class="font-medium text-main">Drive Saya (Root)</span>
				{/if}
			</div>

			<div class="flex items-center gap-2">
				<Button
					type="button"
					variant="secondary"
					data-testid="btn-move-cancel"
					onclick={() => (open = false)}
				>
					Batal
				</Button>
				<Button
					type="button"
					disabled={!canMoveToCurrentFolder}
					data-testid="btn-move-footer"
					onclick={() => executeMove(currentFolderId)}
				>
					Pindahkan
				</Button>
			</div>
		</div>
	</Dialog.Content>
</Dialog.Root>
