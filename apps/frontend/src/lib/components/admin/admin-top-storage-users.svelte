<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { ArrowUpRight, Loader2, ShieldCheck, User, Users } from '@lucide/svelte';
	import type { AdminTopStorageUser } from '$lib/types/admin.js';
	import { adminAPI } from '$lib/api/admin.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { cn } from '$lib/utils.js';
	import { locale } from '$lib/locale.svelte.js';
	let {
		totalSystemUsedBytes = 0,
		class: className
	}: {
		totalSystemUsedBytes?: number;
		class?: string;
	} = $props();

	let users = $state<AdminTopStorageUser[]>([]);
	let loading = $state(true);
	let loadingMore = $state(false);
	let hasMore = $state(false);
	let nextOffset = $state(0);
	let sentinelEl = $state<HTMLDivElement | null>(null);

	async function loadInitial() {
		loading = true;
		try {
			const res = await adminAPI.getTopStorageUsers(20, 0);
			if (res.data) {
				users = res.data.items ?? [];
				hasMore = res.data.has_more ?? false;
				nextOffset = res.data.next_offset ?? 0;
			} else if (res.error) {
				toast.error(
					`${locale.language === 'id' ? 'Gagal memuat pengguna terbesar' : 'Failed to load top storage users'}: ${res.error}`
				);
			}
		} catch (error) {
			toast.error(
				error instanceof Error
					? error.message
					: locale.language === 'id'
						? 'Gagal memuat pengguna'
						: 'Failed to load users'
			);
		} finally {
			loading = false;
		}
	}

	async function loadMore() {
		if (loadingMore || !hasMore) return;
		try {
			loadingMore = true;
			const res = await adminAPI.getTopStorageUsers(20, nextOffset);
			if (res.data) {
				users = [...users, ...(res.data.items ?? [])];
				hasMore = res.data.has_more ?? false;
				nextOffset = res.data.next_offset ?? 0;
			} else if (res.error) {
				toast.error(
					`${locale.language === 'id' ? 'Gagal memuat halaman berikutnya' : 'Failed to load the next page'}: ${res.error}`
				);
			}
		} catch (error) {
			toast.error(
				error instanceof Error
					? error.message
					: locale.language === 'id'
						? 'Gagal memuat halaman berikutnya'
						: 'Failed to load the next page'
			);
		} finally {
			loadingMore = false;
		}
	}

	onMount(() => {
		void loadInitial();
	});

	$effect(() => {
		if (!sentinelEl || !hasMore) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0]?.isIntersecting) {
					void loadMore();
				}
			},
			{ rootMargin: '200px' }
		);
		observer.observe(sentinelEl);
		return () => observer.disconnect();
	});
</script>

<section class={cn('space-y-3', className)}>
	<div class="flex flex-wrap items-center justify-between gap-2">
		<div>
			<h2 class="text-sm font-semibold text-main">
				{locale.language === 'id' ? 'Pengguna dengan konsumsi terbesar' : 'Top storage users'}
			</h2>
			<p class="text-xs text-muted">
				{locale.language === 'id'
					? 'Diurutkan berdasarkan total ukuran penyimpanan berkas pengguna.'
					: 'Ranked by the total size of each user’s stored files.'}
			</p>
		</div>
		<Button
			variant="outline"
			size="sm"
			href="/admin/users"
			class="h-8 gap-1.5 text-xs text-muted hover:text-main"
		>
			<span>{locale.language === 'id' ? 'Kelola Pengguna & Kuota' : 'Manage users & quotas'}</span>
			<ArrowUpRight class="size-3.5" />
		</Button>
	</div>

	{#if loading && users.length === 0}
		<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
			<Card.Content class="p-6">
				<div class="space-y-4">
					{#each Array(4) as _}
						<div class="flex animate-pulse items-center justify-between gap-4">
							<div class="flex items-center gap-3">
								<div class="size-9 rounded-full bg-muted/20"></div>
								<div class="space-y-1.5">
									<div class="h-4 w-32 rounded bg-muted/20"></div>
									<div class="h-3 w-48 rounded bg-muted/10"></div>
								</div>
							</div>
							<div class="h-4 w-24 rounded bg-muted/20"></div>
						</div>
					{/each}
				</div>
			</Card.Content>
		</Card.Root>
	{:else if users.length === 0}
		<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
			<Card.Content class="py-12 text-center">
				<Users class="mx-auto size-10 text-muted/60" />
				<p class="mt-3 text-sm font-medium text-main">
					{locale.language === 'id'
						? 'Belum ada data pemakaian pengguna'
						: 'No user usage data yet'}
				</p>
				<p class="mt-1 text-xs text-muted">
					{locale.language === 'id'
						? 'Data konsumsi penyimpanan pengguna akan muncul di sini setelah ada berkas terunggah.'
						: 'User storage usage will appear here once files have been uploaded.'}
				</p>
			</Card.Content>
		</Card.Root>
	{:else}
		<Card.Root class="overflow-hidden rounded-xl border-subtle bg-surface py-0 shadow-none">
			<Card.Content class="p-0">
				<div class="divide-y divide-subtle">
					{#each users as user, idx (user.id)}
						{@const isSuperAdmin = user.role?.toLowerCase() === 'super_admin'}
						{@const isUnlimited = isSuperAdmin || user.quota_mode?.toUpperCase() === 'UNLIMITED'}
						{@const percentOfTotal =
							totalSystemUsedBytes > 0
								? ((user.used_bytes / totalSystemUsedBytes) * 100).toFixed(1)
								: null}
						<div
							class="flex flex-col gap-3 px-4 py-3 transition-colors hover:bg-black/5 sm:flex-row sm:items-center sm:justify-between dark:hover:bg-white/5"
						>
							<div class="flex min-w-0 items-center gap-3">
								<!-- Ranking number & Avatar -->
								<span class="w-5 text-center text-xs font-semibold text-muted">
									{idx + 1}
								</span>
								<div
									class={cn(
										'flex size-9 shrink-0 items-center justify-center rounded-full',
										isSuperAdmin ? 'bg-primary/15 text-primary' : 'bg-secondary text-main'
									)}
								>
									{#if isSuperAdmin}
										<ShieldCheck class="size-4" />
									{:else}
										<User class="size-4" />
									{/if}
								</div>

								<div class="min-w-0">
									<div class="flex flex-wrap items-center gap-2">
										<p class="truncate text-sm font-medium text-main">
											{user.username}
										</p>
										<Badge
											class={cn(
												'rounded-full border-0 px-2 py-0 text-[11px] font-medium',
												isSuperAdmin
													? 'bg-primary/15 text-primary hover:bg-primary/20'
													: 'bg-app text-muted hover:bg-app'
											)}
										>
											{isSuperAdmin
												? 'Super Admin'
												: locale.language === 'id'
													? 'Pengguna'
													: 'User'}
										</Badge>
										<Badge
											variant="outline"
											class="rounded-full border-subtle px-2 py-0 text-[11px] font-normal text-muted"
										>
											{isUnlimited
												? locale.language === 'id'
													? 'Tanpa batas'
													: 'Unlimited'
												: user.quota_gb
													? `${user.quota_gb} GB`
													: user.quota_bytes > 0
														? formatSize(user.quota_bytes)
														: locale.language === 'id'
															? 'Bawaan'
															: 'Default'}
										</Badge>
									</div>
									<p class="truncate text-xs text-muted">{user.email}</p>
								</div>
							</div>

							<!-- Storage usage stats -->
							<div
								class="flex shrink-0 items-center justify-between gap-3 pl-8 sm:justify-end sm:pl-0"
							>
								<div class="text-left sm:text-right">
									<p class="text-sm font-semibold text-main">
										{formatSize(user.used_bytes)}
									</p>
									<p class="text-xs text-muted">
										{#if percentOfTotal !== null}
											<span
												>{percentOfTotal}% {locale.language === 'id'
													? 'dari total berkas'
													: 'of total files'}</span
											>
										{:else}
											<span>{user.used_gb.toFixed(2)} GB</span>
										{/if}
									</p>
								</div>
							</div>
						</div>
					{/each}
				</div>
			</Card.Content>
		</Card.Root>
	{/if}

	{#if hasMore}
		<div
			bind:this={sentinelEl}
			class="flex items-center justify-center py-4"
			data-testid="infinite-scroll-sentinel"
		>
			{#if loadingMore}
				<div class="flex items-center gap-2 text-xs text-muted">
					<Loader2 class="size-4 animate-spin text-primary" />
					<span
						>{locale.language === 'id'
							? 'Memuat lebih banyak pengguna...'
							: 'Loading more users...'}</span
					>
				</div>
			{:else}
				<Button
					type="button"
					variant="ghost"
					size="sm"
					class="text-xs text-muted hover:text-main"
					onclick={() => void loadMore()}
				>
					{locale.language === 'id' ? 'Muat lebih banyak' : 'Load more'}
				</Button>
			{/if}
		</div>
	{/if}
</section>
