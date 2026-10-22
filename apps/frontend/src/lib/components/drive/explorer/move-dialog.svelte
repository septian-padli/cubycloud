<script lang="ts">
	import type { FileEntry } from '$lib/types/drive.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Tabs from '$lib/components/ui/tabs/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { isAncestor } from '$lib/mocks/file-tree.js';
	import { cn } from '$lib/utils.js';
	import { locale } from '$lib/locale.svelte.js';
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

	const targetList = $derived(entries && entries.length > 0 ? entries : entry ? [entry] : []);
	const effectiveCurrentParentId = $derived(targetList[0]?.parentId ?? currentParentId ?? null);

	const currentLocationName = $derived.by(() => {
		if (!effectiveCurrentParentId) return locale.language === 'id' ? 'Drive Saya' : 'My Drive';
		const parentFolder = folders.find((f) => f.id === effectiveCurrentParentId);
		return parentFolder ? parentFolder.name : locale.language === 'id' ? 'Folder' : 'Folder';
	});
	const breadcrumbs = $derived.by(() => {
		if (!currentFolderId)
			return [{ id: null, name: locale.language === 'id' ? 'Drive Saya' : 'My Drive' }];
		const trail: { id: string | null; name: string }[] = [
			{ id: null, name: locale.language === 'id' ? 'Drive Saya' : 'My Drive' }
		];
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
		class="flex h-[520px] max-h-[85vh] w-full max-w-[calc(100%-2rem)] flex-col overflow-hidden rounded-2xl border border-subtle bg-surface p-0 text-main shadow-none sm:max-w-xl"
		data-testid="move-dialog"
	>
		<!-- Header -->
		<div class="px-6 pt-5 pb-3">
			<div class="flex items-start justify-between gap-4">
				<div class="min-w-0 space-y-1 pr-6">
					<Dialog.Title class="text-xl font-medium tracking-tight text-main">
						{locale.language === 'id' ? 'Pindahkan' : 'Move'}
					</Dialog.Title>
					<p class="truncate text-sm font-medium text-main/90">
						{#if targetList.length > 1}
							{locale.language === 'id'
								? `${targetList.length} item dipilih`
								: `${targetList.length} items selected`}
						{:else}
							“{targetList[0]?.name ?? (locale.language === 'id' ? 'Berkas' : 'File')}”
						{/if}
					</p>
					<div class="flex items-center gap-2 pt-1 text-xs text-muted">
						<span>{locale.language === 'id' ? 'Lokasi saat ini:' : 'Current location:'}</span>
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
		<Tabs.Root value="drive" class="flex min-h-0 flex-1 flex-col">
			<div class="flex items-center justify-between border-b border-subtle px-6">
				<Tabs.List variant="line" class="gap-6">
					<Tabs.Trigger value="drive" class="gap-2" data-testid="move-tab-drive-saya">
						<HardDrive class="size-4" />
						{locale.language === 'id' ? 'Drive Saya' : 'My Drive'}
					</Tabs.Trigger>
					<Tabs.Trigger
						value="organization"
						disabled
						class="cursor-not-allowed gap-2"
						data-testid="move-tab-organisasi"
						title={locale.language === 'id'
							? 'Organisasi belum tersedia'
							: 'Organization is not available yet'}
					>
						<Building2 class="size-4" />
						{locale.language === 'id' ? 'Organisasi' : 'Organization'}
						<span class="py-0.2 rounded bg-muted/15 px-1.5 text-[10px]"
							>{locale.language === 'id' ? 'Segera' : 'Coming soon'}</span
						>
					</Tabs.Trigger>
				</Tabs.List>

				<div class="flex items-center">
					{#if showSearch}
						<div class="relative flex items-center">
							<Search class="pointer-events-none absolute start-2.5 size-3.5 stroke-2 text-muted" />
							<Input
								type="text"
								placeholder={locale.language === 'id' ? 'Cari folder...' : 'Search folders...'}
								bind:value={searchQuery}
								class="h-8 w-36 ps-8 pe-7 text-xs sm:w-44"
							/>
							{#if searchQuery}
								<button
									type="button"
									onclick={() => (searchQuery = '')}
									class="absolute end-6 text-muted hover:text-main"
									aria-label={locale.language === 'id' ? 'Hapus pencarian' : 'Clear search'}
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
								aria-label={locale.language === 'id' ? 'Tutup pencarian' : 'Close search'}
							>
								<X class="size-3.5" />
							</button>
						</div>
					{:else}
						<button
							type="button"
							class="rounded-full p-1.5 text-muted transition-colors hover:bg-app hover:text-main"
							onclick={() => (showSearch = true)}
							title={locale.language === 'id' ? 'Cari folder' : 'Search folders'}
							aria-label={locale.language === 'id' ? 'Cari folder' : 'Search folders'}
						>
							<Search class="size-4" />
						</button>
					{/if}
				</div>
			</div>

			<Tabs.Content value="drive" class="flex min-h-0 flex-1 flex-col overflow-hidden">
				<!-- Breadcrumbs & Navigation Bar inside Drive Saya -->
				{#if currentFolderId && !searchQuery}
					<div class="flex items-center gap-2 border-b border-subtle bg-app/50 px-6 py-2 text-xs">
						<button
							type="button"
							class="rounded-md p-1 text-main transition-colors hover:bg-surface"
							onclick={navigateUp}
							title={locale.language === 'id'
								? 'Kembali ke folder sebelumnya'
								: 'Go to parent folder'}
							aria-label={locale.language === 'id'
								? 'Kembali ke folder sebelumnya'
								: 'Go to parent folder'}
						>
							<ArrowLeft class="size-4" />
						</button>
						<nav
							class="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto"
							aria-label={locale.language === 'id' ? 'Jejak navigasi' : 'Breadcrumb'}
						>
							{#each breadcrumbs as seg, idx (seg.id ?? 'root')}
								{#if idx > 0}
									<span class="text-muted/60">/</span>
								{/if}
								{#if idx === breadcrumbs.length - 1}
									<span class="max-w-[150px] truncate font-medium text-main">{seg.name}</span>
								{:else}
									<button
										type="button"
										class="max-w-[120px] truncate text-muted transition-colors hover:text-main"
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
				<div class="flex-1 space-y-1 overflow-y-auto px-4 py-3">
					{#if currentLevelFolders.length === 0}
						<div class="flex flex-col items-center justify-center py-16 text-center text-muted">
							<FolderOpen class="mb-2 size-10 stroke-1 text-muted/50" />
							<p class="text-sm font-medium text-main/80">
								{locale.language === 'id' ? 'Folder ini kosong' : 'This folder is empty'}
							</p>
							<p class="mt-0.5 text-xs text-muted">
								{locale.language === 'id'
									? 'Tidak ada folder tujuan di lokasi ini'
									: 'There are no destination folders here'}
							</p>
						</div>
					{:else}
						{#each currentLevelFolders as folder (folder.id)}
							{@const invalid = isFolderInvalid(folder)}
							{@const here = isCurrentLocation(folder)}
							<div
								class={cn(
									'group flex items-center justify-between rounded-xl px-3 py-2 transition-colors select-none',
									invalid
										? 'cursor-not-allowed bg-transparent opacity-40'
										: 'cursor-pointer hover:bg-app/70'
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
								<div class="flex min-w-0 flex-1 items-center gap-3 pr-3">
									<Folder class="size-5 shrink-0 text-primary-accent" />
									<span class="truncate text-sm font-medium text-main">{folder.name}</span>
									{#if here}
										<span
											class="shrink-0 rounded-full border border-subtle bg-app px-2 py-0.5 text-[10px] text-muted"
										>
											{locale.language === 'id' ? 'Lokasi saat ini' : 'Current location'}
										</span>
									{:else if invalid}
										<span
											class="shrink-0 rounded-full border border-subtle bg-app px-2 py-0.5 text-[10px] text-muted"
										>
											{locale.language === 'id' ? 'Tidak valid' : 'Unavailable'}
										</span>
									{/if}
								</div>

								<div class="flex shrink-0 items-center gap-1">
									{#if !invalid && !here}
										<Button
											size="sm"
											variant="outline"
											class="h-7 rounded-full border-primary/30 px-3 text-xs text-primary hover:bg-primary hover:text-white"
											data-testid="btn-move-to-{folder.id}"
											onclick={(e) => {
												e.stopPropagation();
												executeMove(folder.id);
											}}
										>
											{locale.language === 'id' ? 'Pindahkan' : 'Move'}
										</Button>
									{/if}
									<button
										type="button"
										disabled={invalid}
										class="rounded-md p-1 text-muted transition-colors group-hover:text-main hover:bg-surface disabled:opacity-40"
										title={locale.language === 'id' ? 'Buka folder' : 'Open folder'}
										aria-label={locale.language === 'id'
											? `Buka folder ${folder.name}`
											: `Open folder ${folder.name}`}
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
				<div class="flex items-center justify-between border-t border-subtle bg-surface p-4 px-6">
					<div class="max-w-[200px] truncate text-xs text-muted">
						{#if currentFolderId}
							{@const activeFolder = folders.find((f) => f.id === currentFolderId)}
							{locale.language === 'id' ? 'Tujuan:' : 'Destination:'}
							<span class="font-medium text-main"
								>{activeFolder?.name ?? (locale.language === 'id' ? 'Folder' : 'Folder')}</span
							>
						{:else}
							{locale.language === 'id' ? 'Tujuan:' : 'Destination:'}
							<span class="font-medium text-main"
								>{locale.language === 'id' ? 'Drive Saya (Root)' : 'My Drive (Root)'}</span
							>
						{/if}
					</div>

					<div class="flex items-center gap-2">
						<Button
							type="button"
							variant="secondary"
							data-testid="btn-move-cancel"
							onclick={() => (open = false)}
						>
							{locale.language === 'id' ? 'Batal' : 'Cancel'}
						</Button>
						<Button
							type="button"
							disabled={!canMoveToCurrentFolder}
							data-testid="btn-move-footer"
							onclick={() => executeMove(currentFolderId)}
						>
							{locale.language === 'id' ? 'Pindahkan' : 'Move'}
						</Button>
					</div>
				</div>
			</Tabs.Content>
		</Tabs.Root>
	</Dialog.Content>
</Dialog.Root>
