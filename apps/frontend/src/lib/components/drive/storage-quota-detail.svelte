<script lang="ts">
	import { locale } from '$lib/locale.svelte.js';
	import type { QuotaState } from '$lib/types/drive.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import QuotaWidget from './quota-widget.svelte';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let {
		quota
	}: {
		quota: QuotaState;
	} = $props();
</script>

<section class="space-y-3">
	<h2 class="text-sm font-semibold text-main">
		{tx('Detail kuota My Storage', 'My Storage quota details')}
	</h2>
	<Card.Root class="rounded-xl border-subtle bg-surface py-0 shadow-none">
		<Card.Content class="p-0">
			<QuotaWidget usedGb={quota.usedGb} limitGb={quota.limitGb} showPercent class="border-t-0" />
			<div class="border-t border-subtle px-4 py-3">
				<p class="text-xs text-muted">
					{tx(
						'Kuota My Storage tidak menghitung berkas Organization. Item di Trash tetap dihitung sampai dihapus permanen.',
						'My Storage quota excludes Organization files. Items in Trash continue to count until permanently deleted.'
					)}
				</p>
			</div>
		</Card.Content>
	</Card.Root>
</section>
