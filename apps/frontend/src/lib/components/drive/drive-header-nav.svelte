<script lang="ts">
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button/index.js';
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
	import { getDriveChrome } from '$lib/drive-context.svelte.js';
	import GlobalSearchBar from '$lib/components/drive/global-search-bar.svelte';
	import HeaderUploadDenialDialog from '$lib/components/drive/header-upload-denial-dialog.svelte';
	import {
		handleHeaderFilesUpload,
		resolveHeaderUploadTarget,
		type HeaderUploadTarget
	} from '$lib/header-upload.js';
	import { Loader2, Menu, Upload } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	let {
		onMenuClick
	}: {
		onMenuClick: () => void;
	} = $props();

	const chrome = getDriveChrome();
	let fileInput = $state<HTMLInputElement>();
	let isUploading = $state(false);
	let uploadProgress = $state(0);
	let denialOpen = $state(false);
	let denialReason = $state('');
	let pendingTarget = $state<HeaderUploadTarget | null>(null);

	function onUploadClick() {
		if (isUploading) return;
		const context = chrome.sharedUploadContext;
		const target = resolveHeaderUploadTarget(
			page.url.pathname,
			context.folderId,
			context.permission
		);
		if (target.action === 'denied') {
			denialReason = target.reason ?? 'Anda tidak memiliki izin mengunggah ke folder bersama ini';
			denialOpen = true;
			return;
		}
		pendingTarget = target;
		fileInput?.click();
	}

	async function handleFilesSelected(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const files = input.files ? Array.from(input.files) : [];
		input.value = '';
		const target = pendingTarget;
		pendingTarget = null;
		if (!files.length || !target) return;

		if (target.action === 'upload-my-storage' && !chrome.user.id) {
			toast.error('Sesi pengguna tidak tersedia. Silakan masuk kembali.');
			return;
		}

		isUploading = true;
		uploadProgress = 0;
		try {
			await handleHeaderFilesUpload(
				files,
				target,
				{
					onProgress: (progress) => (uploadProgress = progress),
					onSuccess: () =>
						toast.success(
							files.length === 1
								? `“${files[0].name}” berhasil diunggah`
								: `${files.length} berkas berhasil diunggah`
						),
					onError: (error) => {
						const message = error instanceof Error ? error.message : 'Gagal mengunggah berkas';
						if (target.action === 'upload-my-storage' && message.toLowerCase().includes('quota')) {
							toast.error('Kuota My Storage penuh');
						} else {
							toast.error('Sebagian atau seluruh berkas gagal diunggah', { description: message });
						}
					}
				},
				{},
				chrome.user.id
			);
			if (target.action === 'upload-my-storage') {
				await chrome.refreshQuota();
			}
			chrome.notifyStorageChange?.();
		} catch {
			// Failure is reported through the upload callback.
		} finally {
			isUploading = false;
		}
	}
</script>

<header
	class="z-30 flex shrink-0 items-center gap-3 border-b border-subtle bg-surface px-4 py-4 md:px-6"
>
	<Button
		type="button"
		variant="ghost"
		size="icon"
		class="md:hidden"
		aria-label="Buka menu navigasi"
		onclick={onMenuClick}
	>
		<Menu class="size-4 stroke-2" />
	</Button>

	<GlobalSearchBar />

	<div class="ms-auto flex shrink-0 items-center gap-2 sm:gap-3">
		<input
			bind:this={fileInput}
			type="file"
			multiple
			class="hidden"
			data-testid="topbar-upload-input"
			onchange={handleFilesSelected}
		/>

		<Button
			type="button"
			class="gap-1.5"
			disabled={isUploading}
			onclick={onUploadClick}
			data-testid="topbar-upload-button"
		>
			{#if isUploading}
				<Loader2 class="size-4 animate-spin" />
				<span class="hidden sm:inline">Mengunggah {uploadProgress}%</span>
			{:else}
				<Upload class="size-4 stroke-2" />
				<span class="hidden sm:inline">Unggah</span>
			{/if}
		</Button>

		<ThemeToggle />
	</div>
</header>

<HeaderUploadDenialDialog bind:open={denialOpen} reason={denialReason} />
