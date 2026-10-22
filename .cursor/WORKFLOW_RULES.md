# CubyCloud — Development Workflow Rules

> Scope: Git authorization, implementation handoff, and documentation placement. This is a **reference document**, not a mandatory full read for every task. Root `AGENTS.md` carries the universal restrictions.

## 1. Git operations: explicit permission required

**Never perform the following without an explicit user request for that action:**

- `git commit`, `git push`, `git push --force`, creating a PR/release, or publishing artifacts.
- Destructive `git reset`, cleaning/removing untracked changes, dropping a stash, or deleting branches, files, or worker worktrees that might contain user work.
- Treat “continue”, “fix it”, “finish”, “run tests”, and “review” as **not authorizing commit or push**.

A request to **commit** authorizes the specified commit only; it does not imply permission to **push**. Inspect current diff/status before any authorized Git mutation, keep unrelated changes intact, and report what was changed.

## 2. Default delivery loop

1. Understand task and ownership; inspect relevant code and contracts using progressive context loading.
2. Implement small, reviewable changes and add focused tests.
3. Run relevant checks; fix and rerun failures when possible.
4. Integrate native OMP subagent work safely, if used; resolve shared-file conflicts under one owner.
5. Report changed files, actual test results, incomplete cases, and risks. **Stop before commit/push unless explicitly instructed.**

Never label implementation “verified” based only on a subagent claim, a successful build without tests, or a test that was not run. Use PASS / FAIL / SKIPPED / NOT RUN honestly.

## 3. Documentation placement

| Scope | Preferred location |
| --- | --- |
| Product requirements, cross-module rules | `docs/prd.md` or another relevant file under `docs/` |
| System architecture, deployment/runtime design | `docs/architecture.md` or `docs/` |
| Frontend implementation guides | `apps/frontend/docs/` |
| Frontend route/page contracts | `apps/frontend/SITEMAP.md` |
| Frontend visual specification | `apps/frontend/DESIGN.md` |
| Backend implementation and API details | `apps/backend/docs/` |
| Repository contribution and project entrypoints | Root `README.md`, `CONTRIBUTING.md`, `SECURITY.md`, license/changelog |
| Agent-wide instructions | Root `AGENTS.md`; per-area implementation rules in scoped `AGENTS.md` |

Place new documentation in its owning area and update affected references. **Do not relocate existing documents solely for tidiness** during unrelated tasks. Do not duplicate product/design contracts inside agent instruction files.

## 4. Reporting checklist

- Implementation and affected contracts summarized.
- Changed files and relevant source references provided.
- Tests identified with their **actual** results; blockers/limitations stated.
- Existing user and parallel-worker changes preserved.
- Commit/push/publish status explicitly reported when relevant.

**Permanent rule:** User approval is required for commits and pushes. No previous violation history is needed in agent context to enforce it.
