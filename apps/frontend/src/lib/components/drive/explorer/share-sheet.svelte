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
	import { locale } from '$lib/locale.svelte.js';

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
			toast.error(
				locale.language === 'id'
					? 'Gagal memuat daftar berbagi.'
					: 'Unable to load sharing details.'
			);
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
					? locale.language === 'id'
						? `Akses pengguna untuk “${entry.name}” dikosongkan`
						: `User access for “${entry.name}” was cleared`
					: locale.language === 'id'
						? `Hak akses untuk “${entry.name}” berhasil disimpan`
						: `Access permissions for “${entry.name}” were saved`
			);
			open = false;
		} catch (err) {
			toast.error(
				err instanceof Error
					? err.message
					: locale.language === 'id'
						? 'Gagal menyimpan akses pengguna.'
						: 'Unable to save user access.'
			);
		} finally {
			saving = false;
		}
	}

	async function submitLink() {
		if (!entry) return;
		if (usePassword && password.trim().length === 0) {
			toast.error(
				locale.language === 'id' ? 'Masukkan kata sandi tautan.' : 'Enter a link password.'
			);
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
			toast.success(locale.language === 'id' ? 'Tautan berbagi dibuat' : 'Share link created');
		} catch (err) {
			toast.error(
				err instanceof Error
					? err.message
					: locale.language === 'id'
						? 'Gagal membuat tautan.'
						: 'Unable to create link.'
			);
		} finally {
			creatingLink = false;
		}
	}

	async function removeLink(linkId: string) {
		if (!entry) return;
		try {
			await deleteShareLink(entry.id, linkId);
			links = links.filter((l) => l.id !== linkId);
			toast.success(locale.language === 'id' ? 'Tautan berhasil dicabut' : 'Link revoked');
		} catch (err) {
			toast.error(
				err instanceof Error
					? err.message
					: locale.language === 'id'
						? 'Gagal mencabut tautan.'
						: 'Unable to revoke link.'
			);
		}
	}

	async function copyUrl(urlToCopy: string) {
		if (!urlToCopy) return;
		try {
			await navigator.clipboard.writeText(urlToCopy);
			toast.success(
				locale.language === 'id' ? 'Tautan disalin ke clipboard' : 'Link copied to clipboard'
			);
		} catch {
			toast.error(locale.language === 'id' ? 'Gagal menyalin tautan' : 'Unable to copy link');
		}
	}

	function formatExpiry(iso: string | null): string {
		if (!iso) return locale.language === 'id' ? 'Tanpa batas waktu' : 'No expiration';
		try {
			const d = new Date(iso);
			if (isNaN(d.getTime())) return iso;
			const date = d.toLocaleDateString(locale.language === 'id' ? 'id-ID' : 'en-US', {
				day: 'numeric',
				month: 'short',
				year: 'numeric'
			});
			return locale.language === 'id' ? `Kedaluwarsa ${date}` : `Expires ${date}`;
		} catch {
			return iso;
		}
	}
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" class="border-subtle bg-surface sm:max-w-md">
		<Sheet.Header>
			<Sheet.Title>{locale.language === 'id' ? 'Bagikan' : 'Share'}</Sheet.Title>
			<Sheet.Description>
				{entry?.name ?? (locale.language === 'id' ? 'Berkas' : 'File')} — {locale.language === 'id'
					? 'bagikan ke pengguna CubyCloud atau buat tautan.'
					: 'share with CubyCloud users or create a link.'}
			</Sheet.Description>
		</Sheet.Header>

		<Tabs.Root bind:value={tab} class="flex min-h-0 flex-1 flex-col">
			<div class="px-4">
				<Tabs.List variant="line" class="grid w-full grid-cols-2">
					<Tabs.Trigger value="users"
						>{locale.language === 'id' ? 'Pengguna' : 'Users'}</Tabs.Trigger
					>
					<Tabs.Trigger value="link">{locale.language === 'id' ? 'Tautan' : 'Link'}</Tabs.Trigger>
				</Tabs.List>
			</div>

			{#if loading}
				<div class="flex flex-1 items-center justify-center py-12 text-muted">
					<Loader2 class="size-6 animate-spin text-primary" />
					<span class="ms-2 text-sm font-medium"
						>{locale.language === 'id'
							? 'Memuat info berbagi...'
							: 'Loading sharing details...'}</span
					>
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
								placeholder={locale.language === 'id'
									? 'Tambahkan pengguna (nama atau email)'
									: 'Add users (name or email)'}
								autocomplete="off"
								class="ps-9"
								aria-label={locale.language === 'id'
									? 'Cari pengguna CubyCloud'
									: 'Search CubyCloud users'}
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
											<li class="px-3 py-2 text-xs text-muted">
												{locale.language === 'id'
													? 'Tidak ada pengguna yang cocok.'
													: 'No matching users.'}
											</li>
										{/if}
									{/each}
								</ul>
							{/if}
						</div>

						<div class="space-y-2">
							<p class="text-xs font-medium text-muted">
								{locale.language === 'id'
									? `Pengguna dengan akses (${grants.length})`
									: `Users with access (${grants.length})`}
							</p>
							{#if grants.length === 0}
								<p class="text-xs text-muted">
									{locale.language === 'id'
										? 'Belum ada pengguna. Cari nama lalu klik untuk menambahkan.'
										: 'No users yet. Search for a name and click to add them.'}
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
														aria-label={locale.language === 'id'
															? `Pilih hak akses ${grant.username}`
															: `Select access for ${grant.username}`}
														class="h-8 w-24 rounded-lg border-subtle bg-surface text-xs text-main"
													>
														<span
															>{grant.permission === 'editor'
																? locale.language === 'id'
																	? 'Ubah'
																	: 'Edit'
																: locale.language === 'id'
																	? 'Lihat'
																	: 'View'}</span
														>
													</Select.Trigger>
													<Select.Content class="border-subtle bg-surface">
														<Select.Item
															value="viewer"
															label={locale.language === 'id' ? 'Lihat' : 'View'}
															>{locale.language === 'id' ? 'Lihat' : 'View'}</Select.Item
														>
														<Select.Item
															value="editor"
															label={locale.language === 'id' ? 'Ubah' : 'Edit'}
															>{locale.language === 'id' ? 'Ubah' : 'Edit'}</Select.Item
														>
													</Select.Content>
												</Select.Root>
												<Button
													type="button"
													variant="ghost"
													size="icon"
													class="size-8 text-muted hover:bg-danger/10 hover:text-danger"
													aria-label={locale.language === 'id'
														? `Hapus ${grant.username}`
														: `Remove ${grant.username}`}
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
						<Button type="button" variant="secondary" onclick={() => (open = false)}
							>{locale.language === 'id' ? 'Batal' : 'Cancel'}</Button
						>
						<Button type="button" onclick={saveUsers} disabled={saving}>
							{#if saving}<Loader2 class="size-4 animate-spin stroke-2" />{/if}
							{locale.language === 'id' ? 'Simpan' : 'Save'}
						</Button>
					</Sheet.Footer>
				</Tabs.Content>
				<Tabs.Content value="link" class="flex min-h-0 flex-1 flex-col">
					<div class="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-4">
						<div class="space-y-4 rounded-xl border border-subtle bg-app p-4">
							<p class="text-xs font-semibold tracking-wider text-muted uppercase">
								{locale.language === 'id' ? 'Buat Tautan Baru' : 'Create a new link'}
							</p>

							<div class="grid gap-2">
								<Label for="share-permission"
									>{locale.language === 'id' ? 'Hak Akses' : 'Access'}</Label
								>
								<Select.Root type="single" bind:value={linkPermission}>
									<Select.Trigger id="share-permission">
										<span
											>{linkPermission === 'editor'
												? locale.language === 'id'
													? 'Editor (Bisa Ubah & Unggah)'
													: 'Editor (Can edit & upload)'
												: locale.language === 'id'
													? 'Viewer (Hanya Lihat & Unduh)'
													: 'Viewer (View & download only)'}</span
										>
									</Select.Trigger>
									<Select.Content>
										<Select.Item
											value="viewer"
											label={locale.language === 'id'
												? 'Viewer (Hanya Lihat & Unduh)'
												: 'Viewer (View & download only)'}
											>{locale.language === 'id'
												? 'Viewer (Hanya Lihat & Unduh)'
												: 'Viewer (View & download only)'}</Select.Item
										>
										<Select.Item
											value="editor"
											label={locale.language === 'id'
												? 'Editor (Bisa Ubah & Unggah)'
												: 'Editor (Can edit & upload)'}
											>{locale.language === 'id'
												? 'Editor (Bisa Ubah & Unggah)'
												: 'Editor (Can edit & upload)'}</Select.Item
										>
									</Select.Content>
								</Select.Root>
							</div>

							<div class="flex items-center justify-between gap-3">
								<div>
									<p class="text-sm font-medium text-main">
										{locale.language === 'id'
											? 'Lindungi dengan kata sandi'
											: 'Password protection'}
									</p>
									<p class="text-xs text-muted">
										{locale.language === 'id'
											? 'Pengunjung harus memasukkan sandi untuk membuka.'
											: 'Visitors must enter a password to open the link.'}
									</p>
								</div>
								<Switch
									bind:checked={usePassword}
									aria-label={locale.language === 'id'
										? 'Aktifkan kata sandi'
										: 'Enable password protection'}
								/>
							</div>

							{#if usePassword}
								<div class="grid gap-2">
									<Label for="share-password"
										>{locale.language === 'id' ? 'Kata sandi' : 'Password'}</Label
									>
									<Input
										id="share-password"
										type="password"
										bind:value={password}
										placeholder="••••••••"
									/>
								</div>
							{/if}

							<div class="grid gap-2">
								<Label for="share-expiry"
									>{locale.language === 'id' ? 'Masa berlaku' : 'Expiration'}</Label
								>
								<Select.Root type="single" bind:value={expiresInDays}>
									<Select.Trigger id="share-expiry">
										<span
											>{expiresInDays === 'never'
												? locale.language === 'id'
													? 'Tanpa kedaluwarsa'
													: 'Never expires'
												: locale.language === 'id'
													? `${expiresInDays} hari`
													: `${expiresInDays} days`}</span
										>
									</Select.Trigger>
									<Select.Content>
										<Select.Item value="1" label={locale.language === 'id' ? '1 hari' : '1 day'}
											>{locale.language === 'id' ? '1 hari' : '1 day'}</Select.Item
										>
										<Select.Item value="7" label={locale.language === 'id' ? '7 hari' : '7 days'}
											>{locale.language === 'id' ? '7 hari' : '7 days'}</Select.Item
										>
										<Select.Item value="30" label={locale.language === 'id' ? '30 hari' : '30 days'}
											>{locale.language === 'id' ? '30 hari' : '30 days'}</Select.Item
										>
										<Select.Item value="90" label={locale.language === 'id' ? '90 hari' : '90 days'}
											>{locale.language === 'id' ? '90 hari' : '90 days'}</Select.Item
										>
										<Select.Item
											value="never"
											label={locale.language === 'id' ? 'Tanpa kedaluwarsa' : 'Never expires'}
											>{locale.language === 'id'
												? 'Tanpa kedaluwarsa'
												: 'Never expires'}</Select.Item
										>
									</Select.Content>
								</Select.Root>
							</div>

							<Button type="button" class="w-full" onclick={submitLink} disabled={creatingLink}>
								{#if creatingLink}
									<Loader2 class="size-4 animate-spin stroke-2" />
								{/if}
								{locale.language === 'id' ? 'Buat tautan' : 'Create link'}
							</Button>
						</div>

						{#if createdUrl}
							<div class="grid gap-2 rounded-xl border border-primary/20 bg-primary/5 p-3">
								<Label for="share-url" class="text-xs font-semibold text-primary"
									>{locale.language === 'id'
										? 'Tautan baru berhasil dibuat'
										: 'New link created successfully'}</Label
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
										aria-label={locale.language === 'id' ? 'Salin tautan baru' : 'Copy new link'}
									>
										<Copy class="size-4 stroke-2" />
									</Button>
								</div>
							</div>
						{/if}

						<div class="space-y-3 pb-4">
							<p class="text-xs font-medium text-muted">
								{locale.language === 'id'
									? `Daftar tautan aktif (${links.length})`
									: `Active links (${links.length})`}
							</p>
							{#if links.length === 0}
								<p class="text-xs text-muted">
									{locale.language === 'id'
										? 'Belum ada tautan aktif untuk berkas/folder ini.'
										: 'There are no active links for this file or folder.'}
								</p>
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
																<Lock class="size-3" />
																{locale.language === 'id' ? 'Sandi' : 'Password'}
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
														aria-label={locale.language === 'id' ? 'Salin tautan' : 'Copy link'}
														onclick={() => copyUrl(linkUrl)}
													>
														<Copy class="size-3.5 stroke-2" />
													</Button>
													<Button
														type="button"
														variant="ghost"
														size="icon"
														class="size-8 text-muted hover:bg-danger/10 hover:text-danger"
														aria-label={locale.language === 'id' ? 'Cabut tautan' : 'Revoke link'}
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
						<Button type="button" variant="secondary" onclick={() => (open = false)}
							>{locale.language === 'id' ? 'Tutup' : 'Close'}</Button
						>
					</Sheet.Footer>
				</Tabs.Content>
			{/if}
		</Tabs.Root>
	</Sheet.Content>
</Sheet.Root>
