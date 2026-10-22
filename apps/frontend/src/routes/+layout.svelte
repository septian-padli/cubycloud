<script lang="ts">
	import { onMount } from 'svelte';
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { theme } from '$lib/theme.svelte.js';
	import { Toaster } from '$lib/components/ui/sonner/index.js';
	import { locale } from '$lib/locale.svelte.js';
	import { Globe } from '@lucide/svelte';
	import * as Select from '$lib/components/ui/select/index.js';

	let { children } = $props();

	onMount(() => {
		theme.init();
		void locale.init();
	});
	$effect(() => {
		if (typeof document !== 'undefined') document.documentElement.lang = locale.language;
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<link rel="preconnect" href="https://fonts.googleapis.com" />
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous" />
	<link
		href="https://fonts.googleapis.com/css2?family=Montserrat:wght@400;500;600;700&display=swap"
		rel="stylesheet"
	/>
</svelte:head>

{@render children()}
<div
	class="fixed top-4 right-4 z-50 flex items-center gap-1 rounded-full border border-subtle bg-surface/95 p-1"
>
	<Globe class="size-4 text-muted" aria-hidden="true" />
	<Select.Root
		type="single"
		value={locale.language}
		onValueChange={(value) => {
			if (value === 'en' || value === 'id') locale.set(value);
		}}
	>
		<Select.Trigger
			aria-label={locale.t('language.label')}
			class="h-8 w-16 border-0 bg-transparent px-2 text-xs"
		>
			<span>{locale.language}</span>
		</Select.Trigger>
		<Select.Content class="border-subtle bg-surface">
			<Select.Item value="en" label="en">en</Select.Item>
			<Select.Item value="id" label="id">id</Select.Item>
		</Select.Content>
	</Select.Root>
</div>
<Toaster closeButton position="bottom-right" />
