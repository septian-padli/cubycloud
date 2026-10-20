<script lang="ts">
	import { Cloud, HardDrive, Settings } from '@lucide/svelte';
	import type { StorageNode } from '$lib/types/setup.js';
	import { isNodeCritical } from '$lib/mocks/admin-settings.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Progress } from '$lib/components/ui/progress/index.js';
	let {
		nodes,
		onEdit
	}: {
		nodes: StorageNode[];
		onEdit: (node: StorageNode) => void;
	} = $props();

	function usedPercent(node: StorageNode) {
		if (node.capacityCapGb <= 0) return 0;
		return (node.usedBytes / (node.capacityCapGb * 1024 ** 3)) * 100;
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
								{#if node.kind === 'S3'}
									<Cloud class="size-4 stroke-2 text-secondary-accent" />
								{:else}
									<HardDrive class="size-4 stroke-2 text-primary" />
								{/if}
							</span>
							<div class="min-w-0">
								<p class="truncate text-sm font-medium text-main">{node.name}</p>
								{#if node.kind === 'S3'}
									<p class="truncate text-xs text-muted" title={node.endpoint}>{node.endpoint}</p>
									<p class="truncate text-xs text-muted" title={node.bucket}>Bucket: {node.bucket}</p>
								{:else}
									<p class="truncate text-xs text-muted">{node.path}</p>
								{/if}
							</div>
						</div>
						<div class="flex shrink-0 flex-wrap justify-end gap-1.5">
							<Badge
								class="rounded-full px-2.5 py-0.5 text-xs font-medium {node.kind === 'S3'
									? 'bg-secondary-accent text-white dark:text-app hover:bg-secondary-accent'
									: 'bg-secondary text-main dark:text-app hover:bg-secondary'}"
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
							<span>{node.freeGb.toFixed(1)} GB bebas</span>
							<span>{(node.usedBytes / 1024 ** 3).toFixed(1)} / {node.capacityCapGb} GB</span>
						</div>
						<Progress
							value={used}
							max={100}
							indicatorClass={critical ? 'bg-danger' : 'bg-primary'}
						/>
						<p class="text-xs text-muted">
							Terpakai {(node.usedBytes / 1024 ** 3).toFixed(1)} GB ({Math.round(used)}%)
						</p>
					</div>

					<Button
						type="button"
						variant="outline"
						size="sm"
						class="w-full gap-1.5"
						onclick={() => onEdit(node)}
					>
						<Settings class="size-3.5" />
						Edit
					</Button>
				</Card.Content>
			</Card.Root>
		{/each}
	</div>
{/if}
