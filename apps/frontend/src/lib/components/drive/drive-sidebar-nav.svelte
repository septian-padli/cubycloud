<script lang="ts">
	import {
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
		href?: string;
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
		{ id: 'home', label: 'Beranda', icon: Home, href: '/drive/home' },
		{ id: 'my-storage', label: 'My Storage', icon: HardDrive, href: '/drive/my-storage' },
		{
			id: 'organization',
			label: 'Organization',
			icon: Building2,
			href: '/drive/organization'
		},
		{ id: 'shared', label: 'Dibagikan pada saya', icon: Users, href: '/drive/shared' },
		{ id: 'starred', label: 'Berbintang', icon: Star, href: '/drive/starred' },
		{ id: 'trash', label: 'Trash', icon: Trash2, href: '/drive/trash' },
		{ id: 'storage-stats', label: 'Penyimpanan', icon: PieChart, href: '/drive/storage-stats' }
	];

	const adminItems: NavItem[] = [
		{ id: 'admin-storage', label: 'Penyimpanan', icon: HardDrive, href: '/admin/storage' },
		{ id: 'admin-settings', label: 'Admin Settings', icon: Settings, href: '/admin/settings' },
		{ id: 'admin-users', label: 'Users & Quotas', icon: UserCog, href: '/admin/users' },
	];

	const pillClass = (active: boolean, linked: boolean) =>
		cn(
			'flex w-full items-center gap-2.5 rounded-full px-3 py-2 text-left text-sm font-medium transition-colors',
			active
				? 'bg-primary text-primary-foreground'
				: linked
					? 'text-muted hover:bg-primary/10'
					: 'cursor-default text-muted hover:bg-primary/10'
		);
</script>

<nav class={cn('flex flex-col gap-1 px-3 py-2', className)} aria-label="Navigasi drive">
	{#each driveItems as item (item.id)}
		{@const active = item.id === activeId}
		{@const ItemIcon = item.icon}
		{#if item.href}
			<a
				href={item.href}
				data-sveltekit-preload-data="hover"
				aria-current={active ? 'page' : undefined}
				class={pillClass(active, true)}
			>
				<ItemIcon class="size-4 shrink-0 stroke-2" aria-hidden="true" />
				<span class="min-w-0 flex-1 truncate">{item.label}</span>
				{#if item.badge}
					<span class="text-xs" aria-hidden="true">{item.badge}</span>
				{/if}
			</a>
		{:else}
			<button
				type="button"
				disabled
				aria-current={active ? 'page' : undefined}
				class={pillClass(active, false)}
			>
				<ItemIcon class="size-4 shrink-0 stroke-2" aria-hidden="true" />
				<span class="min-w-0 flex-1 truncate">{item.label}</span>
				{#if item.badge}
					<span class="text-xs" aria-hidden="true">{item.badge}</span>
				{/if}
			</button>
		{/if}
	{/each}

	{#if showAdmin}
		<div class="my-2 border-t border-subtle" role="separator"></div>
		<p class="px-3 pb-1 text-xs font-medium text-muted">Super Admin</p>
		{#each adminItems as item (item.id)}
			{@const active = item.id === activeId}
			{@const ItemIcon = item.icon}
			{#if item.href}
				<a
					href={item.href}
					data-sveltekit-preload-data="hover"
					aria-current={active ? 'page' : undefined}
					class={pillClass(active, true)}
				>
					<ItemIcon class="size-4 shrink-0 stroke-2" aria-hidden="true" />
					<span class="min-w-0 flex-1 truncate">{item.label}</span>
				</a>
			{/if}
		{/each}
	{/if}
</nav>
