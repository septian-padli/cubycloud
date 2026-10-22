<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { beforeNavigate, goto } from '$app/navigation';
	import { adminAPI } from '$lib/api/admin.js';
	import type { StorageNode } from '$lib/types/setup.js';
	import type { SystemSettings, AuditRetentionDays, PlacementStrategy } from '$lib/types/admin.js';
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import AddStorageNodeDialog from '$lib/components/admin/add-storage-node-dialog.svelte';
	import EditStorageNodeDialog from '$lib/components/admin/edit-storage-node-dialog.svelte';
	import StorageNodesGrid from '$lib/components/admin/storage-nodes-grid.svelte';
	import AdminSystemSettings from '$lib/components/admin/admin-system-settings.svelte';
	import { ConfirmDialog } from '$lib/components/ui/confirm-dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Plus } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let nodes = $state<StorageNode[]>([]);
	let settings = $state<SystemSettings>({
		defaultQuota: { mode: 'unlimited', gb: 0 },
		auditRetentionDays: 90,
		workerConcurrency: 2,
		placementStrategy: 'FILL_FIRST',
		googleAuth: { enabled: false, clientId: '', configured: false, redirectUri: '', clientSecret: '' }
	});
	let initialSettings = $state<SystemSettings | null>(null);
	let loading = $state(true);
	let savingSettings = $state(false);

	let showAddNodeDialog = $state(false);
	let showEditNodeDialog = $state(false);
	let nodeToEdit = $state<StorageNode | null>(null);

	// Navigation leave confirmation
	let showLeaveConfirmDialog = $state(false);
	let pendingNavigationUrl = $state<string | null>(null);
	let allowLeave = $state(false);

	onMount(async () => {
		await loadData();
	});

	async function loadData() {
		loading = true;

		// Load system settings
		const settingsResp = await adminAPI.getSettings();
		if (settingsResp.error) {
			toast.error(tx('Gagal memuat pengaturan sistem', 'Failed to load system settings'), {
				description: settingsResp.error
			});
		} else if (settingsResp.data) {
			const loadedSettings: SystemSettings = {
				defaultQuota: { mode: settingsResp.data.default_quota.mode, gb: settingsResp.data.default_quota.gb },
				auditRetentionDays: settingsResp.data.audit_retention_days as AuditRetentionDays,
				workerConcurrency: settingsResp.data.worker_concurrency,
				placementStrategy: (settingsResp.data.placement_strategy as PlacementStrategy) || 'FILL_FIRST',
				googleAuth: {
					enabled: settingsResp.data.google_auth?.enabled ?? false,
					clientId: settingsResp.data.google_auth?.client_id ?? '',
					configured: settingsResp.data.google_auth?.configured ?? false,
					redirectUri: settingsResp.data.google_auth?.redirect_uri ?? '',
					clientSecret: ''
				}
			};
			settings = { ...loadedSettings, defaultQuota: { ...loadedSettings.defaultQuota } };
			initialSettings = { ...loadedSettings, defaultQuota: { ...loadedSettings.defaultQuota } };
		}

		// Load storage nodes
		const nodesResp = await adminAPI.getStorageNodes();
		if (nodesResp.error) {
			toast.error(tx('Gagal memuat storage nodes', 'Failed to load storage nodes'), {
				description: nodesResp.error
			});
		} else if (nodesResp.data) {
			nodes = nodesResp.data.nodes.map((n) => ({
				id: n.id,
				type: (n.type === 'S3' ? 'S3' : 'LOCAL') as 'LOCAL' | 'S3',
				name: n.name,
				path: n.path,
				kind: (n.kind === 'SSD' || n.kind === 'HDD' || n.kind === 'S3' ? n.kind : 'HDD') as
					'SSD' | 'HDD' | 'S3',
				endpoint: n.endpoint,
				bucket: n.bucket,
				totalGb: n.capacity_cap_gb,
				freeGb: Math.max((n.capacity_cap_gb * 1024 ** 3 - n.used_bytes) / 1024 ** 3, 0),
				usedBytes: n.used_bytes,
				capacityCapGb: n.capacity_cap_gb,
				enabled: n.enabled
			}));
		}

		loading = false;
	}

	function handleAddNode() {
		showAddNodeDialog = true;
	}

	function handleEditNode(node: StorageNode) {
		nodeToEdit = node;
		showEditNodeDialog = true;
	}

	function handleNodeUpdated() {
		void loadData();
	}

	const isDirty = $derived.by(() => {
		if (!initialSettings) return false;
		if (settings.defaultQuota.mode !== initialSettings.defaultQuota.mode) return true;
		if (
			settings.defaultQuota.mode === 'limited' &&
			Number(settings.defaultQuota.gb) !== Number(initialSettings.defaultQuota.gb)
		) {
			return true;
		}
		if (Number(settings.auditRetentionDays) !== Number(initialSettings.auditRetentionDays)) {
			return true;
		}
		if (Number(settings.workerConcurrency) !== Number(initialSettings.workerConcurrency)) {
			return true;
		}
		if (settings.placementStrategy !== initialSettings.placementStrategy) return true;
		if (
			settings.googleAuth.enabled !== initialSettings.googleAuth.enabled ||
			settings.googleAuth.clientId !== initialSettings.googleAuth.clientId ||
			settings.googleAuth.clientSecret !== ''
		) return true;
		return false;
	});

	const isQuotaInvalid = $derived(
		settings.defaultQuota.mode === 'limited' &&
			(isNaN(Number(settings.defaultQuota.gb)) || Number(settings.defaultQuota.gb) < 1)
	);

	const isConcurrencyInvalid = $derived(
		isNaN(Number(settings.workerConcurrency)) ||
			Number(settings.workerConcurrency) < 1 ||
			Number(settings.workerConcurrency) > 10
	);

	const isRetentionInvalid = $derived(
		![7, 14, 30, 60, 90].includes(Number(settings.auditRetentionDays))
	);
	const isStrategyInvalid = $derived(
		!['FILL_FIRST', 'MOST_FREE_SPACE', 'ROUND_ROBIN'].includes(settings.placementStrategy)
	);
	const isGoogleAuthInvalid = $derived(
		settings.googleAuth.enabled &&
			(!settings.googleAuth.clientId.trim() || (!settings.googleAuth.configured && !settings.googleAuth.clientSecret))
	);
	const isValid = $derived(
		!isQuotaInvalid && !isConcurrencyInvalid && !isRetentionInvalid && !isStrategyInvalid && !isGoogleAuthInvalid
	);
	const canSave = $derived(isDirty && isValid && !savingSettings);

	async function saveSettings() {
		if (!canSave) return;
		savingSettings = true;
		const resp = await adminAPI.updateSettings({
			default_quota: { mode: settings.defaultQuota.mode, gb: Number(settings.defaultQuota.gb) },
			audit_retention_days: Number(settings.auditRetentionDays),
			worker_concurrency: Number(settings.workerConcurrency),
			placement_strategy: settings.placementStrategy,
			google_auth: {
				enabled: settings.googleAuth.enabled,
				client_id: settings.googleAuth.clientId,
				...(settings.googleAuth.clientSecret ? { client_secret: settings.googleAuth.clientSecret } : {})
			}
		});
		savingSettings = false;

		if (resp.error) {
			toast.error(tx('Gagal menyimpan pengaturan', 'Failed to save settings'), {
				description: resp.error
			});
		} else {
			toast.success(tx('Pengaturan berhasil disimpan', 'Pengaturan berhasil disimpan'));
			if (resp.data) {
				const synced: SystemSettings = {
					defaultQuota: { mode: resp.data.default_quota.mode, gb: resp.data.default_quota.gb },
					auditRetentionDays: resp.data.audit_retention_days as AuditRetentionDays,
					workerConcurrency: resp.data.worker_concurrency,
					placementStrategy: (resp.data.placement_strategy as PlacementStrategy) || 'FILL_FIRST',
					googleAuth: {
						enabled: resp.data.google_auth?.enabled ?? false,
						clientId: resp.data.google_auth?.client_id ?? '',
						configured: resp.data.google_auth?.configured ?? false,
						redirectUri: resp.data.google_auth?.redirect_uri ?? '',
						clientSecret: ''
					}
				};
				settings = { ...synced, defaultQuota: { ...synced.defaultQuota } };
				initialSettings = { ...synced, defaultQuota: { ...synced.defaultQuota } };
			}
		}
	}

	beforeNavigate(({ cancel, to }) => {
		if (isDirty && !allowLeave) {
			cancel();
			pendingNavigationUrl = to?.url.pathname || to?.url.href || null;
			showLeaveConfirmDialog = true;
		}
	});

	async function handleConfirmLeave() {
		allowLeave = true;
		showLeaveConfirmDialog = false;
		if (pendingNavigationUrl) {
			await goto(pendingNavigationUrl);
		}
	}

	function handleCancelLeave() {
		showLeaveConfirmDialog = false;
		pendingNavigationUrl = null;
	}

	$effect(() => {
		if (!browser) return;
		const handleBeforeUnload = (e: BeforeUnloadEvent) => {
			if (isDirty) {
				e.preventDefault();
				e.returnValue = '';
			}
		};
		window.addEventListener('beforeunload', handleBeforeUnload);
		return () => {
			window.removeEventListener('beforeunload', handleBeforeUnload);
		};
	});
</script>

<svelte:head>
	<title>{tx('Pengaturan Admin', 'Admin Settings')} — CubyCloud</title>
	<meta
		name="description"
		content={tx(
			'Kelola node penyimpanan dan pengaturan sistem CubyCloud.',
			'Manage CubyCloud storage nodes and system settings.'
		)}
	/>
</svelte:head>

<div class="flex flex-col gap-8">
	<DrivePageHeader
		title={tx('Pengaturan Admin', 'Admin Settings')}
		description={tx(
			'Kelola node penyimpanan, kuota default, retensi audit, dan backup basis data.',
			'Manage storage nodes, default quotas, audit retention, and database backups.'
		)}
	/>

	{#if loading}
		<div class="flex items-center justify-center py-12">
			<p class="text-sm text-muted">{tx('Memuat data...', 'Loading data...')}</p>
		</div>
	{:else}
		<section class="space-y-4">
			<div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
				<div class="space-y-1">
					<h2 class="text-sm font-semibold text-main" aria-label="Node penyimpanan">{tx('Node penyimpanan', 'Storage nodes')}</h2>
					<p class="text-xs text-muted">
						{tx(
							'Kapasitas fisik, ruang bebas, dan status kritis (&lt;5% bebas).',
							'Physical capacity, free space, and critical status (&lt;5% free).'
						)}
					</p>
				</div>
				<Button
					type="button"
					variant="secondary"
					class="shrink-0 gap-1.5"
					onclick={handleAddNode}
					data-testid="btn-add-storage-node"
				>
					<Plus class="size-3.5 stroke-2" />
					{tx('Tambah Node', 'Add Node')}
				</Button>
			</div>
			<StorageNodesGrid {nodes} onEdit={handleEditNode} />
		</section>

		<section>
			<AdminSystemSettings
				bind:settings
				saving={savingSettings}
				{isDirty}
				{canSave}
				{isQuotaInvalid}
				{isConcurrencyInvalid}
				onSave={saveSettings}
			/>
		</section>
	{/if}
</div>

<AddStorageNodeDialog bind:open={showAddNodeDialog} onAdded={handleNodeUpdated} />
<EditStorageNodeDialog
	bind:open={showEditNodeDialog}
	node={nodeToEdit}
	onUpdated={handleNodeUpdated}
/>

<ConfirmDialog
	bind:open={showLeaveConfirmDialog}
	title="Tinggalkan Halaman?"
	description={tx(
		'Pengaturan yang Anda ubah belum disimpan. Perubahan akan hilang jika Anda meninggalkan halaman ini.',
		'Your changes to the settings haven’t been saved. They will be lost if you leave this page.'
	)}
	confirmText="Tinggalkan"
	cancelText="Tetap di Sini"
	onConfirm={handleConfirmLeave}
	onCancel={handleCancelLeave}
/>
