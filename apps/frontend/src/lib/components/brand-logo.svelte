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
			src="/logos/horizontal/default/cubycrate-horizontal-color.webp"
			alt="CubyCrate"
			width="1216"
			height="372"
			class="{imgClass} aspect-[1216/372] object-contain dark:hidden"
		/>
		<img
			src="/logos/horizontal/default/cubycrate-horizontal-white.webp"
			alt="CubyCrate"
			width="1216"
			height="372"
			class="{imgClass} aspect-[1216/372] object-contain hidden dark:block"
		/>
	{:else}
		<img
			src="/logos/icon/cubycrate-icon-color.webp"
			alt="CubyCrate"
			width="612"
			height="760"
			class="{imgClass} aspect-[612/760] object-contain dark:hidden"
		/>
		<img
			src="/logos/icon/cubycrate-icon-white.webp"
			alt="CubyCrate"
			width="612"
			height="760"
			class="{imgClass} aspect-[612/760] object-contain hidden dark:block"
		/>
	{/if}
</span>
