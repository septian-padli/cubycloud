<script lang="ts">
	import { onMount } from 'svelte';
	import { adminAPI } from '$lib/api/admin.js';
	import type { StorageNode } from '$lib/types/setup.js';
	import type { SystemSettings, AuditRetentionDays } from '$lib/types/admin.js';
	import AddStorageNodeDialog from '$lib/components/admin/add-storage-node-dialog.svelte';
	import EditStorageNodeDialog from '$lib/components/admin/edit-storage-node-dialog.svelte';
	import StorageNodesGrid from '$lib/components/admin/storage-nodes-grid.svelte';
	import AdminSystemSettings from '$lib/components/admin/admin-system-settings.svelte';
	import DetachNodeDialog from '$lib/components/admin/detach-node-dialog.svelte';
	import RescanNodesDialog from '$lib/components/admin/rescan-nodes-dialog.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { RefreshCw } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	let nodes = $state<StorageNode[]>([]);
	let settings = $state<SystemSettings>({
		defaultQuota: { mode: 'unlimited', gb: 0 },
		auditRetentionDays: 90,
		workerConcurrency: 2
	});
	let loading = $state(true);

	let detachOpen = $state(false);
	let detachNode = $state<StorageNode | null>(null);

	let rescanOpen = $state(false);
	let showAddNodeDialog = $state(false);
	let showEditNodeDialog = $state(false);
	let nodeToEdit = $state<StorageNode | null>(null);
	onMount(async () => {
		await loadData();
	});

	async function loadData() {
		loading = true;

		// Load system settings
		const settingsResp = await adminAPI.getSettings();
		if (settingsResp.error) {
			toast.error('Gagal memuat pengaturan sistem', { description: settingsResp.error });
		} else if (settingsResp.data) {
			settings = {
				defaultQuota: {
					mode: settingsResp.data.default_quota.mode,
					gb: settingsResp.data.default_quota.gb
				},
				auditRetentionDays: settingsResp.data.audit_retention_days as AuditRetentionDays,
				workerConcurrency: settingsResp.data.worker_concurrency
			};
		}

		// Load storage nodes
		const nodesResp = await adminAPI.getStorageNodes();
		if (nodesResp.error) {
			toast.error('Gagal memuat storage nodes', { description: nodesResp.error });
		} else if (nodesResp.data) {
			nodes = nodesResp.data.nodes.map((n) => ({
				id: n.id,
				name: n.name,
				path: n.path,
				kind: (n.kind === 'SSD' || n.kind === 'HDD' ? n.kind : 'HDD') as 'SSD' | 'HDD',
				totalGb: n.total_gb,

	function handleAddNode() {
		showAddNodeDialog = true;
	}

	function handleEditNode(node: StorageNode) {
		nodeToEdit = node;
		showEditNodeDialog = true;
	}

	function handleNodeUpdated() {
		loadData();
	}
				freeGb: n.free_gb,
				capacityCapGb: n.capacity_cap_gb,
				enabled: n.enabled
			}));
		}

		loading = false;
	}

	function openDetach(node: StorageNode) {
		detachNode = node;
		detachOpen = true;
	}

	async function applyDetach(nodeId: string) {
		const resp = await adminAPI.disableNode(nodeId);
		if (resp.error) {
			toast.error('Gagal menonaktifkan node', { description: resp.error });
		} else {
			toast.success('Node berhasil dinonaktifkan');
			await loadData();
		}
	}

	async function applyRescan() {
		const resp = await adminAPI.rescanNodes();
		if (resp.error) {
			toast.error('Gagal memindai ulang nodes', { description: resp.error });
		} else if (resp.data) {
			toast.success('Node berhasil dipindai ulang');
			nodes = resp.data.nodes.map((n) => ({
				id: n.id,
				name: n.name,
				path: n.path,
				kind: (n.kind === 'SSD' || n.kind === 'HDD' ? n.kind : 'HDD') as 'SSD' | 'HDD',
				totalGb: n.total_gb,
				freeGb: n.free_gb,
				capacityCapGb: n.capacity_cap_gb,
				enabled: n.enabled
			}));
		}
	}

	async function saveSettings() {
		const resp = await adminAPI.updateSettings({
			default_quota: {
				mode: settings.defaultQuota.mode,
				gb: settings.defaultQuota.gb
			},
			audit_retention_days: settings.auditRetentionDays,
			worker_concurrency: settings.workerConcurrency
		});

		if (resp.error) {
			toast.error('Gagal menyimpan pengaturan', { description: resp.error });
		} else {
			toast.success('Pengaturan berhasil disimpan');
			if (resp.data) {
				settings = {
					defaultQuota: {
						mode: resp.data.default_quota.mode,
						gb: resp.data.default_quota.gb
					},
					auditRetentionDays: resp.data.audit_retention_days as AuditRetentionDays,
					workerConcurrency: resp.data.worker_concurrency
				};
			}
		}
	}
</script>

<svelte:head>
	<title>Admin Settings — CubyCloud</title>
	<meta name="description" content="Kelola node penyimpanan dan pengaturan sistem CubyCloud." />
</svelte:head>

<div class="flex flex-col gap-8">
	<DrivePageHeader
		title="Admin Settings"
		description="Kelola node penyimpanan, kuota default, retensi audit, dan backup basis data."
	/>

	{#if loading}
		<div class="flex items-center justify-center py-12">
			<p class="text-sm text-muted">Memuat data...</p>
		</div>
	{:else}
		<section class="space-y-4">
			<div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
				<div class="space-y-1">
					<h2 class="text-sm font-semibold text-main">Node penyimpanan</h2>
					<p class="text-xs text-muted">
						Kapasitas fisik, ruang bebas, dan status kritis (&lt;5% bebas).
					</p>
				</div>
				<Button
					type="button"
					variant="secondary"
					class="shrink-0 gap-1.5"
					onclick={() => (rescanOpen = true)}
				>
					<RefreshCw class="size-3.5 stroke-2" />
					Pindai Ulang Node
				</Button>
			</div>
			<StorageNodesGrid 
				nodes={state.nodes} 
				onAdd={handleAddNode}
				onEdit={handleEditNode}
				onDetach={openDetach} 
			/>
		</section>

		<section>
			<AdminSystemSettings bind:settings onSave={saveSettings} />
		</section>
	{/if}
</div>

<DetachNodeDialog bind:open={detachOpen} node={detachNode} onDetach={applyDetach} />
<RescanNodesDialog bind:open={rescanOpen} onRescanned={applyRescan} />

<AddStorageNodeDialog bind:open={showAddNodeDialog} onAdded={handleNodeUpdated} />
<EditStorageNodeDialog bind:open={showEditNodeDialog} node={nodeToEdit} onUpdated={handleNodeUpdated} />
