<script lang="ts">
	import type { StorageNode } from '$lib/types/setup.js';
	import { isNodeCritical } from '$lib/mocks/admin-settings.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Progress } from '$lib/components/ui/progress/index.js';
	import { HardDrive } from '@lucide/svelte';

	let {
		nodes,
		onDetach
	}: {
		nodes: StorageNode[];
		onDetach: (node: StorageNode) => void;
	} = $props();

	function usedPercent(node: StorageNode) {
		if (node.totalGb <= 0) return 0;
		return ((node.totalGb - node.freeGb) / node.totalGb) * 100;
	}
</script>

{#if nodes.length === 0}
	<div class="rounded-xl border border-subtle bg-surface px-4 py-8 text-center shadow-none">
		<p class="text-sm text-muted">Belum ada node penyimpanan.</p>
	</div>
{:else}
	<div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
		{#each nodes as node (node.id)}
			{@const critical = isNodeCritical(node)}
			{@const used = usedPercent(node)}
			<Card.Root
				class="rounded-xl border border-subtle bg-surface shadow-none {!node.enabled
					? 'opacity-60'
					: ''}"
			>
				<Card.Content class="flex flex-col gap-3">
					<div class="flex items-start justify-between gap-2">
						<div class="flex min-w-0 items-center gap-2">
							<span
								class="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10"
							>
								<HardDrive class="size-4 stroke-2 text-primary" />
							</span>
							<div class="min-w-0">
								<p class="truncate text-sm font-medium text-main">{node.name}</p>
								<p class="truncate text-xs text-muted">{node.path}</p>
							</div>
						</div>
						<div class="flex shrink-0 flex-wrap justify-end gap-1.5">
							<Badge
								class="rounded-full bg-secondary px-2.5 py-0.5 text-xs font-medium text-main dark:text-app hover:bg-secondary"
							>
								{node.kind}
							</Badge>
							{#if critical}
								<Badge variant="destructive" class="rounded-full">Kritis (&lt;5%)</Badge>
							{/if}
							{#if !node.enabled}
								<Badge variant="outline" class="rounded-full border-subtle text-muted">
									Lepas
								</Badge>
							{/if}
						</div>
					</div>

					<div class="space-y-2">
						<div class="flex justify-between text-xs text-muted">
							<span>{node.freeGb} GB bebas</span>
							<span>{node.totalGb} GB total</span>
						</div>
						<Progress
							value={used}
							max={100}
							indicatorClass={critical ? 'bg-danger' : 'bg-primary'}
						/>
						<p class="text-xs text-muted">
							Batas kapasitas: {node.capacityCapGb} GB · terpakai {Math.round(used)}%
						</p>
					</div>

					<Button
						type="button"
						variant="secondary"
						class="w-full"
						disabled={!node.enabled}
						onclick={() => onDetach(node)}
					>
						Lepaskan node
					</Button>
				</Card.Content>
			</Card.Root>
		{/each}
	</div>
{/if}
