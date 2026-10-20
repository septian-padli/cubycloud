<script lang="ts">
	import { goto } from '$app/navigation';
	import { authAPI } from '$lib/api/auth.js';
	import { setMockLoggedIn } from '$lib/mocks/auth-session.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { MoreVertical, LogOut } from '@lucide/svelte';
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

	async function handleLogout() {
		await authAPI.logout();
		setMockLoggedIn(false);
		await goto('/login');
	}
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

		<div class="flex items-center justify-between gap-2 border-t border-subtle px-3 py-2">
			<a
				href="/drive/profile/{user.id}"
				class="min-w-0 flex-1 rounded-lg px-2 py-1.5 transition-colors hover:bg-primary/10 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				<p class="truncate text-sm font-medium text-main">{user.displayName || user.username}</p>
				<p class="truncate text-xs text-muted">{user.email}</p>
			</a>
			<DropdownMenu.Root>
				<DropdownMenu.Trigger>
					{#snippet child({ props })}
						<Button
							{...props}
							type="button"
							variant="ghost"
							size="icon"
							class="bg-surface text-muted hover:bg-primary hover:text-white aria-expanded:bg-primary aria-expanded:text-white"
							aria-label="Menu akun"
						>
							<MoreVertical class="size-4 stroke-2" />
						</Button>
					{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content side="top" align="end" sideOffset={8} class="w-[calc(16rem-1.5rem)] p-1.5">
					<DropdownMenu.Item
						variant="destructive"
						class="w-full gap-2.5 cursor-pointer rounded-lg px-3 py-2 text-sm font-medium text-danger hover:bg-danger/10 hover:text-danger focus:bg-danger/10 focus:text-danger data-[highlighted]:bg-danger/10 data-[highlighted]:text-danger [&_svg]:text-danger"
						onclick={handleLogout}
					>
						<LogOut class="size-4 stroke-2 text-danger" />
						Logout
					</DropdownMenu.Item>
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		</div>
	</div>
</aside>
