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
				toast.error(`Gagal memuat pengguna terbesar: ${res.error}`);
			}
		} catch (error) {
			toast.error(error instanceof Error ? error.message : 'Gagal memuat pengguna');
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
				toast.error(`Gagal memuat halaman berikutnya: ${res.error}`);
			}
		} catch (error) {
			toast.error(error instanceof Error ? error.message : 'Gagal memuat halaman berikutnya');
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
			<h2 class="text-sm font-semibold text-main">Pengguna dengan konsumsi terbesar</h2>
			<p class="text-xs text-muted">
				Diurutkan berdasarkan total ukuran penyimpanan berkas pengguna.
			</p>
		</div>
		<Button
			variant="outline"
			size="sm"
			href="/admin/users"
			class="h-8 gap-1.5 text-xs text-muted hover:text-main"
		>
			<span>Kelola Users & Quotas</span>
			<ArrowUpRight class="size-3.5" />
		</Button>
	</div>

	{#if loading && users.length === 0}
		<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
			<Card.Content class="p-6">
				<div class="space-y-4">
					{#each Array(4) as _}
						<div class="flex items-center justify-between gap-4 animate-pulse">
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
				<p class="mt-3 text-sm font-medium text-main">Belum ada data pemakaian pengguna</p>
				<p class="mt-1 text-xs text-muted">
					Data konsumsi penyimpanan pengguna akan muncul di sini setelah ada berkas terunggah.
				</p>
			</Card.Content>
		</Card.Root>
	{:else}
		<Card.Root class="overflow-hidden rounded-xl border-subtle bg-surface py-0 shadow-none">
			<Card.Content class="p-0">
				<div class="divide-y divide-subtle">
					{#each users as user, idx (user.id)}
						{@const isSuperAdmin =
							user.role?.toLowerCase() === 'super_admin'}
						{@const isUnlimited =
							isSuperAdmin ||
							user.quota_mode?.toUpperCase() === 'UNLIMITED'}
						{@const percentOfTotal =
							totalSystemUsedBytes > 0
								? ((user.used_bytes / totalSystemUsedBytes) * 100).toFixed(1)
								: null}
						<div
							class="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center sm:justify-between transition-colors hover:bg-black/5 dark:hover:bg-white/5"
						>
							<div class="flex min-w-0 items-center gap-3">
								<!-- Ranking number & Avatar -->
								<span class="w-5 text-center text-xs font-semibold text-muted">
									{idx + 1}
								</span>
								<div
									class={cn(
										'flex size-9 shrink-0 items-center justify-center rounded-full',
										isSuperAdmin
											? 'bg-primary/15 text-primary'
											: 'bg-secondary text-main'
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
												'rounded-full px-2 py-0 text-[11px] font-medium border-0',
												isSuperAdmin
													? 'bg-primary/15 text-primary hover:bg-primary/20'
													: 'bg-app text-muted hover:bg-app'
											)}
										>
											{isSuperAdmin ? 'Super Admin' : 'User'}
										</Badge>
										<Badge
											variant="outline"
											class="rounded-full border-subtle px-2 py-0 text-[11px] font-normal text-muted"
										>
											{isUnlimited
												? 'Unlimited'
												: user.quota_gb
													? `${user.quota_gb} GB`
													: user.quota_bytes > 0
														? formatSize(user.quota_bytes)
														: 'Default'}
										</Badge>
									</div>
									<p class="truncate text-xs text-muted">{user.email}</p>
								</div>
							</div>

							<!-- Storage usage stats -->
							<div class="flex items-center justify-between sm:justify-end gap-3 shrink-0 pl-8 sm:pl-0">
								<div class="text-left sm:text-right">
									<p class="text-sm font-semibold text-main">
										{formatSize(user.used_bytes)}
									</p>
									<p class="text-xs text-muted">
										{#if percentOfTotal !== null}
											<span>{percentOfTotal}% dari total berkas</span>
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
					<span>Memuat lebih banyak pengguna...</span>
				</div>
			{:else}
				<Button
					type="button"
					variant="ghost"
					size="sm"
					class="text-xs text-muted hover:text-main"
					onclick={() => void loadMore()}
				>
					Muat lebih banyak
				</Button>
			{/if}
		</div>
	{/if}
</section>
