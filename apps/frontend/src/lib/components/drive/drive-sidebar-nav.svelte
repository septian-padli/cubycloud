<script lang="ts">
	import {
		Blocks,
		Building2,
		HardDrive,
		Home,
		PieChart,
		Settings,
		Star,
		Trash2,
		UserCog,
		Users
	} from '@lucide/svelte';
	import { cn } from '$lib/utils.js';
	import type { Component } from 'svelte';

	type NavItem = {
		id: string;
		label: string;
		icon: Component;
		badge?: string;
	};

	let {
		showAdmin = false,
		activeId = 'home',
		class: className
	}: {
		showAdmin?: boolean;
		activeId?: string;
		class?: string;
	} = $props();

	const driveItems: NavItem[] = [
		{ id: 'home', label: 'Beranda', icon: Home },
		{ id: 'my-storage', label: 'My Storage', icon: HardDrive },
		{ id: 'organization', label: 'Organization', icon: Building2, badge: '🏢' },
		{ id: 'shared', label: 'Dibagikan pada saya', icon: Users },
		{ id: 'starred', label: 'Berbintang', icon: Star },
		{ id: 'trash', label: 'Trash', icon: Trash2 },
		{ id: 'storage-stats', label: 'Penyimpanan', icon: PieChart }
	];

	const adminItems: NavItem[] = [
		{ id: 'admin-settings', label: 'Admin Settings', icon: Settings },
		{ id: 'admin-users', label: 'Users & Quotas', icon: UserCog },
		{ id: 'admin-addons', label: 'Add-ons Manager', icon: Blocks }
	];
</script>

<nav class={cn('flex flex-col gap-1 px-3 py-2', className)} aria-label="Navigasi drive">
	{#each driveItems as item (item.id)}
		{@const active = item.id === activeId}
		{@const ItemIcon = item.icon}
		<button
			type="button"
			disabled={!active}
			aria-current={active ? 'page' : undefined}
			class={cn(
				'flex w-full items-center gap-2.5 rounded-full px-3 py-2 text-left text-sm font-medium transition-colors',
				active
					? 'bg-primary text-primary-foreground'
					: 'cursor-default text-muted hover:bg-primary/10'
			)}
		>
			<ItemIcon class="size-4 shrink-0 stroke-2" aria-hidden="true" />
			<span class="min-w-0 flex-1 truncate">{item.label}</span>
			{#if item.badge}
				<span class="text-xs" aria-hidden="true">{item.badge}</span>
			{/if}
		</button>
	{/each}

	{#if showAdmin}
		<div class="my-2 border-t border-subtle" role="separator"></div>
		<p class="px-3 pb-1 text-xs font-medium text-muted">Super Admin</p>
		{#each adminItems as item (item.id)}
			{@const ItemIcon = item.icon}
			<button
				type="button"
				disabled
				class="flex w-full cursor-default items-center gap-2.5 rounded-full px-3 py-2 text-left text-sm font-medium text-muted hover:bg-primary/10"
			>
				<ItemIcon class="size-4 shrink-0 stroke-2" aria-hidden="true" />
				<span class="min-w-0 flex-1 truncate">{item.label}</span>
			</button>
		{/each}
	{/if}
</nav>
