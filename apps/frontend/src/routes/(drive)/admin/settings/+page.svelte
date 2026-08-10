<script lang="ts">
	// ponytail: add +layout.server role check when API exists
	import type { StorageNode } from '$lib/types/setup.js';
	import {
		cloneMockAdminNodes,
		cloneMockSystemSettings
	} from '$lib/mocks/admin-settings.js';
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import StorageNodesGrid from '$lib/components/admin/storage-nodes-grid.svelte';
	import AdminSystemSettings from '$lib/components/admin/admin-system-settings.svelte';
	import DetachNodeDialog from '$lib/components/admin/detach-node-dialog.svelte';
	import RescanNodesDialog from '$lib/components/admin/rescan-nodes-dialog.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { RefreshCw } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	let nodes = $state(cloneMockAdminNodes());
	let settings = $state(cloneMockSystemSettings());

	let detachOpen = $state(false);
	let detachNode = $state<StorageNode | null>(null);
	let rescanOpen = $state(false);

	function openDetach(node: StorageNode) {
		detachNode = node;
		detachOpen = true;
	}

	function applyDetach(nodeId: string) {
		nodes = nodes.map((n) => (n.id === nodeId ? { ...n, enabled: false } : n));
	}

	function applyRescan() {
		nodes = cloneMockAdminNodes();
	}

	function saveSettings() {
		toast.success('Pengaturan disimpan');
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
				variant="outline"
				size="sm"
				class="shrink-0 gap-1.5"
				onclick={() => (rescanOpen = true)}
			>
				<RefreshCw class="size-3.5 stroke-2" />
				Pindai Ulang Node
			</Button>
		</div>
		<StorageNodesGrid {nodes} onDetach={openDetach} />
	</section>

	<section>
		<AdminSystemSettings bind:settings onSave={saveSettings} />
	</section>
</div>

<DetachNodeDialog bind:open={detachOpen} node={detachNode} onDetach={applyDetach} />
<RescanNodesDialog bind:open={rescanOpen} onRescanned={applyRescan} />
