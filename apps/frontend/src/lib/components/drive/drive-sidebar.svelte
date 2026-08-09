<script lang="ts">
	import DriveSidebarNav from './drive-sidebar-nav.svelte';
	import QuotaWidget from './quota-widget.svelte';
	import type { DriveUser, QuotaState } from '$lib/types/drive.js';
	import { cn } from '$lib/utils.js';

	let {
		user,
		quota,
		activeId = 'home',
		class: className
	}: {
		user: DriveUser;
		quota: QuotaState;
		activeId?: string;
		class?: string;
	} = $props();

	const isSuperAdmin = $derived(user.role === 'super_admin');
</script>

<aside
	class={cn(
		'flex h-full min-h-0 w-64 flex-col border-r border-subtle bg-surface text-main',
		className
	)}
>
	<div class="flex shrink-0 items-center gap-2 border-b border-subtle px-4 py-4">
		<p class="text-sm font-semibold">CubyCloud</p>
	</div>

	<div class="min-h-0 flex-1 overflow-y-auto">
		<DriveSidebarNav showAdmin={isSuperAdmin} {activeId} />
	</div>

	<div class="mt-auto shrink-0">
		<QuotaWidget usedGb={quota.usedGb} limitGb={quota.limitGb} />

		<div class="border-t border-subtle px-4 py-3">
			<p class="truncate text-sm font-medium text-main">{user.displayName}</p>
			<p class="truncate text-xs text-muted">@{user.username}</p>
		</div>
	</div>
</aside>
