<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import type { FileEntry } from '$lib/types/drive.js';
	import {
		findSharedEntry,
		getShareChildren,
		isDownloadZipEnabled,
		isShareExpired
	} from '$lib/mocks/share-links.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import { driveView } from '$lib/drive-view.svelte.js';
	import { isMockLoggedIn, setMockLoggedIn } from '$lib/mocks/auth-session.js';
	import ShareAppShell from '$lib/components/share/share-app-shell.svelte';
	import ShareStatusView from '$lib/components/share/share-status-view.svelte';
	import SharePasswordGate from '$lib/components/share/share-password-gate.svelte';
	import ShareRestrictedGate from '$lib/components/share/share-restricted-gate.svelte';
	import ShareGuestExplorer from '$lib/components/share/share-guest-explorer.svelte';
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import DriveViewToggle from '$lib/components/drive/drive-view-toggle.svelte';
	import FilePreviewDialog from '$lib/components/drive/explorer/file-preview-dialog.svelte';
	import FileKindIcon from '$lib/components/drive/explorer/file-kind-icon.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Download, Archive } from '@lucide/svelte';
	import type { PageData } from './$types.js';

	let { data }: { data: PageData } = $props();

	const passwordKey = $derived(`cubycloud-share-unlock:${data.token}`);

	let loggedIn = $state(false);
	let unlocked = $state(false);
	let folderId = $state<string | null>(null);

	onMount(() => {
		driveView.init();
		loggedIn = isMockLoggedIn();
		unlocked = sessionStorage.getItem(passwordKey) === '1';
		const q = page.url.searchParams.get('loggedIn');
		if (q === '1') {
			setMockLoggedIn(true);
			loggedIn = true;
			const next = new URL(page.url);
			next.searchParams.delete('loggedIn');
			void goto(`${next.pathname}${next.search}`, { replaceState: true, keepFocus: true, noScroll: true });
		}
	});

	const link = $derived(data.link);
	const expired = $derived(link ? isShareExpired(link) : false);
	const rootEntry = $derived(link ? (findSharedEntry(link.entryId) ?? null) : null);
	const needsLogin = $derived(!!link && !expired && link.access === 'restricted' && !loggedIn);
	const needsPassword = $derived(
		!!link && !expired && !needsLogin && !!link.password && !unlocked
	);

	const currentFolderId = $derived(
		folderId ?? (rootEntry?.kind === 'folder' ? rootEntry.id : null)
	);
	const folderItems = $derived(
		rootEntry?.kind === 'folder' ? getShareChildren(currentFolderId ?? rootEntry.id) : []
	);

	const previewId = $derived(page.url.searchParams.get('preview'));
	const previewEntry = $derived.by(() => {
		if (!previewId) return null;
		if (rootEntry && rootEntry.id === previewId) return rootEntry;
		return folderItems.find((e) => e.id === previewId) ?? findSharedEntry(previewId) ?? null;
	});
	const previewOpen = $derived(!!previewEntry);
	const zipEnabled = isDownloadZipEnabled();

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

	function onUnlock(password: string): boolean {
		if (!link?.password) return true;
		if (password !== link.password) return false;
		sessionStorage.setItem(passwordKey, '1');
		unlocked = true;
		return true;
	}

	function onDownload(entry: FileEntry) {
		toast.info(`Mengunduh “${entry.name}” (mock)`, {
			description: formatSize(entry.sizeBytes)
		});
	}

	function onDownloadZip() {
		if (!zipEnabled) {
			toast.info('Add-on Download as ZIP nonaktif.');
			return;
		}
		toast.info('Mengunduh ZIP (mock)', { description: rootEntry?.name });
	}

	function onOpen(entry: FileEntry) {
		if (entry.kind === 'folder') {
			folderId = entry.id;
			syncPreview(null);
			return;
		}
		syncPreview(entry.id);
	}

	function goUp() {
		if (!rootEntry || rootEntry.kind !== 'folder' || !currentFolderId) return;
		if (currentFolderId === rootEntry.id) return;
		const current = findSharedEntry(currentFolderId);
		const parentId = current?.parentId ?? null;
		folderId = parentId && parentId !== rootEntry.id ? parentId : rootEntry.id;
	}

	const atShareRoot = $derived(!rootEntry || rootEntry.kind !== 'folder' || currentFolderId === rootEntry.id);

	const loginHref = $derived(`/login?redirect=/s/${data.token}`);
</script>

<svelte:head>
	<title>
		{link ? `${link.entryName} — Dibagikan` : 'Tautan tidak ditemukan'} — CubyCloud
	</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<ShareAppShell {loggedIn}>
	{#if !link}
		<ShareStatusView
			title="Tautan tidak ditemukan"
			description="Tautan ini tidak valid atau sudah dihapus."
		/>
	{:else if expired}
		<ShareStatusView
			title="Tautan kedaluwarsa"
			description="Tautan berbagi ini sudah melewati tanggal kedaluwarsa."
		/>
	{:else if needsLogin}
		<ShareRestrictedGate {loginHref} />
	{:else if needsPassword}
		<SharePasswordGate {onUnlock} />
	{:else if rootEntry}
		<div class="flex flex-col gap-6">
			<DrivePageHeader
				title={rootEntry.name}
				eyebrow="Dibagikan oleh {link.ownerName}"
				description={rootEntry.kind === 'folder'
					? 'Folder dibagikan — pratinjau dan unduh (hanya lihat).'
					: 'Berkas dibagikan — pratinjau dan unduh (hanya lihat).'}
			>
				{#snippet actions()}
					{#if rootEntry.kind === 'folder'}
						<DriveViewToggle />
					{/if}
					{#if zipEnabled}
						<Button type="button" variant="secondary" onclick={onDownloadZip}>
							<Archive class="size-4 stroke-2" />
							Unduh ZIP
						</Button>
					{/if}
					{#if rootEntry.kind !== 'folder'}
						<Button type="button" onclick={() => onDownload(rootEntry)}>
							<Download class="size-4 stroke-2" />
							Unduh
						</Button>
					{/if}
				{/snippet}
			</DrivePageHeader>

			{#if rootEntry.kind === 'folder'}
				{#if !atShareRoot}
					<button
						type="button"
						class="self-start text-sm font-medium text-primary hover:underline"
						onclick={goUp}
					>
						← Kembali
					</button>
				{/if}
				<ShareGuestExplorer items={folderItems} {onOpen} />
			{:else}
				<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
					<Card.Content class="flex flex-col gap-4 sm:flex-row sm:items-center">
						<div
							class="flex size-16 items-center justify-center rounded-xl bg-primary/10 text-primary"
						>
							<FileKindIcon kind={rootEntry.kind} />
						</div>
						<div class="min-w-0 flex-1">
							<p class="truncate text-sm font-medium text-main">{rootEntry.name}</p>
							<p class="text-xs text-muted">
								{formatSize(rootEntry.sizeBytes)} · {rootEntry.modifiedAt}
							</p>
						</div>
						<Button type="button" variant="secondary" onclick={() => syncPreview(rootEntry.id)}>
							Pratinjau
						</Button>
					</Card.Content>
				</Card.Root>
			{/if}
		</div>
	{:else}
		<ShareStatusView
			title="Berkas tidak tersedia"
			description="Berkas pada tautan ini tidak dapat dimuat."
		/>
	{/if}
</ShareAppShell>

<FilePreviewDialog
	open={previewOpen}
	entry={previewEntry}
	onDownload={onDownload}
	onClose={() => syncPreview(null)}
/>
