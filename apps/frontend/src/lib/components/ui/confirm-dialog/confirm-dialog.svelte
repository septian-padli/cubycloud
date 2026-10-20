<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import type { ButtonVariant } from '$lib/components/ui/button/index.js';

	let {
		open = $bindable(false),
		title = 'Konfirmasi',
		description = '',
		confirmText = 'Konfirmasi',
		cancelText = 'Batal',
		variant = 'default',
		loading = false,
		onConfirm,
		onCancel
	}: {
		open?: boolean;
		title?: string;
		description?: string;
		confirmText?: string;
		cancelText?: string;
		variant?: ButtonVariant;
		loading?: boolean;
		onConfirm: () => void | Promise<void>;
		onCancel?: () => void;
	} = $props();

	async function handleConfirm() {
		await onConfirm();
		open = false;
	}

	function handleCancel() {
		onCancel?.();
		open = false;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main" data-testid="confirm-dialog">
		<Dialog.Header>
			<Dialog.Title>{title}</Dialog.Title>
			{#if description}
				<Dialog.Description class="text-sm text-muted">{description}</Dialog.Description>
			{/if}
		</Dialog.Header>
		<Dialog.Footer class="gap-2 sm:gap-0">
			<Button type="button" variant="secondary" onclick={handleCancel}>{cancelText}</Button>
			<Button type="button" {variant} disabled={loading} onclick={handleConfirm}>
				{confirmText}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
