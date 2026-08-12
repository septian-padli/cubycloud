<script lang="ts">
	import type { FileEntry, ShareGrant, SharePermission } from '$lib/types/drive.js';
	import { getGrants, setGrants, shareableUsers } from '$lib/mocks/share-grants.js';
	import { createShareLink, shareUrl } from '$lib/mocks/share-links.js';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { toast } from 'svelte-sonner';
	import { cn } from '$lib/utils.js';
	import { Copy, Search, X } from '@lucide/svelte';

	type TabId = 'users' | 'link';
	type ShareCandidate = ReturnType<typeof shareableUsers>[number];

	let {
		open = $bindable(false),
		entry = null
	}: {
		open?: boolean;
		entry?: FileEntry | null;
	} = $props();

	let tab = $state<TabId>('users');
	let grants = $state<ShareGrant[]>([]);
	let query = $state('');
	let usePassword = $state(false);
	let password = $state('');
	let expiresInDays = $state('7');
	let createdUrl = $state('');

	const candidates = shareableUsers();

	const suggestions = $derived.by(() => {
		const q = query.trim().toLowerCase();
		if (!q) return [];
		const taken = new Set(grants.map((g) => g.userId));
		return candidates.filter(
			(u) =>
				!taken.has(u.userId) &&
				(u.displayName.toLowerCase().includes(q) || u.username.toLowerCase().includes(q))
		);
	});

	$effect(() => {
		if (open && entry) {
			tab = 'users';
			grants = getGrants(entry.id);
			query = '';
			usePassword = false;
			password = '';
			expiresInDays = '7';
			createdUrl = '';
		}
	});

	function addUser(user: ShareCandidate) {
		if (grants.some((g) => g.userId === user.userId)) return;
		grants = [
			...grants,
			{
				userId: user.userId,
				username: user.username,
				displayName: user.displayName,
				permission: 'viewer'
			}
		];
		query = '';
	}

	function removeUser(userId: string) {
		grants = grants.filter((g) => g.userId !== userId);
	}

	function setPermission(userId: string, permission: SharePermission) {
		grants = grants.map((g) => (g.userId === userId ? { ...g, permission } : g));
	}

	function saveUsers() {
		if (!entry) return;
		setGrants(entry.id, grants);
		const count = grants.length;
		toast.success(
			count === 0
				? `Akses pengguna untuk “${entry.name}” dikosongkan (mock)`
				: `Dibagikan ke ${count} pengguna (mock)`
		);
		open = false;
	}

	function submitLink() {
		if (!entry) return;
		if (usePassword && password.trim().length === 0) {
			toast.error('Masukkan kata sandi tautan.');
			return;
		}
		const days = Number(expiresInDays);
		const expiresAt = Number.isFinite(days)
			? new Date(Date.now() + days * 86_400_000).toISOString()
			: null;
		const record = createShareLink(entry, {
			access: 'public',
			password: usePassword ? password : null,
			expiresAt
		});
		createdUrl = `${window.location.origin}${shareUrl(record.token)}`;
		toast.success('Tautan berbagi dibuat (mock)', { description: entry.name });
	}

	async function copyUrl() {
		if (!createdUrl) return;
		try {
			await navigator.clipboard.writeText(createdUrl);
			toast.success('Tautan disalin');
		} catch {
			toast.error('Gagal menyalin tautan');
		}
	}
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" class="border-subtle bg-surface sm:max-w-md">
		<Sheet.Header>
			<Sheet.Title>Bagikan</Sheet.Title>
			<Sheet.Description>
				{entry?.name ?? 'Berkas'} — bagikan ke pengguna CubyCloud atau buat tautan.
			</Sheet.Description>
		</Sheet.Header>

		<div class="flex gap-1 px-4">
			<button
				type="button"
				class={cn(
					'flex-1 rounded-full px-3 py-2 text-sm font-medium transition-colors',
					tab === 'users' ? 'bg-primary text-primary-foreground' : 'text-muted hover:bg-primary/10'
				)}
				onclick={() => (tab = 'users')}
			>
				Pengguna
			</button>
			<button
				type="button"
				class={cn(
					'flex-1 rounded-full px-3 py-2 text-sm font-medium transition-colors',
					tab === 'link' ? 'bg-primary text-primary-foreground' : 'text-muted hover:bg-primary/10'
				)}
				onclick={() => (tab = 'link')}
			>
				Tautan
			</button>
		</div>

		{#if tab === 'users'}
			<div class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-4">
				<div class="relative">
					<Search
						class="pointer-events-none absolute start-3 top-1/2 size-4 -translate-y-1/2 stroke-2 text-muted"
					/>
					<Input
						id="share-user-search"
						bind:value={query}
						placeholder="Tambahkan orang"
						autocomplete="off"
						class="ps-9"
						aria-label="Cari pengguna CubyCloud"
					/>
					{#if query.trim().length > 0}
						<ul
							class="absolute z-10 mt-1 w-full overflow-hidden rounded-xl border border-subtle bg-surface"
							role="listbox"
						>
							{#each suggestions as user (user.userId)}
								<li>
									<button
										type="button"
										class="flex w-full flex-col px-3 py-2 text-left hover:bg-primary/10"
										onclick={() => addUser(user)}
									>
										<span class="truncate text-sm font-medium text-main">{user.displayName}</span>
										<span class="truncate text-xs text-muted">@{user.username}</span>
									</button>
								</li>
							{:else}
								<li class="px-3 py-2 text-xs text-muted">Tidak ada pengguna yang cocok.</li>
							{/each}
						</ul>
					{/if}
				</div>

				<div class="space-y-2">
					<p class="text-xs font-medium text-muted">Orang yang memiliki akses</p>
					{#if grants.length === 0}
						<p class="text-xs text-muted">Belum ada pengguna. Cari nama lalu klik untuk menambahkan.</p>
					{:else}
						{#each grants as grant (grant.userId)}
							<div class="rounded-xl border border-subtle bg-app px-3 py-3">
								<div class="flex items-start justify-between gap-2">
									<div class="min-w-0">
										<p class="truncate text-sm font-medium text-main">{grant.displayName}</p>
										<p class="truncate text-xs text-muted">@{grant.username}</p>
									</div>
									<Button
										type="button"
										variant="ghost"
										size="icon"
										class="text-muted hover:bg-primary hover:text-white"
										aria-label="Hapus {grant.displayName}"
										onclick={() => removeUser(grant.userId)}
									>
										<X class="size-4 stroke-2" />
									</Button>
								</div>
								<div class="mt-3 flex gap-1">
									<button
										type="button"
										class={cn(
											'rounded-full px-2.5 py-1 text-xs font-medium transition-colors',
											grant.permission === 'viewer'
												? 'bg-secondary text-main'
												: 'text-muted hover:bg-primary/10'
										)}
										onclick={() => setPermission(grant.userId, 'viewer')}
									>
										Viewer
									</button>
									<button
										type="button"
										class={cn(
											'rounded-full px-2.5 py-1 text-xs font-medium transition-colors',
											grant.permission === 'editor'
												? 'bg-primary/15 text-primary'
												: 'text-muted hover:bg-primary/10'
										)}
										onclick={() => setPermission(grant.userId, 'editor')}
									>
										Editor
									</button>
								</div>
							</div>
						{/each}
					{/if}
				</div>
			</div>
			<Sheet.Footer>
				<Button type="button" variant="secondary" onclick={() => (open = false)}>Batal</Button>
				<Button type="button" onclick={saveUsers}>Simpan</Button>
			</Sheet.Footer>
		{:else}
			<div class="flex flex-1 flex-col gap-5 px-4">
				<div class="flex items-center justify-between gap-3">
					<div>
						<p class="text-sm font-medium text-main">Lindungi kata sandi</p>
						<p class="text-xs text-muted">Opsional. Siapa saja dengan tautan dapat mengakses.</p>
					</div>
					<Switch bind:checked={usePassword} aria-label="Aktifkan kata sandi" />
				</div>

				{#if usePassword}
					<div class="grid gap-2">
						<Label for="share-password">Kata sandi</Label>
						<Input
							id="share-password"
							type="password"
							bind:value={password}
							placeholder="••••••••"
						/>
					</div>
				{/if}

				<div class="grid gap-2">
					<Label for="share-expiry">Kedaluwarsa (hari)</Label>
					<select
						id="share-expiry"
						bind:value={expiresInDays}
						class="h-9 w-full rounded-full border border-subtle bg-surface px-3 text-sm text-main shadow-none outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
					>
						<option value="1">1 hari</option>
						<option value="7">7 hari</option>
						<option value="30">30 hari</option>
						<option value="90">90 hari</option>
					</select>
				</div>

				{#if createdUrl}
					<div class="grid gap-2">
						<Label for="share-url">Tautan</Label>
						<div class="flex gap-2">
							<Input id="share-url" value={createdUrl} readonly class="text-muted" />
							<Button
								type="button"
								variant="secondary"
								size="icon"
								onclick={copyUrl}
								aria-label="Salin tautan"
							>
								<Copy class="size-4 stroke-2" />
							</Button>
						</div>
					</div>
				{/if}
			</div>
			<Sheet.Footer>
				<Button type="button" variant="secondary" onclick={() => (open = false)}>Batal</Button>
				<Button type="button" onclick={submitLink}>
					{createdUrl ? 'Buat tautan baru' : 'Buat tautan'}
				</Button>
			</Sheet.Footer>
		{/if}
	</Sheet.Content>
</Sheet.Root>
