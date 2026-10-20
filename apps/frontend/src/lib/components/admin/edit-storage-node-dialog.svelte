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

	let name = $state('');
	let nameError = $state('');
	let capacity = $state(0);
	let enabled = $state(true);
	let updating = $state(false);

	// Sync form with node when dialog opens
	$effect(() => {
		if (open && node) {
			name = node.name;
			nameError = '';
			capacity = node.capacityCapGb || 0;
			enabled = node.enabled;
		}
	});

	const usedGb = $derived(node ? node.usedBytes / 1024 ** 3 : 0);
	const capacityBelowUsage = $derived(!!node && capacity * 1024 ** 3 < node.usedBytes);
	const canUpdate = $derived(capacity >= 1 && !capacityBelowUsage);
	async function handleUpdate() {
		if (!node || !canUpdate) return;
		if (!name.trim()) {
			nameError = 'Nama node wajib diisi';
			return;
		}

		updating = true;
		nameError = '';
		const resp = await adminAPI.updateNode(node.id, {
			name: name.trim(),
			capacity_cap_gb: capacity,
			enabled
		});
		updating = false;

		if (resp.error) {
			if (/name.*(?:required|already exists)|nama.*(?:wajib|sudah|digunakan)/i.test(resp.error)) nameError = resp.error;
			else toast.error('Gagal memperbarui node', { description: resp.error });
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
				<div class="space-y-2">
					<Label for="edit-node-name">Nama Node</Label>
					<Input
						id="edit-node-name"
						data-testid="input-edit-node-name"
						bind:value={name}
						oninput={() => (nameError = '')}
						disabled={updating}
						aria-invalid={!!nameError}
						aria-describedby={nameError ? 'edit-node-name-error' : undefined}
					/>
					{#if nameError}
						<p id="edit-node-name-error" data-testid="error-edit-node-name" class="text-xs text-destructive" role="alert">{nameError}</p>
					{/if}
					{#if node.path}
						<p class="text-xs text-muted">{node.path}</p>
					{/if}
				</div>

				<div class="space-y-2">
					<Label for="edit-capacity">Kapasitas Maksimal (GB)</Label>
					<Input
						id="edit-capacity"
						data-testid="input-edit-node-capacity"
						type="number"
						min="1"
						bind:value={capacity}
						disabled={updating}
						aria-describedby={capacityBelowUsage ? 'edit-capacity-error' : undefined}
						aria-invalid={capacityBelowUsage}
					/>
					<p class="text-xs text-muted">
						Batas kapasitas yang diizinkan untuk node ini (minimal 1 GB).
					</p>
					{#if capacityBelowUsage}
						<p id="edit-capacity-error" data-testid="warning-node-capacity" class="text-xs text-destructive" role="alert">
							Kapasitas tidak boleh lebih kecil dari pemakaian node ({usedGb.toFixed(1)} GB).
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
