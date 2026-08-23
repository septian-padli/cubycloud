<script lang="ts">
	import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { toast } from 'svelte-sonner';
	import { adminAPI } from '$lib/api/admin.js';
	import type { StorageNode } from '$lib/types/setup.js';

	let {
		open = $bindable(false),
		node,
		onUpdated
	}: {
		open: boolean;
		node: StorageNode | null;
		onUpdated: () => void;
	} = $props();

	let capacity = $state(0);
	let enabled = $state(true);
	let updating = $state(false);

	// Sync form with node when dialog opens
	$effect(() => {
		if (open && node) {
			capacity = node.capacityCapGb || 0;
			enabled = node.enabled;
		}
	});

	const canUpdate = $derived(capacity >= 1);

	async function handleUpdate() {
		if (!node || !canUpdate) return;

		updating = true;
		const resp = await adminAPI.updateNode(node.id, {
			capacity_cap_gb: capacity,
			enabled
		});
		updating = false;

		if (resp.error) {
			toast.error('Gagal memperbarui node', { description: resp.error });
		} else {
			toast.success('Node berhasil diperbarui');
			open = false;
			onUpdated();
		}
	}

	function handleOpenChange(newOpen: boolean) {
		if (!updating) {
			open = newOpen;
		}
	}
</script>

<Dialog bind:open onOpenChange={handleOpenChange}>
	<DialogContent class="max-w-md">
		<DialogHeader>
			<DialogTitle>Edit Node Penyimpanan</DialogTitle>
			<DialogDescription>
				Ubah kapasitas maksimal dan status node
			</DialogDescription>
		</DialogHeader>

		{#if node}
			<div class="space-y-4">
				<div class="rounded-lg border border-subtle bg-surfaceAlt p-3">
					<div class="text-xs text-muted">Node</div>
					<div class="font-medium text-main">{node.name}</div>
					{#if node.path}
						<div class="text-xs text-muted">{node.path}</div>
					{/if}
				</div>

				<div class="space-y-2">
					<Label for="edit-capacity">Kapasitas Maksimal (GB)</Label>
					<Input
						id="edit-capacity"
						type="number"
						min="1"
						bind:value={capacity}
						disabled={updating}
					/>
					<p class="text-xs text-muted">
						Batas kapasitas yang diizinkan untuk node ini (minimal 1 GB).
					</p>
					{#if node.totalGb && capacity < node.totalGb - node.freeGb}
						<p class="text-xs text-destructive">
							⚠️ Kapasitas tidak boleh lebih kecil dari ruang terpakai ({(node.totalGb - node.freeGb).toFixed(1)} GB)
						</p>
					{/if}
				</div>

				<div class="flex items-center justify-between">
					<div class="space-y-0.5">
						<Label for="edit-enabled">Status Node</Label>
						<p class="text-xs text-muted">
							Node yang dinonaktifkan tidak akan menerima file baru
						</p>
					</div>
					<Switch id="edit-enabled" bind:checked={enabled} disabled={updating} />
				</div>
			</div>
		{/if}

		<DialogFooter>
			<Button variant="outline" onclick={() => (open = false)} disabled={updating}>
				Batal
			</Button>
			<Button onclick={handleUpdate} disabled={!canUpdate || updating}>
				{updating ? 'Memperbarui...' : 'Simpan Perubahan'}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>
