<script lang="ts">
	import type { QuotaState } from '$lib/types/drive.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import QuotaWidget from './quota-widget.svelte';

	let {
		quota
	}: {
		quota: QuotaState;
	} = $props();
</script>

<section class="space-y-3">
	<h2 class="text-sm font-semibold text-main">Detail kuota My Storage</h2>
	<Card.Root class="rounded-xl border-subtle bg-surface py-0 shadow-none">
		<Card.Content class="p-0">
			<QuotaWidget
				usedGb={quota.usedGb}
				limitGb={quota.limitGb}
				showPercent
				class="border-t-0"
			/>
			<div class="border-t border-subtle px-4 py-3">
				<p class="text-xs text-muted">
					Kuota My Storage tidak menghitung berkas Organization. Item di Trash tetap dihitung
					sampai dihapus permanen.
				</p>
			</div>
		</Card.Content>
	</Card.Root>
</section>
