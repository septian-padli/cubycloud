<script lang="ts">
	import { Globe } from '@lucide/svelte';
	import * as Select from '$lib/components/ui/select/index.js';
	import { locale } from '$lib/locale.svelte.js';
	import { cn } from '$lib/utils.js';

	let { class: className = '' }: { class?: string } = $props();
</script>

<Select.Root
	type="single"
	value={locale.language}
	onValueChange={(value) => {
		if (value === 'en' || value === 'id') locale.set(value);
	}}
>
	<Select.Trigger
		aria-label={locale.t('language.label')}
		class={cn(
			'h-7 w-auto gap-1.5 rounded-full border border-subtle bg-surface px-2.5 text-xs font-medium text-main shadow-none transition-colors hover:bg-surface/80 focus-visible:ring-1 focus-visible:ring-primary [&_svg:first-child]:size-3.5 [&_svg:last-child]:size-3.5',
			className
		)}
	>
		<Globe class="size-3.5 shrink-0 text-muted" aria-hidden="true" />
		<span>{locale.language}</span>
	</Select.Trigger>
	<Select.Content class="border-subtle bg-surface">
		<Select.Item value="en" label="en">en</Select.Item>
		<Select.Item value="id" label="id">id</Select.Item>
	</Select.Content>
</Select.Root>
