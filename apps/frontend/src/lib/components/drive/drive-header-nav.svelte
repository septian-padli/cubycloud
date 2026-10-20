<script lang="ts">
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
	import { getDriveChrome } from '$lib/drive-context.svelte.js';
	import { myStorageAPI } from '$lib/api/my-storage.js';
	import { Loader2, Menu, Search, Upload } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	let {
		onMenuClick
	}: {
		onMenuClick: () => void;
	} = $props();

	const chrome = getDriveChrome();
	let search = $state('');
	let fileInput = $state<HTMLInputElement>();
	let isUploading = $state(false);
	const onExplorerPage = $derived(
		page.url.pathname === '/drive/my-storage' || page.url.pathname === '/drive/organization'
	);

	function onUploadClick() {
		if (isUploading) return;
		if (onExplorerPage) {
			chrome.requestUpload();
			return;
		}
		fileInput?.click();
	}

	async function handleFilesSelected(event: Event) {
		const target = event.currentTarget as HTMLInputElement;
		const files = target.files;
		if (!files || files.length === 0) return;

		const fileList = Array.from(files);
		target.value = '';

		isUploading = true;
		let successCount = 0;
		let failedCount = 0;
		let lastError = '';

		for (const file of fileList) {
			try {
				// Upload automatically to root of My Drive (parentId = null)
				await myStorageAPI.upload(file, null, () => {});
				successCount += 1;
				void chrome.refreshQuota();
				chrome.notifyStorageChange?.();
			} catch (err: unknown) {
				failedCount += 1;
				const errorObj = err as { message?: string; status?: number; code?: string };
				const isQuota =
					errorObj?.status === 413 ||
					errorObj?.code === 'USER_QUOTA_EXCEEDED' ||
					errorObj?.code === 'STORAGE_POOL_FULL' ||
					(errorObj?.message && errorObj.message.toLowerCase().includes('kuota'));

				lastError = isQuota ? 'Kuota My Storage penuh' : (errorObj?.message ?? 'Gagal mengunggah berkas');
				if (isQuota) break;
			}
		}

		isUploading = false;
		await chrome.refreshQuota();
		chrome.notifyStorageChange?.();

		if (failedCount === 0 && successCount > 0) {
			if (successCount === 1) {
				toast.success(`“${fileList[0].name}” berhasil diunggah ke My Drive`);
			} else {
				toast.success(`${successCount} berkas berhasil diunggah ke My Drive`);
			}
		} else if (successCount > 0 && failedCount > 0) {
			toast.warning(`${successCount} berhasil, ${failedCount} gagal diunggah`, {
				description: lastError
			});
		} else if (failedCount > 0) {
			toast.error('Gagal mengunggah berkas ke My Drive', {
				description: lastError
			});
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

	<div class="relative min-w-0 w-full max-w-sm">
		<Search
			class="pointer-events-none absolute start-3 top-1/2 size-4 -translate-y-1/2 stroke-2 text-muted"
			aria-hidden="true"
		/>
		<Input
			class="ps-9"
			placeholder="Cari berkas…"
			bind:value={search}
			aria-label="Cari berkas"
		/>
	</div>

	<div class="ms-auto flex shrink-0 items-center gap-2 sm:gap-3">
		{#if !onExplorerPage}
			<input
				bind:this={fileInput}
				type="file"
				multiple
				class="hidden"
				data-testid="topbar-upload-input"
				onchange={handleFilesSelected}
			/>
		{/if}

		<Button
			type="button"
			class="gap-1.5"
			disabled={isUploading}
			onclick={onUploadClick}
			data-testid="topbar-upload-button"
		>
			{#if isUploading}
				<Loader2 class="size-4 animate-spin" />
				<span class="hidden sm:inline">Mengunggah…</span>
			{:else}
				<Upload class="size-4 stroke-2" />
				<span class="hidden sm:inline">Unggah</span>
			{/if}
		</Button>

		<ThemeToggle />
	</div>
</header>
