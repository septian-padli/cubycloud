<script lang="ts">
	import * as Popover from '$lib/components/ui/popover/index.js';
	import { Info } from '@lucide/svelte';
	import { locale } from '$lib/locale.svelte.js';

	let open = $state(false);
</script>

<Popover.Root bind:open>
	<Popover.Trigger>
		{#snippet child({ props })}
			<button
				{...props}
				type="button"
				aria-label={locale.t('offline.message')}
				class="rounded-full text-danger focus-visible:outline-2 focus-visible:outline-primary"
				onclick={(event) => {
					event.stopPropagation();
					open = true;
				}}
				onmouseenter={() => (open = true)}
			>
				<Info class="size-4" />
			</button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Content side="left" class="w-64">
		<p>{locale.t('offline.message')}</p>
	</Popover.Content>
</Popover.Root>
