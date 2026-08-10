<script lang="ts">
	import type { JobQueueItem, JobStatus } from '$lib/types/admin.js';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import { Badge, type BadgeVariant } from '$lib/components/ui/badge/index.js';

	let {
		open = $bindable(false),
		jobs = []
	}: {
		open?: boolean;
		jobs?: JobQueueItem[];
	} = $props();

	const statusVariant: Record<JobStatus, BadgeVariant> = {
		PENDING: 'outline',
		PROCESSING: 'default',
		COMPLETED: 'secondary',
		FAILED: 'destructive'
	};
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" class="border-subtle bg-surface sm:max-w-md">
		<Sheet.Header>
			<Sheet.Title>Antrean Job (SQLite Queue)</Sheet.Title>
			<Sheet.Description>
				Monitor worker queue — hanya tampilan (mock, read-only).
			</Sheet.Description>
		</Sheet.Header>

		<div class="flex flex-1 flex-col gap-3 overflow-y-auto px-4 pb-4">
			{#if jobs.length === 0}
				<p class="text-sm text-muted">Tidak ada job di antrean.</p>
			{:else}
				{#each jobs as job (job.id)}
					<div
						class="flex flex-col gap-2 rounded-xl border border-subtle bg-app p-4 shadow-none"
					>
						<div class="flex items-start justify-between gap-3">
							<div class="min-w-0 space-y-1">
								<p class="truncate text-sm font-medium text-main">{job.fileLabel}</p>
								<p class="text-xs text-muted">{job.type}</p>
							</div>
							<Badge variant={statusVariant[job.status]} class="shadow-none">
								{job.status}
							</Badge>
						</div>
						<p class="text-xs text-muted">{job.createdAt}</p>
					</div>
				{/each}
			{/if}
		</div>
	</Sheet.Content>
</Sheet.Root>
