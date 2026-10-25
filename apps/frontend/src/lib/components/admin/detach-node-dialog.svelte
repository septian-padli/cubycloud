<script lang="ts">
	import type { StorageNode } from '$lib/types/setup.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { adminAPI } from '$lib/api/admin.js';
	import { toast } from 'svelte-sonner';
	import { locale } from '$lib/locale.svelte.js';

	let {
		open = $bindable(false),
		node = null,
		onDetach
	}: {
		open?: boolean;
		node?: StorageNode | null;
		onDetach: (nodeId: string) => void;
	} = $props();

	let detaching = $state(false);

	async function confirm() {
		if (!node || detaching) return;
		detaching = true;

		try {
			const res = await adminAPI.detachNode(node.id);
			if (res.error) {
				toast.error(
					locale.language === 'id' ? 'Gagal melepaskan node' : 'Failed to detach node',
					{ description: res.error }
				);
			} else if (res.data) {
				if (res.data.safe_to_disconnect) {
					toast.success(
						locale.language === 'id'
							? `Node “${node.name}” berhasil dilepaskan dan aman dicabut.`
							: `Node “${node.name}” detached and safe to disconnect.`
					);
				} else {
					toast.info(
						locale.language === 'id'
							? `Migrasi berkas fisik node “${node.name}” dimulai di latar belakang.`
							: `Physical file migration for node “${node.name}” started in background.`,
						{
							description:
								locale.language === 'id'
									? 'Seluruh data akan dipindahkan dan diverifikasi sebelum node dinyatakan aman dicabut.'
									: 'All data will be copied and verified before the node is declared safe to disconnect.'
						}
					);
				}
				onDetach(node.id);
				open = false;
			}
		} catch (err) {
			toast.error(
				locale.language === 'id' ? 'Terjadi kesalahan sistem' : 'System error occurred'
			);
		} finally {
			detaching = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>{locale.language === 'id' ? 'Lepaskan node?' : 'Detach node?'}</Dialog.Title>
			<Dialog.Description>
				{#if node}
					{#if node.usedBytes > 0}
						{locale.language === 'id'
							? `Node “${node.name}” memiliki berkas tersimpan. Operasi ini akan menonaktifkan alokasi baru dan memigrasikan seluruh berkas fisik ke node tujuan secara streaming dengan verifikasi checksum.`
							: `Node “${node.name}” contains stored files. This will disable new allocations and migrate physical files to eligible nodes via streaming with checksum verification.`}
						<span class="mt-2 block font-medium text-warning">
							{locale.language === 'id'
								? 'Node belum aman dicabut hingga proses migrasi seluruh berkas selesai tanpa kesalahan.'
								: 'The node is not safe to disconnect until all files are completely and safely migrated.'}
						</span>
					{:else}
						{locale.language === 'id'
							? `Node “${node.name}” tidak memiliki berkas tersimpan dan akan langsung dinonaktifkan dari pool secara aman.`
							: `Node “${node.name}” has no stored files and will be immediately and safely disabled from the pool.`}
					{/if}
				{:else}
					{locale.language === 'id'
						? 'Pilih node yang ingin dilepaskan.'
						: 'Select a node to detach.'}
				{/if}
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer>
			<Button type="button" variant="secondary" disabled={detaching} onclick={() => (open = false)}>
				{locale.language === 'id' ? 'Batal' : 'Cancel'}
			</Button>
			<Button type="button" variant="destructive" disabled={!node || detaching} onclick={confirm}>
				{detaching
					? locale.language === 'id'
						? 'Memproses...'
						: 'Processing...'
					: locale.language === 'id'
						? 'Lepaskan'
						: 'Detach'}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
