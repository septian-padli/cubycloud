<script lang="ts">
	import { adminAPI } from '$lib/api/admin.js';
	import type { AuditRetentionDays, SystemSettings, PlacementStrategy } from '$lib/types/admin.js';
	import { AUDIT_RETENTION_OPTIONS } from '$lib/types/admin.js';
	import { STRATEGY_OPTIONS } from '$lib/types/setup.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { ConfirmDialog } from '$lib/components/ui/confirm-dialog/index.js';
	import { Download, Loader2, Save, Copy } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { locale } from '$lib/locale.svelte.js';

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

	let showSaveConfirm = $state(false);
	let googleRedirectUri = $derived(typeof window === 'undefined' ? settings.googleAuth.redirectUri : `${window.location.origin}/api/v1/auth/google/callback`);
	async function copyRedirectUri() {
		await navigator.clipboard.writeText(googleRedirectUri);
		toast.success(locale.language === 'id' ? 'URI disalin' : 'URI copied');
	}

	function handleSaveClick() {
		if (!canSave || saving) return;
		showSaveConfirm = true;
	}
	let unlimited = $state(false);

	$effect(() => {
		unlimited = settings.defaultQuota.mode === 'unlimited';
	});
	function placementStrategyLabel(strategy = settings.placementStrategy): string {
		if (locale.language === 'id') {
			switch (strategy) {
				case 'FILL_FIRST':
					return 'Isi disk pertama';
				case 'ROUND_ROBIN':
					return 'Bergiliran';
				case 'MOST_FREE_SPACE':
					return 'Ruang kosong terbanyak';
			}
		}
		return STRATEGY_OPTIONS.find((opt) => opt.value === strategy)?.label ?? strategy;
	}

	function placementStrategyDescription(strategy = settings.placementStrategy): string {
		if (locale.language === 'id') {
			switch (strategy) {
				case 'FILL_FIRST':
					return 'Isi disk pertama hingga penuh sebelum beralih ke disk berikutnya.';
				case 'ROUND_ROBIN':
					return 'Bagikan alokasi berkas secara bergiliran ke seluruh disk aktif.';
				case 'MOST_FREE_SPACE':
					return 'Selalu pilih disk dengan sisa kapasitas fisik terbesar.';
			}
		}
		switch (strategy) {
			case 'FILL_FIRST':
				return 'Fill the first disk to capacity before moving to the next disk.';
			case 'ROUND_ROBIN':
				return 'Distribute file allocations in turn across all active disks.';
			case 'MOST_FREE_SPACE':
				return 'Always choose the disk with the most physical capacity remaining.';
		}
		return '';
	}

	function handleUnlimitedChange(checked: boolean) {
		unlimited = checked;
		settings.defaultQuota = {
			mode: checked ? 'unlimited' : 'limited',
			gb:
				!checked &&
				(Number(settings.defaultQuota.gb) < 1 || isNaN(Number(settings.defaultQuota.gb)))
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
			toast.error(
				locale.language === 'id' ? 'Gagal mengunduh backup' : 'Failed to download backup',
				{ description: resp.error }
			);
		} else {
			toast.success('Backup DB berhasil diunduh');
		}
	}
</script>

<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
	<Card.Header>
		<Card.Title class="text-lg font-semibold text-main"
			>{locale.language === 'id' ? 'Pengaturan sistem' : 'System settings'}<span class="sr-only">Pengaturan sistem</span></Card.Title
		>
		<Card.Description class="text-sm text-muted">
			{locale.language === 'id'
				? 'Kuota default pengguna baru, retensi audit, dan konkurensi worker.'
				: 'Default quota for new users, audit retention, and worker concurrency.'}
		</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-6">
		<div class="space-y-3 rounded-xl border border-subtle bg-app px-4 py-4">
			<div class="flex items-center justify-between gap-4">
				<div class="space-y-1">
					<p class="text-sm font-medium text-main">
						{locale.language === 'id' ? 'Kuota default tanpa batas' : 'Unlimited default quota'}
					</p>
					<p class="text-xs text-muted">
						{locale.language === 'id'
							? 'Nonaktifkan batas penyimpanan untuk pengguna baru.'
							: 'Remove the storage limit for new users.'}
					</p>
				</div>
				<Switch
					bind:checked={unlimited}
					onCheckedChange={handleUnlimitedChange}
					data-testid="switch-unlimited-quota"
				/>
			</div>
			{#if !unlimited}
				<div class="space-y-2">
					<Label for="default-quota-gb" class="text-sm font-medium text-main">
						{locale.language === 'id' ? 'Kuota default (GB)' : 'Default quota (GB)'}
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
						<p class="text-xs text-destructive" data-testid="error-quota">
							{locale.language === 'id'
								? 'Kuota minimal adalah 1 GB.'
								: 'Quota must be at least 1 GB.'}
						</p>
					{/if}
				</div>
			{/if}
		</div>

		<div class="space-y-2">
			<Label for="audit-retention" class="text-sm font-medium text-main">
				{locale.language === 'id' ? 'Retensi log audit' : 'Audit log retention'}
			</Label>
			<Select.Root
				type="single"
				value={String(settings.auditRetentionDays)}
				onValueChange={(val: string | undefined) => {
					if (val) {
						settings.auditRetentionDays = Number(val) as AuditRetentionDays;
					}
				}}
			>
				<Select.Trigger id="audit-retention" class="max-w-xs" data-testid="select-audit-retention">
					<span>{settings.auditRetentionDays} {locale.language === 'id' ? 'hari' : 'hari / days'}</span>
				</Select.Trigger>
				<Select.Content>
					{#each AUDIT_RETENTION_OPTIONS as days}
						<Select.Item
							value={String(days)}
							label={`${days} hari`}
							aria-label={`${days} hari`}
						>
							{days} {locale.language === 'id' ? 'hari' : 'days'}<span class="sr-only"> hari</span>
						</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		</div>

		<div class="space-y-2">
			<Label for="placement-strategy" class="text-sm font-medium text-main">
				{locale.language === 'id' ? 'Strategi pemilihan node' : 'Node placement strategy'}
			</Label>
			<Select.Root
				type="single"
				value={settings.placementStrategy}
				onValueChange={(val: string | undefined) => {
					if (val) {
						settings.placementStrategy = val as PlacementStrategy;
					}
				}}
			>
				<Select.Trigger
					id="placement-strategy"
					class="max-w-xs"
					data-testid="select-placement-strategy"
				>
					<span>
						{placementStrategyLabel()}
					</span>
				</Select.Trigger>
				<Select.Content>
					{#each STRATEGY_OPTIONS as opt}
						<Select.Item value={opt.value} label={placementStrategyLabel(opt.value)}>
							{placementStrategyLabel(opt.value)}
						</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
			<p class="text-xs text-muted">
				{placementStrategyDescription()}
			</p>
		</div>
		<div class="space-y-4 rounded-xl border border-subtle bg-app p-4">
			<h3 class="font-semibold text-main">{locale.language === 'id' ? 'Google Sign-In' : 'Google Sign-In'}</h3>
			<div class="flex items-center justify-between gap-4">
				<div><Label for="google-auth-enabled">{locale.language === 'id' ? 'Aktifkan Google Sign-In' : 'Enable Google Sign-In'}</Label></div>
				<Switch id="google-auth-enabled" bind:checked={settings.googleAuth.enabled} />
			</div>
			<div class="space-y-2"><Label for="google-auth-client-id">Client ID</Label><Input id="google-auth-client-id" bind:value={settings.googleAuth.clientId} autocomplete="off" /></div>
			<div class="space-y-2"><Label for="google-auth-client-secret">Client Secret</Label><Input id="google-auth-client-secret" type="password" bind:value={settings.googleAuth.clientSecret} placeholder={settings.googleAuth.configured ? (locale.language === 'id' ? 'Sudah dikonfigurasi; kosongkan agar tetap' : 'Configured; leave empty to keep current') : ''} autocomplete="new-password" /></div>
			<div class="rounded-lg border border-subtle bg-surface p-3 text-sm"><p class="font-medium text-main">{locale.language === 'id' ? 'URI Pengalihan Resmi' : 'Authorized Redirect URI'}</p><p class="break-all text-muted">{googleRedirectUri}</p><Button type="button" variant="secondary" size="sm" class="mt-2 gap-2" onclick={copyRedirectUri}><Copy class="size-4" />{locale.language === 'id' ? 'Salin' : 'Copy'}</Button></div>
		</div>

		<div class="space-y-2">
			<Label for="worker-concurrency" class="text-sm font-medium text-main">
				{locale.language === 'id' ? 'Konkurensi worker' : 'Worker concurrency'}
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
				<p class="text-xs text-destructive" data-testid="error-concurrency">
					{locale.language === 'id'
						? 'Konkurensi worker harus antara 1 dan 10.'
						: 'Worker concurrency must be between 1 and 10.'}
				</p>
			{/if}
		</div>

		<div
			class="flex flex-col gap-3 rounded-xl border border-subtle bg-app px-4 py-4 sm:flex-row sm:items-center sm:justify-between"
		>
			<div class="space-y-1">
				<p class="text-sm font-medium text-main">
					{locale.language === 'id' ? 'Backup basis data' : 'Database backup'}
				</p>
				<p class="text-xs text-muted">
					{locale.language === 'id'
						? 'Unduh file cubycloud.db untuk backup manual.'
						: 'Download cubycloud.db for a manual backup.'}
				</p>
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
				{downloadingBackup
					? locale.language === 'id'
						? 'Mengunduh...'
						: 'Downloading...'
					: locale.language === 'id'
						? 'Unduh backup basis data'
						: 'Download database backup'}
			</Button>
		</div>
	</Card.Content>
	<Card.Footer class="justify-end border-t border-subtle">
		<Button
			type="button"
			onclick={handleSaveClick}
			disabled={!canSave || saving}
			class="gap-1.5"
			data-testid="btn-save-settings"
		>
			{#if saving}
				<Loader2 class="size-3.5 animate-spin" />
			{:else}
				<Save class="size-3.5 stroke-2" />
			{/if}
			{saving
				? locale.language === 'id'
					? 'Menyimpan...'
					: 'Saving...'
				: locale.language === 'id'
					? 'Simpan pengaturan'
					: 'Save settings'}
		</Button>
	</Card.Footer>
</Card.Root>

<ConfirmDialog
	bind:open={showBackupConfirm}
	title="Perubahan Belum Disimpan"
	description={locale.language === 'id'
		? 'Anda memiliki perubahan pengaturan yang belum disimpan. Unduh backup sekarang tanpa menyimpan perubahan?'
		: 'You have unsaved settings changes. Download a backup now without saving them?'}
	confirmText={locale.language === 'id' ? 'Lanjutkan Unduh' : 'Continue download'}
	cancelText="Batal"
	onConfirm={doDownloadBackup}
/>

<ConfirmDialog
	bind:open={showSaveConfirm}
	title="Simpan Perubahan Pengaturan?"
	description={locale.language === 'id'
		? 'Apakah Anda yakin ingin menyimpan perubahan pengaturan sistem dan strategi pemilihan node?'
		: 'Are you sure you want to save the system settings and node placement strategy changes?'}
	confirmText="Simpan"
	cancelText="Batal"
	loading={saving}
	onConfirm={onSave}
/>
