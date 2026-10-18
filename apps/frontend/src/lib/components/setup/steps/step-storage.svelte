<script lang="ts">
	import type { LinuxDevice, StorageNode } from '$lib/types/setup.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Checkbox } from '$lib/components/ui/checkbox/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Progress } from '$lib/components/ui/progress/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import { RefreshCw } from '@lucide/svelte';
	import { scanStorageNodes } from '$lib/api/setup';
	import { toast } from 'svelte-sonner';

	let {
		nodes = $bindable(),
		devices = $bindable(),
		loading = false
	}: {
		nodes: StorageNode[];
		devices: LinuxDevice[];
		loading?: boolean;
	} = $props();

	let scanning = $state(false);

	async function rescan() {
		scanning = true;
		try {
			const data = await scanStorageNodes();
			nodes = data.nodes.map((n, i) => ({
				id: `node-${i}`,
				name: n.path.split('/').pop() || n.path,
				path: n.path,
				kind: (n.kind === 'SSD' || n.kind === 'HDD' ? n.kind : 'HDD') as 'SSD' | 'HDD',
				totalGb: Math.round(n.capacity_bytes / 1e9),
				freeGb: Math.round(n.free_bytes / 1e9),
				enabled: true,
				capacityCapGb: Math.round(n.capacity_bytes / 1e9)
			}));
		} catch {
			toast.error('Scan gagal');
		} finally {
			scanning = false;
		}
	}

	function usedPercent(node: StorageNode) {
		if (node.totalGb <= 0) return 0;
		return ((node.totalGb - node.freeGb) / node.totalGb) * 100;
	}
</script>

<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
	<Card.Header class="flex flex-row items-start justify-between gap-3 space-y-0">
		<div>
			<Card.Title class="text-lg font-semibold text-main">Penyimpanan Lokal (Opsional)</Card.Title>
			<Card.Description class="text-sm text-muted">
				Pilih volume di <code class="text-xs">/app/data/storage/</code> dan tetapkan batas kapasitas
				node. Bisa dilewati jika hanya memakai penyimpanan cloud.
			</Card.Description>
		</div>
		<Button
			variant="outline"
			size="sm"
			class="shrink-0 gap-1.5"
			disabled={scanning || loading}
			onclick={rescan}
		>
			<RefreshCw class="size-3.5 stroke-2 {scanning ? 'animate-spin' : ''}" />
			Pindai Ulang
		</Button>
	</Card.Header>
	<Card.Content class="space-y-4">
		{#if scanning || loading}
			<div class="space-y-2 rounded-lg border border-subtle bg-app px-4 py-3">
				<p class="text-xs font-medium text-muted">Memindai volume…</p>
				<Progress value={66} max={100} />
			</div>
		{:else}
			<div class="space-y-3">
				{#each nodes as node, i (node.id)}
					<div class="rounded-xl border border-subtle bg-app px-4 py-4">
						<div class="flex items-start gap-3">
							<Checkbox
								id="node-{node.id}"
								checked={node.enabled}
								onCheckedChange={(v) => {
									nodes[i].enabled = v === true;
								}}
								class="mt-1"
							/>
							<div class="min-w-0 flex-1 space-y-3">
								<div class="flex flex-wrap items-center gap-2">
									<Label for="node-{node.id}" class="text-sm font-medium text-main">
										{node.name}
									</Label>
									<Badge
										class="rounded-full bg-secondary px-2.5 py-1 text-xs font-medium text-main dark:text-app hover:bg-secondary"
									>
										{node.kind}
									</Badge>
								</div>
								<p class="text-xs text-muted">{node.path}</p>
								<div>
									<div class="mb-1.5 flex justify-between text-xs text-muted">
										<span>{node.freeGb} GB bebas</span>
										<span>{node.totalGb} GB total</span>
									</div>
									<div class="h-1.5 rounded-full bg-black/10 dark:bg-white/10">
										<div
											class="h-1.5 rounded-full bg-primary"
											style="width: {usedPercent(node)}%"
										></div>
									</div>
								</div>
								{#if node.enabled}
									<div class="space-y-2">
										<Label for="cap-{node.id}" class="text-xs text-muted">
											Batas kapasitas node (GB)
										</Label>
										<Input
											id="cap-{node.id}"
											type="number"
											min={1}
											max={node.totalGb}
											class="max-w-xs"
											bind:value={nodes[i].capacityCapGb}
										/>
									</div>
								{/if}
							</div>
						</div>
					</div>
				{/each}
			</div>
		{/if}

		<Separator />

		<div>
			<p class="text-sm font-medium text-main">Device Linux (opsional)</p>
			<p class="mt-1 text-xs text-muted">
				Daftarkan device terdeteksi sebagai node tambahan. (Mock UI — belum di-mount.)
			</p>
			<div class="mt-3 space-y-2">
				{#each devices as device, i (device.id)}
					<label
						class="flex cursor-pointer items-center gap-3 rounded-lg border border-subtle bg-app px-4 py-3"
					>
						<Checkbox
							checked={device.selected}
							onCheckedChange={(v) => {
								devices[i].selected = v === true;
							}}
						/>
						<div class="min-w-0 flex-1">
							<p class="text-sm font-medium text-main">{device.path}</p>
							<p class="text-xs text-muted">
								{device.model} · {device.sizeGb} GB
							</p>
						</div>
					</label>
				{/each}
			</div>
		</div>
	</Card.Content>
</Card.Root>
