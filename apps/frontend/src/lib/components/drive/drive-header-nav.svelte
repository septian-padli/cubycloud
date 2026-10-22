<script lang="ts">
	import { locale } from '$lib/locale.svelte.js';
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
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

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
			denialReason =
				target.reason ??
				tx(
					'Anda tidak memiliki izin mengunggah ke folder bersama ini',
					'You do not have permission to upload to this shared folder'
				);
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
			toast.error(
				tx(
					'Sesi pengguna tidak tersedia. Silakan masuk kembali.',
					'User session is unavailable. Please sign in again.'
				)
			);
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
								? tx(
										`“${files[0].name}” berhasil diunggah`,
										`“${files[0].name}” uploaded successfully`
									)
								: tx(
										`${files.length} berkas berhasil diunggah`,
										`${files.length} files uploaded successfully`
									)
						),
					onError: (error) => {
						const message =
							error instanceof Error
								? error.message
								: tx('Gagal mengunggah berkas', 'Failed to upload files');
						if (target.action === 'upload-my-storage' && message.toLowerCase().includes('quota')) {
							toast.error(tx('Kuota My Storage penuh', 'My Storage quota is full'));
						} else {
							toast.error(
								tx(
									'Sebagian atau seluruh berkas gagal diunggah',
									'Some or all files failed to upload'
								),
								{ description: message }
							);
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
		aria-label={tx('Buka menu navigasi', 'Open navigation menu')}
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
				<span class="hidden sm:inline"
					>{tx(`Mengunggah ${uploadProgress}%`, `Uploading ${uploadProgress}%`)}</span
				>
			{:else}
				<Upload class="size-4 stroke-2" />
				<span class="hidden sm:inline">{tx('Unggah', 'Upload')}</span>
			{/if}
		</Button>

		<ThemeToggle />
	</div>
</header>

<HeaderUploadDenialDialog bind:open={denialOpen} reason={denialReason} />
