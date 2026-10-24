<script lang="ts">
	import {
		Dialog,
		DialogContent,
		DialogDescription,
		DialogFooter,
		DialogHeader,
		DialogTitle
	} from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { toast } from 'svelte-sonner';
	import { adminAPI } from '$lib/api/admin.js';
	import type { StorageNode } from '$lib/types/setup.js';
	import { locale } from '$lib/locale.svelte.js';
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
			nameError = locale.language === 'id' ? 'Nama node wajib diisi' : 'Node name is required';
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
			if (/name.*(?:required|already exists)|nama.*(?:wajib|sudah|digunakan)/i.test(resp.error))
				nameError =
					locale.language === 'id'
						? /already exists|sudah|digunakan/i.test(resp.error)
							? 'Nama node sudah digunakan'
							: 'Nama node wajib diisi'
						: /already exists/i.test(resp.error)
							? 'Node name already exists'
							: 'Node name is required';
			else
				toast.error(locale.language === 'id' ? 'Gagal memperbarui node' : 'Failed to update node', {
					description: resp.error
				});
		} else {
			toast.success(
				locale.language === 'id' ? 'Node berhasil diperbarui' : 'Node updated successfully'
			);
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
			<DialogTitle
				>{locale.language === 'id' ? 'Edit Node Penyimpanan' : 'Edit storage node'}</DialogTitle
			>
			<DialogDescription>
				{locale.language === 'id'
					? 'Ubah kapasitas maksimal dan status node'
					: 'Change the node’s maximum capacity and status'}
			</DialogDescription>
		</DialogHeader>

		{#if node}
			<div class="space-y-4">
				<div class="space-y-2">
					<Label for="edit-node-name">{locale.language === 'id' ? 'Nama Node' : 'Node name'}</Label>
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
						<p
							id="edit-node-name-error"
							data-testid="error-edit-node-name"
							class="text-xs text-destructive"
							role="alert"
						>
							{nameError}
						</p>
					{/if}
					{#if node.path}
						<p class="text-xs text-muted">{node.path}</p>
					{/if}
				</div>

				<div class="space-y-2">
					<Label for="edit-capacity"
						>{locale.language === 'id' ? 'Kapasitas Maksimal (GiB)' : 'Maximum capacity (GiB)'}</Label
					>
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
						{locale.language === 'id'
							? 'Batas kapasitas yang diizinkan untuk node ini (minimal 1 GiB).'
							: 'Capacity limit allowed for this node (minimum 1 GiB).'}
					</p>
					{#if capacityBelowUsage}
						<p
							id="edit-capacity-error"
							data-testid="warning-node-capacity"
							class="text-xs text-destructive"
							role="alert"
						>
							{locale.language === 'id'
								? `Kapasitas tidak boleh lebih kecil dari pemakaian node (${usedGb.toFixed(1)} GiB).`
								: `Capacity cannot be lower than the node’s current usage (${usedGb.toFixed(1)} GiB).`}
						</p>
					{/if}
				</div>

				<div class="flex items-center justify-between">
					<div class="space-y-0.5">
						<Label for="edit-enabled"
							>{locale.language === 'id' ? 'Status Node' : 'Node status'}</Label
						>
						<p class="text-xs text-muted">
							{locale.language === 'id'
								? 'Node yang dinonaktifkan tidak akan menerima file baru'
								: 'Disabled nodes will not receive new files'}
						</p>
					</div>
					<Switch id="edit-enabled" bind:checked={enabled} disabled={updating} />
				</div>
			</div>
		{/if}

		<DialogFooter>
			<Button variant="outline" onclick={() => (open = false)} disabled={updating}>
				{locale.language === 'id' ? 'Batal' : 'Cancel'}
			</Button>
			<Button onclick={handleUpdate} disabled={!canUpdate || updating}>
				{updating
					? locale.language === 'id'
						? 'Memperbarui...'
						: 'Updating...'
					: locale.language === 'id'
						? 'Simpan Perubahan'
						: 'Save changes'}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>
