<script lang="ts">
	import { locale } from '$lib/locale.svelte.js';
	import { LayoutGrid, LayoutList } from '@lucide/svelte';
	import { driveView, type DriveViewMode } from '$lib/drive-view.svelte.js';
	import { cn } from '$lib/utils.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	const options = $derived([
		{ id: 'list' as DriveViewMode, label: tx('Daftar', 'List'), icon: LayoutList },
		{ id: 'grid' as DriveViewMode, label: tx('Kisi', 'Grid'), icon: LayoutGrid }
	]);
</script>

<div
	class="inline-flex items-center gap-1 rounded-full border border-subtle bg-surface p-0.5"
	role="group"
	aria-label={tx('Tampilan daftar', 'View mode')}
>
	{#each options as opt (opt.id)}
		{@const Icon = opt.icon}
		{@const active = driveView.mode === opt.id}
		<button
			type="button"
			class={cn(
				'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1.5 text-xs font-medium transition-colors',
				active
					? 'bg-primary text-primary-foreground'
					: 'text-muted hover:bg-primary/10 hover:text-main'
			)}
			aria-pressed={active}
			aria-label={tx(`Tampilan ${opt.label}`, `${opt.label} view`)}
			onclick={() => driveView.setMode(opt.id)}
		>
			<Icon class="size-3.5 stroke-2" aria-hidden="true" />
			<span class="hidden sm:inline">{opt.label}</span>
		</button>
	{/each}
</div>
