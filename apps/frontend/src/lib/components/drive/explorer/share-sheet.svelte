<script lang="ts">
	import type { FileEntry } from '$lib/types/drive.js';
	import {
		searchShareUsers,
		getItemShares,
		updateItemGrants,
		deleteUserGrant,
		createShareLink,
		deleteShareLink,
		type SharePermission,
		type ShareUserItem,
		type ShareGrantItem,
		type ShareLinkItem
	} from '$lib/api/share.js';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import * as Tabs from '$lib/components/ui/tabs/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import { toast } from 'svelte-sonner';
	import { cn } from '$lib/utils.js';
	import { Copy, Lock, Loader2, Search, Trash2, X } from '@lucide/svelte';

	type TabId = 'users' | 'link';

	let {
		open = $bindable(false),
		entry = null
	}: {
		open?: boolean;
		entry?: FileEntry | null;
	} = $props();

	let tab = $state<TabId>('users');
	let grants = $state<ShareGrantItem[]>([]);
	let links = $state<ShareLinkItem[]>([]);
	let loading = $state(false);
	let saving = $state(false);
	let creatingLink = $state(false);

	let query = $state('');
	let userSuggestions = $state<ShareUserItem[]>([]);
	let searchingUsers = $state(false);

	let usePassword = $state(false);
	let password = $state('');
	let expiresInDays = $state('7');
	let linkPermission = $state<SharePermission>('viewer');
	let createdUrl = $state('');

	let searchTimer: ReturnType<typeof setTimeout> | null = null;

	$effect(() => {
		const q = query.trim();
		if (searchTimer) clearTimeout(searchTimer);
		if (!q) {
			userSuggestions = [];
			searchingUsers = false;
			return;
		}
		searchingUsers = true;
		searchTimer = setTimeout(async () => {
			try {
				const users = await searchShareUsers(q);
				const taken = new Set(grants.map((g) => g.user_id));
				userSuggestions = users.filter((u) => !taken.has(u.id));
			} catch {
				userSuggestions = [];
			} finally {
				searchingUsers = false;
			}
		}, 250);
	});

	$effect(() => {
		if (open && entry) {
			tab = 'users';
			query = '';
			userSuggestions = [];
			usePassword = false;
			password = '';
			expiresInDays = '7';
			linkPermission = 'viewer';
			createdUrl = '';
			void loadShares();
		}
	});

	async function loadShares() {
		if (!entry) return;
		loading = true;
		try {
			const res = await getItemShares(entry.id);
			grants = res.grants ?? [];
			links = res.links ?? [];
		} catch (err) {
			console.error('Failed to load item shares', err);
			toast.error('Gagal memuat daftar berbagi.');
		} finally {
			loading = false;
		}
	}

	function addUser(user: ShareUserItem) {
		if (grants.some((g) => g.user_id === user.id)) return;
		grants = [
			...grants,
			{
				id: '',
				user_id: user.id,
				username: user.username,
				email: user.email,
				permission: 'viewer'
			}
		];
		query = '';
		userSuggestions = [];
	}

	async function removeUser(userId: string) {
		grants = grants.filter((g) => g.user_id !== userId);
		if (entry) {
			try {
				await deleteUserGrant(entry.id, userId);
			} catch {
				// Handled on saveUsers sync
			}
		}
	}

	function setPermission(userId: string, permission: SharePermission) {
		grants = grants.map((g) => (g.user_id === userId ? { ...g, permission } : g));
	}

	async function saveUsers() {
		if (!entry) return;
		saving = true;
		try {
			const payload = grants.map((g) => ({ user_id: g.user_id, permission: g.permission }));
			const res = await updateItemGrants(entry.id, payload);
			grants = res.grants ?? [];
			const count = grants.length;
			toast.success(
				count === 0
					? `Akses pengguna untuk “${entry.name}” dikosongkan`
					: `Hak akses untuk “${entry.name}” berhasil disimpan`
			);
			open = false;
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Gagal menyimpan akses pengguna.');
		} finally {
			saving = false;
		}
	}

	async function submitLink() {
		if (!entry) return;
		if (usePassword && password.trim().length === 0) {
			toast.error('Masukkan kata sandi tautan.');
			return;
		}
		creatingLink = true;
		try {
			const days = expiresInDays === 'never' ? null : Number(expiresInDays);
			const res = await createShareLink(entry.id, {
				permission: linkPermission,
				password: usePassword ? password.trim() : undefined,
				expires_days: days
			});
			const fullUrl = `${window.location.origin}/s/${res.link.token}`;
			createdUrl = fullUrl;
			links = [res.link, ...links.filter((l) => l.id !== res.link.id)];
			password = '';
			usePassword = false;
			toast.success('Tautan berbagi dibuat');
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Gagal membuat tautan.');
		} finally {
			creatingLink = false;
		}
	}

	async function removeLink(linkId: string) {
		if (!entry) return;
		try {
			await deleteShareLink(entry.id, linkId);
			links = links.filter((l) => l.id !== linkId);
			toast.success('Tautan berhasil dicabut');
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Gagal mencabut tautan.');
		}
	}

	async function copyUrl(urlToCopy: string) {
		if (!urlToCopy) return;
		try {
			await navigator.clipboard.writeText(urlToCopy);
			toast.success('Tautan disalin ke clipboard');
		} catch {
			toast.error('Gagal menyalin tautan');
		}
	}

	function formatExpiry(iso: string | null): string {
		if (!iso) return 'Tanpa batas waktu';
		try {
			const d = new Date(iso);
			if (isNaN(d.getTime())) return iso;
			return `Kedaluwarsa ${d.toLocaleDateString('id-ID', {
				day: 'numeric',
				month: 'short',
				year: 'numeric'
			})}`;
		} catch {
			return iso;
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

		<Tabs.Root bind:value={tab} class="flex min-h-0 flex-1 flex-col">
			<div class="px-4">
				<Tabs.List variant="line" class="grid w-full grid-cols-2">
					<Tabs.Trigger value="users">Pengguna</Tabs.Trigger>
					<Tabs.Trigger value="link">Tautan</Tabs.Trigger>
				</Tabs.List>
			</div>

			{#if loading}
				<div class="flex flex-1 items-center justify-center py-12 text-muted">
					<Loader2 class="size-6 animate-spin text-primary" />
					<span class="ms-2 text-sm font-medium">Memuat info berbagi...</span>
				</div>
			{:else}
				<Tabs.Content value="users" class="flex min-h-0 flex-1 flex-col">
					<div class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-4">
						<div class="relative">
							<Search
								class="pointer-events-none absolute start-3 top-1/2 size-4 -translate-y-1/2 stroke-2 text-muted"
							/>
							<Input
								id="share-user-search"
								bind:value={query}
								placeholder="Tambahkan pengguna (nama atau email)"
								autocomplete="off"
								class="ps-9"
								aria-label="Cari pengguna CubyCloud"
							/>
							{#if searchingUsers}
								<div class="absolute end-3 top-1/2 -translate-y-1/2">
									<Loader2 class="size-4 animate-spin text-muted" />
								</div>
							{/if}
							{#if query.trim().length > 0}
								<ul
									class="absolute z-10 mt-1 max-h-48 w-full overflow-y-auto rounded-xl border border-subtle bg-surface shadow-md"
									role="listbox"
								>
									{#each userSuggestions as user (user.id)}
										<li>
											<button
												type="button"
												class="flex w-full flex-col px-3 py-2 text-left hover:bg-primary/10"
												onclick={() => addUser(user)}
											>
												<span class="truncate text-sm font-medium text-main">{user.username}</span>
												<span class="truncate text-xs text-muted">{user.email}</span>
											</button>
										</li>
									{:else}
										{#if !searchingUsers}
											<li class="px-3 py-2 text-xs text-muted">Tidak ada pengguna yang cocok.</li>
										{/if}
									{/each}
								</ul>
							{/if}
						</div>

						<div class="space-y-2">
							<p class="text-xs font-medium text-muted">Pengguna dengan akses ({grants.length})</p>
							{#if grants.length === 0}
								<p class="text-xs text-muted">
									Belum ada pengguna. Cari nama lalu klik untuk menambahkan.
								</p>
							{:else}
								{#each grants as grant (grant.user_id)}
									<div class="rounded-xl border border-subtle bg-app p-3">
										<div class="flex items-center justify-between gap-3">
											<div class="min-w-0 flex-1">
												<p class="truncate text-sm font-medium text-main">{grant.username}</p>
												<p class="truncate text-xs text-muted">{grant.email}</p>
											</div>
											<div class="flex shrink-0 items-center gap-1.5">
												<Select.Root
													type="single"
													value={grant.permission}
													onValueChange={(val) => {
														if (val === 'viewer' || val === 'editor') {
															setPermission(grant.user_id, val);
														}
													}}
												>
													<Select.Trigger
														aria-label="Pilih hak akses {grant.username}"
														class="h-8 w-24 rounded-lg border-subtle bg-surface text-xs text-main"
													>
														<span>{grant.permission === 'editor' ? 'Ubah' : 'Lihat'}</span>
													</Select.Trigger>
													<Select.Content class="border-subtle bg-surface">
														<Select.Item value="viewer" label="Lihat">Lihat</Select.Item>
														<Select.Item value="editor" label="Ubah">Ubah</Select.Item>
													</Select.Content>
												</Select.Root>
												<Button
													type="button"
													variant="ghost"
													size="icon"
													class="size-8 text-muted hover:bg-danger/10 hover:text-danger"
													aria-label="Hapus {grant.username}"
													onclick={() => removeUser(grant.user_id)}
												>
													<X class="size-4 stroke-2" />
												</Button>
											</div>
										</div>
									</div>
								{/each}
							{/if}
						</div>
					</div>
					<Sheet.Footer>
						<Button type="button" variant="secondary" onclick={() => (open = false)}>Batal</Button>
						<Button type="button" onclick={saveUsers} disabled={saving}>
							{#if saving}
								<Loader2 class="size-4 animate-spin stroke-2" />
							{/if}
							Simpan
						</Button>
					</Sheet.Footer>
				</Tabs.Content>
				<Tabs.Content value="link" class="flex min-h-0 flex-1 flex-col">
					<div class="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-4">
						<div class="space-y-4 rounded-xl border border-subtle bg-app p-4">
							<p class="text-xs font-semibold tracking-wider text-muted uppercase">
								Buat Tautan Baru
							</p>

							<div class="grid gap-2">
								<Label for="share-permission">Hak Akses</Label>
								<Select.Root type="single" bind:value={linkPermission}>
									<Select.Trigger id="share-permission">
										<span
											>{linkPermission === 'editor'
												? 'Editor (Bisa Ubah & Unggah)'
												: 'Viewer (Hanya Lihat & Unduh)'}</span
										>
									</Select.Trigger>
									<Select.Content>
										<Select.Item value="viewer" label="Viewer (Hanya Lihat & Unduh)">
											Viewer (Hanya Lihat & Unduh)
										</Select.Item>
										<Select.Item value="editor" label="Editor (Bisa Ubah & Unggah)">
											Editor (Bisa Ubah & Unggah)
										</Select.Item>
									</Select.Content>
								</Select.Root>
							</div>

							<div class="flex items-center justify-between gap-3">
								<div>
									<p class="text-sm font-medium text-main">Lindungi kata sandi</p>
									<p class="text-xs text-muted">Pengunjung harus memasukkan sandi untuk membuka.</p>
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
								<Label for="share-expiry">Masa Berlaku Kedaluwarsa</Label>
								<Select.Root type="single" bind:value={expiresInDays}>
									<Select.Trigger id="share-expiry">
										<span
											>{expiresInDays === 'never'
												? 'Tanpa kedaluwarsa'
												: `${expiresInDays} hari`}</span
										>
									</Select.Trigger>
									<Select.Content>
										<Select.Item value="1" label="1 hari">1 hari</Select.Item>
										<Select.Item value="7" label="7 hari">7 hari</Select.Item>
										<Select.Item value="30" label="30 hari">30 hari</Select.Item>
										<Select.Item value="90" label="90 hari">90 hari</Select.Item>
										<Select.Item value="never" label="Tanpa kedaluwarsa"
											>Tanpa kedaluwarsa</Select.Item
										>
									</Select.Content>
								</Select.Root>
							</div>

							<Button type="button" class="w-full" onclick={submitLink} disabled={creatingLink}>
								{#if creatingLink}
									<Loader2 class="size-4 animate-spin stroke-2" />
								{/if}
								Buat tautan
							</Button>
						</div>

						{#if createdUrl}
							<div class="grid gap-2 rounded-xl border border-primary/20 bg-primary/5 p-3">
								<Label for="share-url" class="text-xs font-semibold text-primary"
									>Tautan baru berhasil dibuat</Label
								>
								<div class="flex gap-2">
									<Input
										id="share-url"
										value={createdUrl}
										readonly
										class="bg-surface text-xs text-main"
									/>
									<Button
										type="button"
										variant="secondary"
										size="icon"
										onclick={() => copyUrl(createdUrl)}
										aria-label="Salin tautan baru"
									>
										<Copy class="size-4 stroke-2" />
									</Button>
								</div>
							</div>
						{/if}

						<div class="space-y-3 pb-4">
							<p class="text-xs font-medium text-muted">Daftar tautan aktif ({links.length})</p>
							{#if links.length === 0}
								<p class="text-xs text-muted">Belum ada tautan aktif untuk berkas/folder ini.</p>
							{:else}
								<div class="space-y-2">
									{#each links as link (link.id)}
										{@const linkUrl = `${typeof window !== 'undefined' ? window.location.origin : ''}/s/${link.token}`}
										<div class="rounded-xl border border-subtle bg-app p-3">
											<div class="flex items-start justify-between gap-2">
												<div class="min-w-0 flex-1">
													<div class="flex items-center gap-2">
														<span
															class={cn(
																'rounded-full px-2 py-0.5 text-[11px] font-medium',
																link.permission === 'editor'
																	? 'bg-primary/15 text-primary'
																	: 'bg-secondary text-main'
															)}
														>
															{link.permission === 'editor' ? 'Editor' : 'Viewer'}
														</span>
														{#if link.has_password}
															<span class="inline-flex items-center gap-1 text-[11px] text-muted">
																<Lock class="size-3" /> Sandi
															</span>
														{/if}
													</div>
													<p class="mt-1 truncate font-mono text-xs text-muted">/s/{link.token}</p>
													<p class="mt-0.5 text-[11px] text-muted">
														{formatExpiry(link.expires_at)}
													</p>
												</div>
												<div class="flex items-center gap-1">
													<Button
														type="button"
														variant="ghost"
														size="icon"
														class="size-8 text-muted hover:bg-primary/10 hover:text-main"
														aria-label="Salin tautan"
														onclick={() => copyUrl(linkUrl)}
													>
														<Copy class="size-3.5 stroke-2" />
													</Button>
													<Button
														type="button"
														variant="ghost"
														size="icon"
														class="size-8 text-muted hover:bg-danger/10 hover:text-danger"
														aria-label="Cabut tautan"
														onclick={() => removeLink(link.id)}
													>
														<Trash2 class="size-3.5 stroke-2" />
													</Button>
												</div>
											</div>
										</div>
									{/each}
								</div>
							{/if}
						</div>
					</div>
					<Sheet.Footer>
						<Button type="button" variant="secondary" onclick={() => (open = false)}>Tutup</Button>
					</Sheet.Footer>
				</Tabs.Content>
			{/if}
		</Tabs.Root>
	</Sheet.Content>
</Sheet.Root>
