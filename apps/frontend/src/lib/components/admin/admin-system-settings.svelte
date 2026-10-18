<script lang="ts">
	import { adminAPI } from '$lib/api/admin.js';
	import type { AuditRetentionDays, SystemSettings } from '$lib/types/admin.js';
	import { AUDIT_RETENTION_OPTIONS } from '$lib/types/admin.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { ConfirmDialog } from '$lib/components/ui/confirm-dialog/index.js';
	import { Download, Loader2, Save } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	let {
		settings = $bindable(),
		saving = false,
		isDirty = false,
		canSave = false,
		isQuotaInvalid = false,
		isConcurrencyInvalid = false,
		onSave
	}: {
		settings: SystemSettings;
		saving?: boolean;
		isDirty?: boolean;
		canSave?: boolean;
		isQuotaInvalid?: boolean;
		isConcurrencyInvalid?: boolean;
		onSave: () => void | Promise<void>;
	} = $props();

	let downloadingBackup = $state(false);
	let showBackupConfirm = $state(false);

	let unlimited = $state(false);

	$effect(() => {
		unlimited = settings.defaultQuota.mode === 'unlimited';
	});

	function handleUnlimitedChange(checked: boolean) {
		unlimited = checked;
		settings.defaultQuota = {
			mode: checked ? 'unlimited' : 'limited',
			gb: !checked && (Number(settings.defaultQuota.gb) < 1 || isNaN(Number(settings.defaultQuota.gb)))
				? 10
				: Number(settings.defaultQuota.gb)
		};
	}

	function onRetentionChange(event: Event) {
		const value = Number((event.currentTarget as HTMLSelectElement).value);
		settings.auditRetentionDays = value as AuditRetentionDays;
	}

	function handleBackupClick() {
		if (isDirty) {
			showBackupConfirm = true;
		} else {
			void doDownloadBackup();
		}
	}

	async function doDownloadBackup() {
		downloadingBackup = true;
		const resp = await adminAPI.downloadBackup();
		downloadingBackup = false;

		if (resp.error) {
			toast.error('Gagal mengunduh backup', { description: resp.error });
		} else {
			toast.success('Backup DB berhasil diunduh');
		}
	}
</script>

<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
	<Card.Header>
		<Card.Title class="text-lg font-semibold text-main">Pengaturan sistem</Card.Title>
		<Card.Description class="text-sm text-muted">
			Kuota default pengguna baru, retensi audit, dan konkurensi worker.
		</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-6">
		<div class="space-y-3 rounded-xl border border-subtle bg-app px-4 py-4">
			<div class="flex items-center justify-between gap-4">
				<div class="space-y-1">
					<p class="text-sm font-medium text-main">Kuota default unlimited</p>
					<p class="text-xs text-muted">
						Nonaktifkan batas penyimpanan untuk pengguna baru.
					</p>
				</div>
				<Switch bind:checked={unlimited} onCheckedChange={handleUnlimitedChange} data-testid="switch-unlimited-quota" />
			</div>
			{#if !unlimited}
				<div class="space-y-2">
					<Label for="default-quota-gb" class="text-sm font-medium text-main">
						Kuota default (GB)
					</Label>
					<Input
						id="default-quota-gb"
						type="number"
						min="1"
						bind:value={settings.defaultQuota.gb}
						class="max-w-xs"
						data-testid="input-default-quota"
					/>
					{#if isQuotaInvalid}
						<p class="text-xs text-destructive" data-testid="error-quota">Kuota minimal adalah 1 GB.</p>
					{/if}
				</div>
			{/if}
		</div>

		<div class="space-y-2">
			<Label for="audit-retention" class="text-sm font-medium text-main">
				Retensi log audit
			</Label>
			<select
				id="audit-retention"
				value={settings.auditRetentionDays}
				onchange={onRetentionChange}
				class="flex h-10 w-full max-w-xs rounded-lg border border-subtle bg-app px-3 py-2 text-sm text-main ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
				data-testid="select-audit-retention"
			>
				{#each AUDIT_RETENTION_OPTIONS as days}
					<option value={days}>{days} hari</option>
				{/each}
			</select>
		</div>

		<div class="space-y-2">
			<Label for="worker-concurrency" class="text-sm font-medium text-main">
				Konkurensi worker
			</Label>
			<Input
				id="worker-concurrency"
				type="number"
				min="1"
				max="10"
				bind:value={settings.workerConcurrency}
				class="max-w-xs"
				data-testid="input-worker-concurrency"
			/>
			{#if isConcurrencyInvalid}
				<p class="text-xs text-destructive" data-testid="error-concurrency">Konkurensi worker harus antara 1 dan 10.</p>
			{/if}
		</div>

		<div
			class="flex flex-col gap-3 rounded-xl border border-subtle bg-app px-4 py-4 sm:flex-row sm:items-center sm:justify-between"
		>
			<div class="space-y-1">
				<p class="text-sm font-medium text-main">Backup basis data</p>
				<p class="text-xs text-muted">Unduh file cubycloud.db untuk backup manual.</p>
			</div>
			<Button
				type="button"
				variant="secondary"
				class="gap-1.5"
				onclick={handleBackupClick}
				disabled={downloadingBackup}
				data-testid="btn-download-backup"
			>
				{#if downloadingBackup}
					<Loader2 class="size-3.5 animate-spin" />
				{:else}
					<Download class="size-3.5 stroke-2" />
				{/if}
				{downloadingBackup ? 'Mengunduh...' : 'Unduh Backup DB'}
			</Button>
		</div>
	</Card.Content>
	<Card.Footer class="justify-end border-t border-subtle">
		<Button
			type="button"
			onclick={onSave}
			disabled={!canSave || saving}
			class="gap-1.5"
			data-testid="btn-save-settings"
		>
			{#if saving}
				<Loader2 class="size-3.5 animate-spin" />
			{:else}
				<Save class="size-3.5 stroke-2" />
			{/if}
			{saving ? 'Menyimpan...' : 'Simpan Pengaturan'}
		</Button>
	</Card.Footer>
</Card.Root>

<ConfirmDialog
	bind:open={showBackupConfirm}
	title="Perubahan Belum Disimpan"
	description="Anda memiliki perubahan pengaturan yang belum disimpan. Unduh backup sekarang tanpa menyimpan perubahan?"
	confirmText="Lanjutkan Unduh"
	cancelText="Batal"
	onConfirm={doDownloadBackup}
/>
