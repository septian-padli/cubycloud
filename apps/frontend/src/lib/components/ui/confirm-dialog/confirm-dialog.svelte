<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import type { ButtonVariant } from '$lib/components/ui/button/index.js';
	import { locale } from '$lib/locale.svelte.js';

	let {
		open = $bindable(false),
		title,
		description = '',
		confirmText,
		cancelText,
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

	const resolvedTitle = $derived(title ?? (locale.language === 'id' ? 'Konfirmasi' : 'Confirm'));
	const resolvedConfirmText = $derived(
		confirmText ?? (locale.language === 'id' ? 'Konfirmasi' : 'Confirm')
	);
	const resolvedCancelText = $derived(
		cancelText ?? (locale.language === 'id' ? 'Batal' : 'Cancel')
	);

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
			<Dialog.Title>{resolvedTitle}</Dialog.Title>
			{#if description}
				<Dialog.Description class="text-sm text-muted">{description}</Dialog.Description>
			{/if}
		</Dialog.Header>
		<Dialog.Footer class="gap-2 sm:gap-0">
			<Button type="button" variant="secondary" onclick={handleCancel}>{resolvedCancelText}</Button>
			<Button type="button" {variant} disabled={loading} onclick={handleConfirm}>
				{resolvedConfirmText}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
