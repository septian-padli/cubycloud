<script lang="ts">
	import type { LargestFileItem } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let {
		open = $bindable(false),
		suggestions = [],
		trashCount = 0,
		onGoTrash
	}: {
		open?: boolean;
		suggestions?: LargestFileItem[];
		trashCount?: number;
		onGoTrash?: () => void;
	} = $props();

	const top = $derived(suggestions.slice(0, 3));
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>{tx('Saran pembersihan', 'Cleanup suggestions')}</Dialog.Title>
			<Dialog.Description>
				{tx(
					'Beberapa cara cepat mengosongkan kuota My Storage (mock).',
					'A few quick ways to free up My Storage quota (mock).'
				)}
			</Dialog.Description>
		</Dialog.Header>

		<div class="grid gap-4">
			<div class="rounded-xl border border-subtle bg-app p-4">
				<p class="text-sm font-medium text-main">{tx('Kosongkan Trash', 'Empty Trash')}</p>
				<p class="mt-1 text-xs text-muted">
					{tx(
						`Ada ${trashCount} item di Trash yang masih dihitung ke kuota sampai dihapus permanen.`,
						`${trashCount} items in Trash still count toward your quota until permanently deleted.`
					)}
				</p>
				{#if onGoTrash}
					<Button type="button" variant="secondary" class="mt-3" onclick={onGoTrash}>
						{tx('Kelola Sampah', 'Manage Trash')}
					</Button>
				{/if}
			</div>

			{#if top.length > 0}
				<div class="rounded-xl border border-subtle bg-app p-4">
					<p class="text-sm font-medium text-main">{tx('Berkas terbesar', 'Largest files')}</p>
					<ul class="mt-2 space-y-2">
						{#each top as file (file.id)}
							<li class="flex items-center justify-between gap-2 text-xs">
								<span class="truncate text-main">{file.name}</span>
								<span class="shrink-0 text-muted">{formatSize(file.sizeBytes)}</span>
							</li>
						{/each}
					</ul>
				</div>
			{/if}
		</div>

		<Dialog.Footer>
			<Button type="button" variant="secondary" onclick={() => (open = false)}
				>{tx('Tutup', 'Close')}</Button
			>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
