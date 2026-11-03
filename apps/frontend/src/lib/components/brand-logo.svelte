<script lang="ts">
	let {
		variant = 'horizontal',
		class: className = '',
		imgClass = 'h-6 w-auto'
	}: {
		variant?: 'horizontal' | 'icon';
		class?: string;
		imgClass?: string;
	} = $props();
</script>

<span class="inline-flex items-center {className}">
	{#if variant === 'horizontal'}
		<img
			src="/logos/horizontal/default/cubycrate-horizontal-color.svg"
			alt="CubyCrate"
			class="{imgClass} dark:hidden"
		/>
		<img
			src="/logos/horizontal/default/cubycrate-horizontal-white.svg"
			alt="CubyCrate"
			class="{imgClass} hidden dark:block"
		/>
	{:else}
		<img
			src="/logos/icon/cubycrate-icon-color.svg"
			alt="CubyCrate"
			class="{imgClass} dark:hidden"
		/>
		<img
			src="/logos/icon/cubycrate-icon-white.svg"
			alt="CubyCrate"
			class="{imgClass} hidden dark:block"
		/>
	{/if}
</span>
