# DESIGN.md — CubyCloud Frontend Design System

> Source of truth for **visual tokens, component appearance, interaction styling, and accessibility**. Product behavior belongs to `docs/prd.md`; routes, page actions, and access contracts belong to [`SITEMAP.md`](./SITEMAP.md). Read the sections relevant to the UI being changed instead of rereading the full document.

## 1. Design principles

- **Flat Earthy:** warm cream and olive/forest tones, Montserrat typography, clean WhatsApp-inspired spatial rhythm **without** chat-specific ornamentation.
- **100% flat surfaces:** no Tailwind shadows, glassmorphism, floating/decorative gradients, or aggressive borders. Establish hierarchy through `bg-app`/`bg-surface` layering and `border-subtle` dividers; `shadow-none` also applies to modal, sheet, dropdown, toast, controls, and cards.
- **Storage-first:** efficient metadata scanning. File list is the default view; use the existing List/Grid toggle and grid behavior where provided rather than introducing another view mode.
- **Tailwind CSS v4:** use standard utilities and project tokens; **no arbitrary-value classes** (`w-[13px]`, `p-[14px]`, `text-[15px]`, `bg-[#...]`). Genuine new tokens belong in `src/routes/layout.css` and this document.
- **Dark and light:** always honor the corresponding palette and contrast; avoid hard-coded visual colors in components.

## 2. Color tokens

Defined in `src/routes/layout.css` (`@layer base` root/dark CSS variables mapped with `@theme inline`). The source CSS is the implementation authority; keep it consistent with this palette.

| Functional token / Tailwind utility | Light | Dark | Use |
| --- | --- | --- | --- |
| `--bg-app` / `bg-app` | `#F0EEE3` | `#0F130E` | Application background |
| `--surface-card` / `bg-surface` | `#FFFFFF` | `#182017` | Cards, sidebar, file lists, dialogs |
| `--primary` / `bg-primary` | `#606C38` | `#99AB5E` | Primary actions, active, success |
| `--primary-accent` / `text-primary-accent` | `#283618` | `#ADCB8A` | Emphasis, headings, default folder icon |
| `--secondary` / `bg-secondary` | `#DDA15E` | `#E1AB70` | Local-storage badge, in-progress |
| `--secondary-accent` / `bg-secondary-accent` | `#BC6C25` | `#D78032` | Cloud-storage badge, important emphasis |
| `--danger` / `bg-danger` | `#EC003F` | `#FF3366` | Destructive actions only |
| `--text-main` / `text-main` | `#283618` | `#F0EEE3` | Primary text |
| `--text-muted` / `text-muted` | `#5A6B46` | `#9CA88F` | Metadata and descriptions |
| `--border-subtle` / `border-subtle` | `rgba(0,0,0,0.06)` | `rgba(255,255,255,0.08)` | Thin borders/dividers |

Font mapping: `--font-sans: 'Montserrat', sans-serif`. Use `@import 'tailwindcss'` and the existing `@theme inline` token mapping, not inline per-component redefinitions.

## 3. Typography and layout

| Element | Standard utilities | Size/line-height | Weight |
| --- | --- | --- | --- |
| Page title | `text-2xl font-bold leading-tight` | 24/28px | 700 |
| Section heading | `text-lg font-semibold leading-snug` | 18/24px | 600 |
| File/item title | `text-sm font-medium leading-normal` | 14/20px | 500 |
| Body | `text-sm font-normal leading-normal` | 14/20px | 400 |
| Metadata/badge | `text-xs font-medium leading-none` | 12/16px | 500 |

- Use `bg-surface` foreground panels on `bg-app`; separate rows/panels with subtle borders, never shadows.
- **Radius:** `rounded-full` for input, buttons, filter chips, sidebar pills, switch tracks, avatars and badge pills; `rounded-xl` for dialogs, cards and concise panels.
- **Card spacing:** symmetric `--card-spacing` (default `--spacing(4)` = 16px). `Card.Root`: `py-(--card-spacing)` and `gap-(--card-spacing)` between Header/Content/Footer. Sections use `px-(--card-spacing)`; do not mix one-off `p-3`/`p-4` overrides with a different spacing token.
- **Edge-to-edge table/list exception:** root `py-0` and content `p-0` when the surface intentionally occupies the full card width (e.g., largest files, quota table).
- Use responsive Tailwind utilities and existing layouts; preserve usable touch targets and keyboard focus.

## 4. Explorer list, grid, and badges

- **List:** default. File row uses `flex`, `items-center`, `justify-between`, `border-b border-subtle px-4 py-3`, with `transition-colors` and neutral `hover:bg-black/5 dark:hover:bg-white/5`. Name `text-sm font-medium text-main`; metadata `text-xs text-muted`; icon container `rounded-lg bg-primary/10 p-2 text-primary`.
- **Grid:** existing `$lib/drive-view.svelte.ts` (`localStorage`: `cubycloud-drive-view`) and `DriveViewToggle` with `FileGridCard`. Icon/media area `aspect-video`, default `bg-surface`, hover/selected `bg-surface/80`, not `bg-primary/5`.
- List/Grid row actions: match **DropdownMenu** (⋮) and right-click **ContextMenu** items; do not introduce inconsistent action sets.
- **Storage node badges:** render physical `[R2]`, `[S3]`, `[SSD-n]`, `[HDD-n]` **only for Super Admin**. Cloud badge `bg-secondary-accent text-white dark:text-app`; local badge `bg-secondary text-main dark:text-app`. Physical node identifiers must not be disclosed to regular users.
- **Organization badge:** visible to all on Organization; `bg-primary/15 text-primary font-medium`.
- **Shared permission badges:** Viewer `bg-secondary text-main`; Editor `bg-primary/15 text-primary`.

## 5. Buttons and controls

Use the existing shadcn-svelte primitives, particularly `$lib/components/ui/button/button.svelte` (`rounded-full shadow-none`).

| Variant | Appearance / use |
| --- | --- |
| `default` | `bg-primary text-white hover:bg-primary/80` — upload, save, primary CTA |
| `secondary` | `bg-surface` with `border-subtle`, hover `bg-app` — back, rescan, backup |
| `outline` | `bg-app` with `border-subtle`, hover `bg-surface` |
| `destructive` | `bg-danger text-white hover:bg-danger/90` — permanent delete, purge |
| `ghost` | Transparent; contextual icons/menu |
| `link` | Text link with underline |

- Standard button `default`: `h-9` / `px-3.5`; `icon`: `size-9`. `xs/sm/lg`: `h-6/h-8/h-10`; icon `xs/sm/lg`: `size-6/size-8/size-10` when justified.
- Standard ⋮ action: `variant="ghost" size="icon"` and `bg-surface text-muted hover:bg-primary hover:text-white aria-expanded:bg-primary aria-expanded:text-white`.
- **Switch:** inactive track `bg-app`, checked `data-[state=checked]:bg-surface`, border `border-primary`, thumb `bg-primary`; bits-ui state selectors, no shadows.
- Maintain built-in semantic labels, focus rings, disabled styles, and keyboard interactions.

## 6. Quota and transfer feedback

- Quota Widget appears in sidebar for all logged-in users, with detailed breakdown on `/drive/storage-stats`; display in GB.
- Limited: `My Storage`, `8.2 / 15 GB`, flat `h-1.5 rounded-full` bar on neutral track. Fill `bg-primary`; near-limit (≥80%) warn using `bg-secondary-accent` and corresponding toast; full quota blocks writes and shows localized `USER_QUOTA_EXCEEDED` feedback.
- Unlimited/Super Admin: `Used: 8.2 GB` only; **no** limit number or capped progress bar.
- Transfer progress: flat `h-1.5 rounded-full bg-black/10 dark:bg-white/10` with `bg-primary` fill; when paused due to network use `bg-secondary`. Progress reaches 100% only on confirmed server success. Queue layout/behavior follows `SITEMAP.md`.

## 7. Interactions and overlays

- **Hover:** change background color only (`transition-colors`); no positional translations or shadow jumps. Sidebar inactive `hover:bg-primary/10`; active solid `bg-primary` without extra hover. Grid hover/selected `bg-surface/80`; file list/activity rows use neutral hover, not grid selection tint.
- **Dialogs/lightboxes:** overlay `bg-black/60 backdrop-blur-none`; container `rounded-xl bg-surface border border-subtle shadow-none`.
- **Error pages:** `$lib/components/error/error-page.svelte`, `$lib/error-pages.ts`, and illustrations in `$lib/assets/illustration/`; support 401/403/404/500/503, layout `bg-app`, approx. `md:w-1/2` illustration, primary return-to-home CTA `/`.
- **Offline-backed file rows:** keep file metadata visible; disable file actions requiring the offline node, but keep folder navigation. Show shadcn-svelte Popover on hover or click with localized notice; never disclose physical node name or ID. See `SITEMAP.md` for backend fields and permissions.
- **Localization:** English and Indonesian for labels, placeholders, errors, notifications, statuses, and accessible names; initial language from browser (`id` prefix → Indonesian, otherwise English); explicit selection from account-scoped browser `localStorage`, never backend. Global language switch uses Globe + shadcn Select (`id` / `en`).

## 8. Tailwind and review gates

- Prefer standard Tailwind v4 utilities such as `gap-2 rounded-lg bg-primary p-3.5 text-xs`.
- **Reject one-off arbitrary classes** such as `gap-[7px]`, `rounded-[10px]`, `bg-[#606C38]`, `p-[14px]`, or `text-[13px]`. If required, register the token in `src/routes/layout.css` using the project's `@theme inline` pattern, then document it here.
- Review `.svelte`/`.html` files for arbitrary-value styling and inconsistent visual tokens; do not rely on an overly narrow regex as the only validation.
- Follow `SITEMAP.md` for behavior and permissions; preserve user-visible accessibility and responsive behavior while editing styles.
