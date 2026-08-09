<script lang="ts">
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import { getDriveChrome } from '$lib/drive-context.js';
	import DriveSidebar from './drive-sidebar.svelte';
	import DriveMain from './drive-main.svelte';
	import type { Snippet } from 'svelte';

	let {
		activeId = 'home',
		content
	}: {
		activeId?: string;
		content: Snippet;
	} = $props();

	const { user, quota } = getDriveChrome();
	let mobileOpen = $state(false);
</script>

<div class="h-dvh overflow-hidden bg-app text-main">
	<div class="fixed inset-y-0 start-0 z-40 hidden w-64 md:block">
		<DriveSidebar {user} {quota} {activeId} class="h-full" />
	</div>

	<Sheet.Root bind:open={mobileOpen}>
		<Sheet.Content side="left" class="w-64 gap-0 border-subtle bg-surface p-0 sm:max-w-none">
			<Sheet.Title class="sr-only">Navigasi CubyCloud</Sheet.Title>
			<Sheet.Description class="sr-only">Menu drive dan kuota penyimpanan</Sheet.Description>
			<DriveSidebar {user} {quota} {activeId} class="h-full w-full border-0" />
		</Sheet.Content>
	</Sheet.Root>

	<div class="h-full md:ps-64">
		<DriveMain
			{content}
			onMenuClick={() => {
				mobileOpen = true;
			}}
		/>
	</div>
</div>
