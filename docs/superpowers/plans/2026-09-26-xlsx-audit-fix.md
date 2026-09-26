# xlsx 审计例外修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use inline execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the vulnerable `xlsx@0.18.5` dependency with a pinned patched SheetJS 0.20.3 tarball, remove the two temporary audit exceptions, verify the admin export path, and publish the result through CI and VPS deployment.

**Architecture:** Keep the existing dynamic import and admin-only export flow unchanged. Change only the frontend dependency source/lockfile and the audit exception registry, then use the existing CI/security/Docker pipeline and image-based VPS deployment.

**Tech Stack:** pnpm 12, Vue 3, TypeScript, Vitest, Vite, GitHub Actions, GHCR, Docker Compose, SSH.

---

### Task 1: Update the dependency and audit policy

**Files:**
- Modify: `H:/sub2api/frontend/package.json`
- Modify: `H:/sub2api/frontend/pnpm-lock.yaml`
- Modify: `H:/sub2api/.github/audit-exceptions.yml`

- [ ] **Step 1: Change the dependency source**

Set the `xlsx` dependency in `frontend/package.json` to the exact official tarball URL:

```json
"xlsx": "https://cdn.sheetjs.com/xlsx-0.20.3/xlsx-0.20.3.tgz"
```

Keep `frontend/src/views/admin/UsageView.vue` unchanged.

- [ ] **Step 2: Regenerate the lockfile**

Run from `H:/sub2api/frontend`:

```powershell
pnpm install --lockfile-only
```

Expected: the importer specifier and package snapshot resolve to the 0.20.3 tarball, with no unrelated dependency changes.

- [ ] **Step 3: Remove only the two xlsx exceptions**

Delete the entries for:

```text
GHSA-4r6h-8v6p-xvw6
GHSA-5pgg-2g8v-p4x9
```

Keep all unrelated audit exceptions unchanged.

- [ ] **Step 4: Check the diff**

Run from `H:/sub2api`:

```powershell
git diff --check
git diff -- frontend/package.json frontend/pnpm-lock.yaml .github/audit-exceptions.yml
```

Expected: only the xlsx version/source, its lockfile records, and the two xlsx exception blocks change.

### Task 2: Run security and compatibility checks

**Files:**
- Test: `H:/sub2api/frontend/src/views/admin/__tests__/UsageView.spec.ts`
- Test: generated `H:/sub2api/frontend/audit.json`

- [ ] **Step 1: Install from the frozen lockfile**

Run from `H:/sub2api/frontend`:

```powershell
pnpm install --frozen-lockfile
```

Expected: install succeeds without lockfile changes.

- [ ] **Step 2: Run production dependency audit**

Run from `H:/sub2api/frontend`:

```powershell
pnpm audit --prod --audit-level=high --json > audit.json
```

Expected: the audit has no `xlsx` findings and exits successfully.

- [ ] **Step 3: Validate the repository exception checker**

Run from `H:/sub2api`:

```powershell
python tools/check_pnpm_audit_exceptions.py --audit frontend/audit.json --exceptions .github/audit-exceptions.yml
```

Expected: success with no missing or expired xlsx exception.

- [ ] **Step 4: Run the admin export test**

Run from `H:/sub2api/frontend`:

```powershell
pnpm vitest run src/views/admin/__tests__/UsageView.spec.ts
```

Expected: the admin export test passes and still produces the requested/sent/response/mismatch columns.

### Task 3: Run the frontend release gates

- [ ] **Step 1: Run all frontend tests**

Run from `H:/sub2api/frontend`:

```powershell
pnpm test:run
```

Expected: all frontend tests pass.

- [ ] **Step 2: Run typecheck and production build**

Run from `H:/sub2api/frontend`:

```powershell
pnpm run typecheck
pnpm run build
```

Expected: typecheck and Vite production build pass.

- [ ] **Step 3: Confirm the final worktree scope**

Run from `H:/sub2api`:

```powershell
git diff --check
git status --short --branch
```

Expected: only the intended dependency, lockfile, and audit policy files are modified, plus any generated audit output that is ignored or intentionally excluded.

### Task 4: Commit, push, monitor, and deploy

- [ ] **Step 1: Commit the dependency fix**

```powershell
git add frontend/package.json frontend/pnpm-lock.yaml .github/audit-exceptions.yml
git commit -m "fix: remove xlsx audit exceptions"
```

- [ ] **Step 2: Push and monitor the release gates**

```powershell
git push origin main
gh run list --commit HEAD --limit 20 --json databaseId,headSha,status,conclusion,name,workflowName,url,createdAt,updatedAt
```

Wait until CI, Security Scan, and Build Docker Image for the pushed commit are all successful.

- [ ] **Step 3: Verify the GHCR image**

Confirm `ghcr.io/balabalabling/sub2api:latest` and the commit tag point to the same digest for the pushed commit.

- [ ] **Step 4: Record the VPS pre-release state**

```powershell
ssh my-vps "cd /opt/sub2api && docker compose ps && docker inspect sub2api --format '{{.Image}} {{.State.Status}} {{.State.Health.Status}}'"
```

- [ ] **Step 5: Deploy only the application container**

```powershell
ssh my-vps "cd /opt/sub2api && docker compose pull sub2api && docker compose up -d sub2api"
```

- [ ] **Step 6: Verify production health**

Check `docker compose ps`, the application image digest, `http://127.0.0.1:8080/health`, and recent application logs. If health fails, use the recorded pre-release digest for rollback.

- [ ] **Step 7: Append the release record**

Add the final commit, workflow URLs, image digest, VPS before/after state, health result, and any new issue to `H:/sub2api/docs/RELEASE_CHECKLIST.md`, then commit and push the documentation-only record with `[skip ci]` so it does not replace the deployed application image.
