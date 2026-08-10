<script lang="ts">
	import type { AuditRetentionDays, SystemSettings } from '$lib/types/admin.js';
	import { AUDIT_RETENTION_OPTIONS } from '$lib/types/admin.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { Download, Save } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	let {
		settings = $bindable(),
		onSave
	}: {
		settings: SystemSettings;
		onSave: () => void;
	} = $props();

	const unlimited = $derived(settings.defaultQuota.mode === 'unlimited');

	function setUnlimited(checked: boolean) {
		settings.defaultQuota.mode = checked ? 'unlimited' : 'limited';
	}

	function onRetentionChange(event: Event) {
		const value = Number((event.currentTarget as HTMLSelectElement).value);
		settings.auditRetentionDays = value as AuditRetentionDays;
	}

	function downloadBackup() {
		toast.success('Backup DB diunduh (mock)', {
			description: 'File cubycloud.db siap (simulasi).'
		});
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
				<div>
					<p class="text-sm font-medium text-main">Kuota default pengguna</p>
					<p class="text-xs text-muted">Berlaku untuk pengguna baru tanpa override.</p>
				</div>
				<div class="flex items-center gap-2">
					<span class="text-xs text-muted">Tak terbatas</span>
					<Switch
						checked={unlimited}
						aria-label="Kuota tak terbatas"
						onCheckedChange={setUnlimited}
					/>
				</div>
			</div>
			{#if !unlimited}
				<div class="space-y-2">
					<Label for="default-quota-gb" class="text-xs text-muted">Batas (GB)</Label>
					<Input
						id="default-quota-gb"
						type="number"
						min={1}
						class="max-w-xs"
						bind:value={settings.defaultQuota.gb}
					/>
				</div>
			{/if}
		</div>

		<div class="space-y-2">
			<Label for="audit-retention" class="text-sm font-medium text-main">
				Retensi audit log
			</Label>
			<select
				id="audit-retention"
				class="flex h-10 w-full max-w-xs rounded-full border border-subtle bg-surface px-4 text-sm text-main shadow-none outline-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/50"
				value={settings.auditRetentionDays}
				onchange={onRetentionChange}
			>
				{#each AUDIT_RETENTION_OPTIONS as days (days)}
					<option value={days}>{days} hari</option>
				{/each}
			</select>
			<p class="text-xs text-muted">Default: 90 hari.</p>
		</div>

		<div class="space-y-2">
			<Label for="worker-concurrency" class="text-sm font-medium text-main">
				Konkurensi worker
			</Label>
			<Input
				id="worker-concurrency"
				type="number"
				min={1}
				max={16}
				class="max-w-xs"
				bind:value={settings.workerConcurrency}
			/>
			<p class="text-xs text-muted">Jumlah job paralel di antrean SQLite (default: 2).</p>
		</div>

		<div
			class="flex flex-col gap-3 rounded-xl border border-subtle bg-app px-4 py-4 sm:flex-row sm:items-center sm:justify-between"
		>
			<div>
				<p class="text-sm font-medium text-main">Backup basis data</p>
				<p class="text-xs text-muted">Unduh salinan file SQLite (.db) untuk cadangan.</p>
			</div>
			<Button type="button" variant="outline" class="gap-1.5" onclick={downloadBackup}>
				<Download class="size-4 stroke-2" />
				Unduh Backup DB (.db)
			</Button>
		</div>
	</Card.Content>
	<Card.Footer class="justify-end border-t border-subtle">
		<Button type="button" class="gap-1.5" onclick={onSave}>
			<Save class="size-4 stroke-2" />
			Simpan pengaturan
		</Button>
	</Card.Footer>
</Card.Root>
