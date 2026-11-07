# Post-Publication Website Updates for CubyCrate (External Action Items)

**Context:** The marketing website and Starlight documentation repository (`cubycrate.com`, managed independently via `septian-padli/cubycloud-page` / Cloudflare Worker) must be updated **only after** the `v0.0.1-alpha.2` package is officially published to GHCR.

> **Rule:** Do NOT modify, commit, or deploy the website repository within this task. These action items are documented here for the website maintainer / owner.

---

## Required Website / Starlight Changes (Post-T8)

1. **Quickstart & Install Code Blocks:**
   - Update quickstart Compose instructions from local build fallback or historical alpha to official published image:
     ```yaml
     image: ghcr.io/septian-padli/cubycrate:v0.0.1-alpha.2
     ```
   - Verify `curl` download commands in docs point to `https://raw.githubusercontent.com/septian-padli/cubycrate/main/compose.yaml`.

2. **Repository & Issue Links:**
   - Verify all "View on GitHub", "Star on GitHub", and issue report links point to `https://github.com/septian-padli/cubycrate`.

3. **Docker & Registry Links:**
   - Update registry badge and documentation links to `https://github.com/septian-padli/cubycrate/pkgs/container/cubycrate`.

4. **Starlight Documentation Guides:**
   - Update "Getting Started" guide to reflect `v0.0.1-alpha.2` as the active alpha release.
   - Archive or clearly distinguish any historical `v0.0.1-alpha.1` guides with Clean Break warning notes.
   - Update backup/restore documentation to reflect volume `cubycrate_data` and database `db/cubycrate.db`.

5. **Cloudflare Worker & Domain Verification:**
   - Verify `https://cubycrate.com` routes correctly and SSL certificates are healthy.
