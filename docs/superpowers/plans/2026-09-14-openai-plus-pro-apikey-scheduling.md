# OpenAI PLUS、PRO 与 API Key 分层调度 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将当前定制分支升级到 Sub2API `v0.2.4`，并实现新会话严格按 PLUS、PRO、API Key、兼容账号池逐级调度，其中 PLUS 按 5 小时剩余额度优先，PRO 完全忽略 5 小时限制。

**Architecture:** 先把上游稳定版合入独立升级分支并完成现有定制回归，再在 `v0.2.4` 高级 OpenAI 调度器的通用资格过滤之后增加四级候选池。会话粘性仍先于分池执行；分池内部复用已有并发获取、等待计划、冷却、模型能力和失败排除机制。

**Tech Stack:** Go 1.27、Ent、Google Wire、PostgreSQL、Redis、Vue 3、TypeScript、Vitest、pnpm、GitHub Actions、Docker Compose

---

## File map

### Upgrade and conflict resolution

- `backend/cmd/server/wire_gen.go`: preserve generated dependency injection after merging upstream and local storefront services.
- `backend/go.mod`, `backend/go.sum`: take the v0.2.4 dependency set plus the local dependencies still imported by retained features.
- `backend/internal/handler/admin/group_handler.go`: preserve upstream group behavior and local image/model routing behavior.
- `backend/internal/pkg/ctxkey/ctxkey.go`: retain local request context keys on top of the upstream key set.
- `backend/internal/server/router.go`: preserve upstream routes and local storefront routes.
- `backend/internal/service/gateway_scheduling.go`: keep upstream scheduling architecture and reapply local image routing semantics.
- `backend/internal/service/model_rate_limit.go`: keep upstream model cooldown behavior and local model routing additions.
- `backend/internal/service/openai_account_scheduler.go`: take the v0.2.4 scheduler as the structural base, then reapply required local behavior.
- `backend/internal/service/openai_ws_forwarder.go`: preserve upstream WS fixes and local Responses namespace handling hooks.
- `frontend/src/api/admin/index.ts`: retain both upstream admin API modules and local storefront admin API export.
- `frontend/src/components/keys/UseKeyModal.vue`: retain upstream UI changes and local Codex configuration download.
- `frontend/src/i18n/locales/en/admin/index.ts`, `frontend/src/i18n/locales/zh/admin/index.ts`: retain upstream locale modules and local store locale module.

### Tiered scheduling feature

- Create `backend/internal/service/openai_account_tier.go`: account pool classification, PLUS quota snapshot rank, pool partitioning, deterministic pool-specific ordering.
- Create `backend/internal/service/openai_account_tier_test.go`: pure unit tests for plan classification and PLUS/PRO ordering.
- Modify `backend/internal/service/openai_account_scheduler.go`: four-pool acquisition and decision observability.
- Modify `backend/internal/service/openai_account_scheduler_test.go`: scheduler-level fallback, sticky-session, concurrency and exclusion tests.
- Modify `backend/internal/service/gateway_service.go`: carry the selected internal scheduling pool on `AccountSelectionResult` without exposing it through an API DTO.

### Existing custom regression targets

- `backend/internal/service/gateway_multiplatform_test.go`
- `backend/internal/service/image_generation_intent_test.go`
- `backend/internal/service/openai_gateway_service_tool_correction_test.go`
- `backend/internal/service/openai_ws_forwarder_hotpath_optimization_test.go`
- `backend/internal/service/payment_config_plans_validation_test.go`
- `frontend/src/components/keys/__tests__/UseKeyModal.spec.ts`
- `frontend/src/composables/__tests__/useDocumentTitle.spec.ts`
- `frontend/src/utils/__tests__/configScriptDownload.spec.ts`
- `frontend/src/views/user/__tests__/PaymentView.spec.ts`

---

### Task 1: Create the upgrade branch and record the baseline

**Files:**
- Verify: `docs/superpowers/specs/2026-09-14-openai-plus-pro-apikey-scheduling-design.md`
- Verify: repository state and refs

- [ ] **Step 1: Confirm the approved design commit and a clean tree**

Run from `H:\sub2api`:

```powershell
git status --short --branch
git log -2 --oneline
git rev-parse 'refs/tags/v0.2.4^{}'
```

Expected: the tree is clean, commit `4e1c32abd` is present, and the tag resolves to commit `5de5e2bed035d43591a2e10e51f420ef6a84eb98`.

- [ ] **Step 2: Create the isolated upgrade branch**

```powershell
git switch -c codex/upgrade-v0.2.4-tiered-openai-scheduling
```

Expected: Git reports the new branch and `git branch --show-current` prints `codex/upgrade-v0.2.4-tiered-openai-scheduling`.

- [ ] **Step 3: Save machine-readable baseline evidence**

```powershell
git status --porcelain=v1
git rev-list --left-right --count main...'refs/tags/v0.2.4^{}'
git diff --name-only 2730c1c43b29be003925b033f3f9e645e726bb8c..main
```

Expected: no uncommitted files; divergence reports 43 local commits and 1473 v0.2.4-side commits before this plan commit is counted.

---

### Task 2: Merge Sub2API v0.2.4 while retaining local features

**Files:**
- Modify the 14 conflict files listed in the file map.
- Verify all automatically merged custom files listed by `git diff --name-only --diff-filter=U`.

- [ ] **Step 1: Start the stable-version merge**

```powershell
git merge --no-ff 'refs/tags/v0.2.4^{}' -m "merge: upgrade Sub2API to v0.2.4"
```

Expected: merge pauses with the known conflict set. Capture it with:

```powershell
git diff --name-only --diff-filter=U
```

- [ ] **Step 2: Resolve generated and dependency files from their source of truth**

For `backend/go.mod` and `backend/go.sum`, start from v0.2.4, then let Go restore dependencies imported by retained local code:

```powershell
git checkout --theirs -- backend/go.mod backend/go.sum
Push-Location backend
go mod tidy
Pop-Location
```

For `backend/cmd/server/wire_gen.go`, resolve `backend/cmd/server/wire.go` first so it contains both upstream providers and the retained storefront/payment providers. Then regenerate:

```powershell
Push-Location backend
go generate ./cmd/server
Pop-Location
```

Expected: `wire_gen.go` contains no conflict markers and includes constructors required by `storefront`, payment fulfillment and all v0.2.4 services.

- [ ] **Step 3: Resolve backend semantic conflicts using upstream structure plus local behavior**

For each file below, begin from the v0.2.4 side and reapply only the named local behavior:

```powershell
git checkout --theirs -- backend/internal/handler/admin/group_handler.go
git checkout --theirs -- backend/internal/pkg/ctxkey/ctxkey.go
git checkout --theirs -- backend/internal/server/router.go
git checkout --theirs -- backend/internal/service/gateway_scheduling.go
git checkout --theirs -- backend/internal/service/model_rate_limit.go
git checkout --theirs -- backend/internal/service/openai_account_scheduler.go
git checkout --theirs -- backend/internal/service/openai_ws_forwarder.go
```

Reapply from the local parent with focused diffs:

```powershell
git diff 'refs/tags/v0.2.4^{}'...main -- backend/internal/handler/admin/group_handler.go
git diff 'refs/tags/v0.2.4^{}'...main -- backend/internal/pkg/ctxkey/ctxkey.go
git diff 'refs/tags/v0.2.4^{}'...main -- backend/internal/server/router.go
git diff 'refs/tags/v0.2.4^{}'...main -- backend/internal/service/gateway_scheduling.go
git diff 'refs/tags/v0.2.4^{}'...main -- backend/internal/service/model_rate_limit.go
git diff 'refs/tags/v0.2.4^{}'...main -- backend/internal/service/openai_account_scheduler.go
git diff 'refs/tags/v0.2.4^{}'...main -- backend/internal/service/openai_ws_forwarder.go
```

Required retained semantics:

```text
group_handler.go: expose model routing needed by OpenAI image groups
ctxkey.go: preserve local image-routing and fulfillment context keys
router.go: register storefront routes alongside v0.2.4 routes
gateway_scheduling.go: preserve native image routing and fulfillment recovery
model_rate_limit.go: preserve local image/model route exemptions
openai_account_scheduler.go: preserve image capability routing only; use v0.2.4 scheduler control flow
openai_ws_forwarder.go: preserve custom tool namespace sanitation integration
```

After editing, verify every retained local symbol still has references:

```powershell
rg -n "ImageGenerationIntent|Storefront|ToolNamespace|Fulfillment|configScript" backend frontend
```

- [ ] **Step 4: Resolve frontend conflicts using named exports**

Start from v0.2.4, then restore the local exports and UI actions:

```powershell
git checkout --theirs -- frontend/src/api/admin/index.ts
git checkout --theirs -- frontend/src/components/keys/UseKeyModal.vue
git checkout --theirs -- frontend/src/i18n/locales/en/admin/index.ts
git checkout --theirs -- frontend/src/i18n/locales/zh/admin/index.ts
```

Required retained code shapes:

```typescript
// frontend/src/api/admin/index.ts
export * from './store'
```

```typescript
// both admin locale index files
import store from './store'
// include `store` exactly once in the exported admin locale object
```

In `UseKeyModal.vue`, retain the call to the local configuration generator from `@/utils/configScriptDownload` and the download action, while preserving all new v0.2.4 props, validation and layout.

- [ ] **Step 5: Clear conflict markers and stage the merge**

```powershell
$unmerged = @(git diff --name-only --diff-filter=U)
if ($unmerged.Count -ne 0) { $unmerged; throw 'Unmerged files remain' }
rg -n '^(<<<<<<<|=======|>>>>>>>)' . -g '!frontend/node_modules' -g '!backend/bin'
git add --all
git diff --cached --check
```

Expected: the unmerged list is empty, the marker search returns no matches, and `git diff --cached --check` reports no whitespace errors.

- [ ] **Step 6: Run focused compile checks before completing the merge**

```powershell
Push-Location backend
go test ./internal/service ./internal/handler/... ./internal/server/...
Pop-Location
pnpm --dir frontend run typecheck
```

Expected: Go packages compile and tests pass; TypeScript typecheck passes.

- [ ] **Step 7: Complete the merge commit**

```powershell
git commit --no-edit
```

Expected: a merge commit named `merge: upgrade Sub2API to v0.2.4`.

---

### Task 3: Regenerate code and prove the upgraded baseline

**Files:**
- Regenerate: `backend/ent/**`
- Regenerate: `backend/cmd/server/wire_gen.go`
- Verify: all existing backend and frontend tests

- [ ] **Step 1: Regenerate Ent and Wire**

```powershell
Push-Location backend
go generate ./ent
go generate ./cmd/server
Pop-Location
```

Expected: generation completes successfully.

- [ ] **Step 2: Verify generated output is reproducible**

```powershell
git diff --check
git status --short
```

Expected: generated differences are limited to deterministic files whose source schema or provider graph changed during conflict resolution. Inspect each difference before staging.

- [ ] **Step 3: Run the retained custom backend regressions**

```powershell
Push-Location backend
go test ./internal/service -run 'Test.*(ImageGenerationIntent|ToolCorrection|Namespace|Payment|Storefront|Multiplatform)' -count=1
Pop-Location
```

Expected: all matching tests pass. If a retained custom test was renamed by v0.2.4, run its owning package without `-run` and record the replacement test name in the commit body.

- [ ] **Step 4: Run the retained custom frontend regressions**

```powershell
pnpm --dir frontend exec vitest run `
  src/components/keys/__tests__/UseKeyModal.spec.ts `
  src/composables/__tests__/useDocumentTitle.spec.ts `
  src/utils/__tests__/configScriptDownload.spec.ts `
  src/views/user/__tests__/PaymentView.spec.ts
```

Expected: all listed Vitest files pass.

- [ ] **Step 5: Build the upgraded baseline**

```powershell
make build-backend
make build-frontend
```

Expected: both builds finish successfully and `backend/bin/server` is produced locally.

- [ ] **Step 6: Commit deterministic baseline fixes, if generation or regression repair changed files**

```powershell
git add backend frontend
git diff --cached --check
git commit -m "fix: preserve local features on v0.2.4"
```

Expected: commit created only when the index contains reviewed baseline fixes. When no files changed, skip the commit and proceed.

---

### Task 4: Add pure account-pool classification

**Files:**
- Create: `backend/internal/service/openai_account_tier.go`
- Create: `backend/internal/service/openai_account_tier_test.go`

- [ ] **Step 1: Write failing classification tests**

Create `backend/internal/service/openai_account_tier_test.go` with:

```go
package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIAccountPoolFor(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
		want    openAIAccountPool
	}{
		{name: "plus", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": " PLUS "}}, want: openAIAccountPoolPlus},
		{name: "pro", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "Pro"}}, want: openAIAccountPoolPro},
		{name: "api key", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, want: openAIAccountPoolAPIKey},
		{name: "business", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "business"}}, want: openAIAccountPoolCompat},
		{name: "free", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "free"}}, want: openAIAccountPoolCompat},
		{name: "unknown", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}, want: openAIAccountPoolCompat},
		{name: "nil", account: nil, want: openAIAccountPoolCompat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, openAIAccountPoolFor(tt.account))
		})
	}
}

func TestPartitionOpenAIAccountsByPoolPreservesInputOrder(t *testing.T) {
	accounts := []*Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "pro"}},
		{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "plus"}},
		{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "team"}},
	}

	pools := partitionOpenAIAccountsByPool(accounts)
	require.Equal(t, []int64{3}, accountIDs(pools[openAIAccountPoolPlus]))
	require.Equal(t, []int64{2}, accountIDs(pools[openAIAccountPoolPro]))
	require.Equal(t, []int64{1}, accountIDs(pools[openAIAccountPoolAPIKey]))
	require.Equal(t, []int64{4}, accountIDs(pools[openAIAccountPoolCompat]))
}

func accountIDs(accounts []*Account) []int64 {
	ids := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.ID)
	}
	return ids
}
```

- [ ] **Step 2: Run the tests and verify the new symbols are missing**

```powershell
Push-Location backend
go test ./internal/service -run 'Test(OpenAIAccountPoolFor|PartitionOpenAIAccountsByPool)' -count=1
Pop-Location
```

Expected: compile failure mentioning `openAIAccountPool` or `openAIAccountPoolFor`.

- [ ] **Step 3: Implement pool classification**

Create `backend/internal/service/openai_account_tier.go` with:

```go
package service

import "strings"

type openAIAccountPool string

const (
	openAIAccountPoolPlus   openAIAccountPool = "plus"
	openAIAccountPoolPro    openAIAccountPool = "pro"
	openAIAccountPoolAPIKey openAIAccountPool = "apikey"
	openAIAccountPoolCompat openAIAccountPool = "compat"
)

var openAIAccountPoolOrder = []openAIAccountPool{
	openAIAccountPoolPlus,
	openAIAccountPoolPro,
	openAIAccountPoolAPIKey,
	openAIAccountPoolCompat,
}

func openAIAccountPoolFor(account *Account) openAIAccountPool {
	if account == nil || !account.IsOpenAI() {
		return openAIAccountPoolCompat
	}
	if account.IsOpenAIApiKey() {
		return openAIAccountPoolAPIKey
	}
	if !account.IsOpenAIOAuth() {
		return openAIAccountPoolCompat
	}
	switch strings.ToLower(strings.TrimSpace(account.GetCredential("plan_type"))) {
	case "plus":
		return openAIAccountPoolPlus
	case "pro":
		return openAIAccountPoolPro
	default:
		return openAIAccountPoolCompat
	}
}

func partitionOpenAIAccountsByPool(accounts []*Account) map[openAIAccountPool][]*Account {
	pools := make(map[openAIAccountPool][]*Account, len(openAIAccountPoolOrder))
	for _, account := range accounts {
		pool := openAIAccountPoolFor(account)
		pools[pool] = append(pools[pool], account)
	}
	return pools
}
```

- [ ] **Step 4: Run the classification tests**

```powershell
Push-Location backend
go test ./internal/service -run 'Test(OpenAIAccountPoolFor|PartitionOpenAIAccountsByPool)' -count=1
Pop-Location
```

Expected: PASS.

- [ ] **Step 5: Commit classification**

```powershell
git add backend/internal/service/openai_account_tier.go backend/internal/service/openai_account_tier_test.go
git commit -m "feat: classify OpenAI accounts into scheduling pools"
```

---

### Task 5: Add deterministic PLUS and PRO ordering

**Files:**
- Modify: `backend/internal/service/openai_account_tier.go`
- Modify: `backend/internal/service/openai_account_tier_test.go`

- [ ] **Step 1: Write failing PLUS quota-rank tests**

Append tests covering fresh, stale and missing states:

```go
func TestOpenAIPlusQuotaRank(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	fresh := &Account{Extra: map[string]any{
		"codex_5h_used_percent": 25.0,
		"codex_usage_updated_at": now.Add(-time.Minute).Format(time.RFC3339),
	}}
	stale := &Account{Extra: map[string]any{
		"codex_5h_used_percent": 10.0,
		"codex_usage_updated_at": now.Add(-9 * time.Hour).Format(time.RFC3339),
	}}

	require.Equal(t, openAIPlusQuotaFresh, openAIPlusQuotaRankFor(fresh, now).State)
	require.InDelta(t, 75.0, openAIPlusQuotaRankFor(fresh, now).RemainingPercent, 0.001)
	require.Equal(t, openAIPlusQuotaStale, openAIPlusQuotaRankFor(stale, now).State)
	require.Equal(t, openAIPlusQuotaMissing, openAIPlusQuotaRankFor(&Account{}, now).State)
}

func TestSortOpenAIAccountPoolPlusPrefersFreshThenRemaining(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	accounts := []*Account{
		{ID: 1, Priority: 0, Extra: map[string]any{"codex_5h_used_percent": 5.0, "codex_usage_updated_at": now.Add(-9 * time.Hour).Format(time.RFC3339)}},
		{ID: 2, Priority: 5, Extra: map[string]any{"codex_5h_used_percent": 60.0, "codex_usage_updated_at": now.Add(-time.Minute).Format(time.RFC3339)}},
		{ID: 3, Priority: 9, Extra: map[string]any{"codex_5h_used_percent": 20.0, "codex_usage_updated_at": now.Add(-time.Minute).Format(time.RFC3339)}},
	}
	loads := map[int64]*AccountLoadInfo{
		1: {AccountID: 1, LoadRate: 0},
		2: {AccountID: 2, LoadRate: 0},
		3: {AccountID: 3, LoadRate: 99},
	}

	sortOpenAIAccountsForPool(accounts, openAIAccountPoolPlus, loads, now)
	require.Equal(t, []int64{3, 2, 1}, accountIDs(accounts))
}
```

Add `time` to the test imports.

- [ ] **Step 2: Write the failing PRO test proving 5-hour data is ignored**

```go
func TestSortOpenAIAccountPoolProIgnoresFiveHourQuota(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	accounts := []*Account{
		{ID: 1, Priority: 5, Extra: map[string]any{"codex_5h_used_percent": 0.0}},
		{ID: 2, Priority: 0, Extra: map[string]any{"codex_5h_used_percent": 100.0}},
	}
	loads := map[int64]*AccountLoadInfo{
		1: {AccountID: 1, LoadRate: 0},
		2: {AccountID: 2, LoadRate: 90},
	}

	sortOpenAIAccountsForPool(accounts, openAIAccountPoolPro, loads, now)
	require.Equal(t, []int64{2, 1}, accountIDs(accounts))
}
```

- [ ] **Step 3: Run the new tests and verify helper symbols are missing**

```powershell
Push-Location backend
go test ./internal/service -run 'Test(OpenAIPlusQuotaRank|SortOpenAIAccountPool)' -count=1
Pop-Location
```

Expected: compile failure for `openAIPlusQuotaRankFor` or `sortOpenAIAccountsForPool`.

- [ ] **Step 4: Implement quota rank and deterministic sorting**

Extend `openai_account_tier.go`:

```go
import (
	"sort"
	"time"
)

type openAIPlusQuotaState int

const (
	openAIPlusQuotaFresh openAIPlusQuotaState = iota
	openAIPlusQuotaStale
	openAIPlusQuotaMissing
)

type openAIPlusQuotaRank struct {
	State            openAIPlusQuotaState
	RemainingPercent float64
}

func openAIPlusQuotaRankFor(account *Account, now time.Time) openAIPlusQuotaRank {
	if account == nil || len(account.Extra) == 0 {
		return openAIPlusQuotaRank{State: openAIPlusQuotaMissing}
	}
	used, ok := resolveAccountExtraNumber(account.Extra, "codex_5h_used_percent", "codex_secondary_used_percent")
	if !ok {
		return openAIPlusQuotaRank{State: openAIPlusQuotaMissing}
	}
	if openAIQuotaWindowResetAny(account.Extra, now, "secondary", "5h") || openAIQuotaHeadroomSnapshotStale(account.Extra, now) {
		return openAIPlusQuotaRank{State: openAIPlusQuotaStale}
	}
	return openAIPlusQuotaRank{
		State:            openAIPlusQuotaFresh,
		RemainingPercent: 100 * (1 - clamp01(used/100)),
	}
}

func sortOpenAIAccountsForPool(accounts []*Account, pool openAIAccountPool, loads map[int64]*AccountLoadInfo, now time.Time) {
	loadFor := func(account *Account) *AccountLoadInfo {
		if account != nil {
			if load := loads[account.ID]; load != nil {
				return load
			}
		}
		return &AccountLoadInfo{}
	}
	sort.SliceStable(accounts, func(i, j int) bool {
		left, right := accounts[i], accounts[j]
		if pool == openAIAccountPoolPlus {
			leftQuota := openAIPlusQuotaRankFor(left, now)
			rightQuota := openAIPlusQuotaRankFor(right, now)
			if leftQuota.State != rightQuota.State {
				return leftQuota.State < rightQuota.State
			}
			if leftQuota.State == openAIPlusQuotaFresh && leftQuota.RemainingPercent != rightQuota.RemainingPercent {
				return leftQuota.RemainingPercent > rightQuota.RemainingPercent
			}
		}
		if left.Priority != right.Priority {
			return left.Priority < right.Priority
		}
		leftLoad, rightLoad := loadFor(left), loadFor(right)
		if leftLoad.LoadRate != rightLoad.LoadRate {
			return leftLoad.LoadRate < rightLoad.LoadRate
		}
		if leftLoad.WaitingCount != rightLoad.WaitingCount {
			return leftLoad.WaitingCount < rightLoad.WaitingCount
		}
		return left.ID < right.ID
	})
}
```

Keep the existing `strings` import in the same grouped import block. Runtime EWMA error rate and TTFT remain scheduler score tie-breakers after this preordering; do not copy scheduler-private stats into this helper.

- [ ] **Step 5: Run unit tests**

```powershell
Push-Location backend
go test ./internal/service -run 'Test(OpenAIPlusQuotaRank|SortOpenAIAccountPool|OpenAIAccountPoolFor|PartitionOpenAIAccountsByPool)' -count=1
Pop-Location
```

Expected: PASS.

- [ ] **Step 6: Commit ordering helpers**

```powershell
git add backend/internal/service/openai_account_tier.go backend/internal/service/openai_account_tier_test.go
git commit -m "feat: rank PLUS accounts by five-hour headroom"
```

---

### Task 6: Replace the two-pool scheduler path with strict four-pool acquisition

**Files:**
- Modify: `backend/internal/service/openai_account_scheduler.go:1400-1565` after the v0.2.4 merge
- Modify: `backend/internal/service/gateway_service.go:578-584` after the v0.2.4 merge
- Modify: `backend/internal/service/openai_account_scheduler_test.go`

- [ ] **Step 1: Add failing end-to-end scheduler tests for PLUS, PRO and API Key**

Add table-driven tests next to the existing subscription-priority tests. Reuse `newSchedulerTestSubscriptionPriorityConfig`, the existing in-memory account repository and concurrency mock. The core assertions must be:

```go
func TestOpenAIGatewayService_SelectAccountWithScheduler_TieredPoolOrder(t *testing.T) {
	tests := []struct {
		name     string
		accounts []Account
		wantID   int64
		wantPool openAIAccountPool
	}{
		{
			name: "plus before pro and api key",
			accounts: []Account{
				newSchedulableOpenAITestAccount(1, AccountTypeAPIKey, ""),
				newSchedulableOpenAITestAccount(2, AccountTypeOAuth, "pro"),
				newSchedulableOpenAITestAccount(3, AccountTypeOAuth, "plus"),
			},
			wantID: 3, wantPool: openAIAccountPoolPlus,
		},
		{
			name: "pro before api key when plus absent",
			accounts: []Account{
				newSchedulableOpenAITestAccount(4, AccountTypeAPIKey, ""),
				newSchedulableOpenAITestAccount(5, AccountTypeOAuth, "pro"),
			},
			wantID: 5, wantPool: openAIAccountPoolPro,
		},
		{
			name: "api key before compatibility pool",
			accounts: []Account{
				newSchedulableOpenAITestAccount(6, AccountTypeOAuth, "business"),
				newSchedulableOpenAITestAccount(7, AccountTypeAPIKey, ""),
			},
			wantID: 7, wantPool: openAIAccountPoolAPIKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTieredSchedulerTestService(t, tt.accounts)
			selection, decision, err := svc.SelectAccountWithScheduler(context.Background(), nil, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
			require.NoError(t, err)
			require.Equal(t, tt.wantID, selection.Account.ID)
			require.Equal(t, string(tt.wantPool), decision.SelectedPool)
		})
	}
}
```

Implement `newSchedulableOpenAITestAccount` and `newTieredSchedulerTestService` using the same repository and concurrency constructors already used by `TestOpenAIGatewayService_SelectAccountWithScheduler_SubscriptionPriorityChoosesSubscriptionPoolFirst`; do not introduce a second mock framework.

- [ ] **Step 2: Add a failing test proving PRO ignores five-hour exhaustion**

```go
func TestOpenAIGatewayService_SelectAccountWithScheduler_ProIgnoresFiveHourSnapshot(t *testing.T) {
	pro := newSchedulableOpenAITestAccount(20, AccountTypeOAuth, "pro")
	pro.Extra = map[string]any{
		"codex_5h_used_percent": 100.0,
		"codex_usage_updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	apiKey := newSchedulableOpenAITestAccount(21, AccountTypeAPIKey, "")

	svc := newTieredSchedulerTestService(t, []Account{apiKey, pro})
	selection, decision, err := svc.SelectAccountWithScheduler(context.Background(), nil, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.Equal(t, int64(20), selection.Account.ID)
	require.Equal(t, string(openAIAccountPoolPro), decision.SelectedPool)
}
```

The test fixture must avoid setting PRO's generic `Schedulable` state to false through threshold evaluation; the assertion targets scheduler treatment of an already schedulable PRO candidate.

- [ ] **Step 3: Run focused scheduler tests and verify failure**

```powershell
Push-Location backend
go test ./internal/service -run 'TestOpenAIGatewayService_SelectAccountWithScheduler_(TieredPoolOrder|ProIgnoresFiveHourSnapshot)' -count=1
Pop-Location
```

Expected: compile failure for `SelectedPool` or behavioral failure selecting the old combined subscription pool.

- [ ] **Step 4: Add pool observability to the decision**

Extend `OpenAIAccountScheduleDecision`:

```go
type OpenAIAccountScheduleDecision struct {
	// existing fields remain unchanged
	SelectedPool     string
	FallbackFromPool string
	PlusQuotaState   string
}
```

Set `SelectedPool` only for the load-balance layer. Sticky decisions retain their current layer and derive `SelectedPool` from the selected account without forcing a reselection.

- [ ] **Step 5: Introduce a reusable single-pool attempt helper**

Add beside `trySelectByLoadBalancePool`:

```go
type openAIAccountPoolAttempt struct {
	pool    openAIAccountPool
	result  *AccountSelectionResult
	attempt openAIAccountLoadSelectionAttempt
}

func (s *defaultOpenAIAccountScheduler) tryOpenAIAccountPool(
	ctx context.Context,
	req OpenAIAccountScheduleRequest,
	pool openAIAccountPool,
	accounts []*Account,
	loadMap map[int64]*AccountLoadInfo,
	budget *openAISelectionProbeBudget,
) openAIAccountPoolAttempt {
	ordered := append([]*Account(nil), accounts...)
	sortOpenAIAccountsForPool(ordered, pool, loadMap, time.Now())
	attempt := s.trySelectByLoadBalancePool(ctx, req, ordered, loadMap, budget)
	return openAIAccountPoolAttempt{pool: pool, result: attempt.result, attempt: attempt}
}
```

- [ ] **Step 6: Replace subscription-vs-regular partitioning with ordered pools**

In `selectByLoadBalance`, replace the `partitionOpenAIChatGPTSubscriptionAccounts` block with:

```go
if req.SubscriptionPriority && NormalizeOpenAICompatiblePlatform(req.Platform) == PlatformOpenAI {
	pools := partitionOpenAIAccountsByPool(filtered)
	var firstAttempt *openAIAccountPoolAttempt
	for _, pool := range openAIAccountPoolOrder {
		accounts := pools[pool]
		if len(accounts) == 0 {
			continue
		}
		poolAttempt := s.tryOpenAIAccountPool(ctx, req, pool, accounts, loadMap, budget)
		if firstAttempt == nil {
			copyAttempt := poolAttempt
			firstAttempt = &copyAttempt
		}
		if poolAttempt.attempt.err != nil && !poolAttempt.attempt.noCompactCandidates {
			return nil, poolAttempt.attempt.candidateCount, poolAttempt.attempt.topK, poolAttempt.attempt.loadSkew, poolAttempt.attempt.err
		}
		if poolAttempt.result != nil {
			poolAttempt.result.SchedulingPool = string(pool)
			return poolAttempt.result, poolAttempt.attempt.candidateCount, poolAttempt.attempt.topK, poolAttempt.attempt.loadSkew, nil
		}
	}
	if firstAttempt != nil {
		return s.finishLoadBalanceSelectionFallback(ctx, req, firstAttempt.attempt, budget, filterStats)
	}
}
```

Add `SchedulingPool string` to the internal `AccountSelectionResult` in `backend/internal/service/gateway_service.go`:

```go
type AccountSelectionResult struct {
	Account        *Account
	Acquired       bool
	ReleaseFunc    func()
	WaitPlan       *AccountWaitPlan
	profitGate     *openAIProfitControlGate
	SchedulingPool string
}
```

This service type is not an API DTO. When `finishLoadBalanceSelectionFallback` returns a waiting plan, copy the originating pool into the result so the decision remains observable.

Delete `partitionOpenAIChatGPTSubscriptionAccounts` after its callers reach zero. Keep `Account.IsOpenAIChatGPTSubscription()` because other v0.2.4 code may still use it.

- [ ] **Step 7: Preserve same-pool waiting before cross-pool fallback**

Adjust the loop so a pool with a valid `WaitPlan` is finalized before entering the next pool when the existing scheduler considers it waitable. Encode this as a helper:

```go
func shouldWaitInPreferredOpenAIAccountPool(attempt openAIAccountLoadSelectionAttempt) bool {
	return attempt.result == nil && attempt.err == nil && len(attempt.selectionOrder) > 0
}
```

Use `finishLoadBalanceSelectionFallback` for that pool and return its waiting result when present. Enter the next pool only when the preferred pool has no immediate result and no valid waiting result.

- [ ] **Step 8: Run the tier-order tests**

```powershell
Push-Location backend
go test ./internal/service -run 'TestOpenAIGatewayService_SelectAccountWithScheduler_(TieredPoolOrder|ProIgnoresFiveHourSnapshot|SubscriptionPriority)' -count=1
Pop-Location
```

Expected: PASS, including the retained v0.2.4 subscription-priority regressions after their assertions are updated to the four-pool semantics.

- [ ] **Step 9: Commit four-pool scheduling**

```powershell
git add backend/internal/service/openai_account_scheduler.go backend/internal/service/openai_account_scheduler_test.go backend/internal/service/openai_account_tier.go
git commit -m "feat: schedule OpenAI accounts by subscription tier"
```

---

### Task 7: Prove PLUS headroom, stale snapshots, same-tier retry and sticky behavior

**Files:**
- Modify: `backend/internal/service/openai_account_scheduler_test.go`
- Modify: `backend/internal/service/openai_account_tier.go`
- Modify: `backend/internal/service/openai_account_scheduler.go`

- [ ] **Step 1: Add failing scheduler tests for PLUS remaining quota**

Create two schedulable PLUS fixtures with fresh snapshots and inverted priority/load values. Assert the larger remaining quota wins even when it has a worse manual priority and higher load:

```go
func TestOpenAIGatewayService_SelectAccountWithScheduler_PlusUsesFiveHourHeadroomAsHardOrder(t *testing.T) {
	now := time.Now().UTC()
	lowRemaining := newSchedulableOpenAITestAccount(30, AccountTypeOAuth, "plus")
	lowRemaining.Priority = 0
	lowRemaining.Extra = map[string]any{"codex_5h_used_percent": 80.0, "codex_usage_updated_at": now.Format(time.RFC3339)}
	highRemaining := newSchedulableOpenAITestAccount(31, AccountTypeOAuth, "plus")
	highRemaining.Priority = 9
	highRemaining.Extra = map[string]any{"codex_5h_used_percent": 20.0, "codex_usage_updated_at": now.Format(time.RFC3339)}

	svc := newTieredSchedulerTestService(t, []Account{lowRemaining, highRemaining})
	selection, _, err := svc.SelectAccountWithScheduler(context.Background(), nil, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.Equal(t, int64(31), selection.Account.ID)
}
```

- [ ] **Step 2: Add stale and all-stale tests**

Add one test where a fresh PLUS with less remaining quota wins over a stale PLUS, and one where all PLUS snapshots are stale yet a PLUS still wins over a PRO:

```go
func TestOpenAIGatewayService_SelectAccountWithScheduler_PlusFreshSnapshotPrecedesStale(t *testing.T) {
	now := time.Now().UTC()
	stale := newSchedulableOpenAITestAccount(40, AccountTypeOAuth, "plus")
	stale.Extra = map[string]any{"codex_5h_used_percent": 1.0, "codex_usage_updated_at": now.Add(-9 * time.Hour).Format(time.RFC3339)}
	fresh := newSchedulableOpenAITestAccount(41, AccountTypeOAuth, "plus")
	fresh.Extra = map[string]any{"codex_5h_used_percent": 90.0, "codex_usage_updated_at": now.Format(time.RFC3339)}

	svc := newTieredSchedulerTestService(t, []Account{stale, fresh})
	selection, _, err := svc.SelectAccountWithScheduler(context.Background(), nil, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.Equal(t, int64(41), selection.Account.ID)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_AllStalePlusStillPrecedesPro(t *testing.T) {
	now := time.Now().UTC()
	plus := newSchedulableOpenAITestAccount(42, AccountTypeOAuth, "plus")
	plus.Extra = map[string]any{"codex_5h_used_percent": 50.0, "codex_usage_updated_at": now.Add(-9 * time.Hour).Format(time.RFC3339)}
	pro := newSchedulableOpenAITestAccount(43, AccountTypeOAuth, "pro")

	svc := newTieredSchedulerTestService(t, []Account{pro, plus})
	selection, _, err := svc.SelectAccountWithScheduler(context.Background(), nil, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.Equal(t, int64(42), selection.Account.ID)
}
```

- [ ] **Step 3: Add same-tier retry and cross-tier fallback tests**

Extend the existing concurrency mock used by the subscription-priority tests with a deterministic acquisition sequence:

```go
type tieredAcquireRecorder struct {
	failIDs   map[int64]bool
	attempted []int64
}

func (r *tieredAcquireRecorder) acquire(accountID int64) bool {
	r.attempted = append(r.attempted, accountID)
	return !r.failIDs[accountID]
}
```

Wire `acquire` into the mock's existing `TryAcquireAccountSlot` callback, then add these assertions:

```go
func TestOpenAIGatewayService_SelectAccountWithScheduler_TriesSamePlusPoolBeforePro(t *testing.T) {
	recorder := &tieredAcquireRecorder{failIDs: map[int64]bool{50: true}}
	first := newSchedulableOpenAITestAccount(50, AccountTypeOAuth, "plus")
	second := newSchedulableOpenAITestAccount(51, AccountTypeOAuth, "plus")
	pro := newSchedulableOpenAITestAccount(52, AccountTypeOAuth, "pro")
	svc := newTieredSchedulerTestServiceWithAcquireRecorder(t, []Account{first, second, pro}, recorder)

	selection, _, err := svc.SelectAccountWithScheduler(context.Background(), nil, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.Equal(t, int64(51), selection.Account.ID)
	require.Equal(t, []int64{50, 51}, recorder.attempted)
}

func TestOpenAIGatewayService_SelectAccountWithScheduler_FallsBackToProAfterPlusPoolExhausted(t *testing.T) {
	recorder := &tieredAcquireRecorder{failIDs: map[int64]bool{60: true, 61: true}}
	first := newSchedulableOpenAITestAccount(60, AccountTypeOAuth, "plus")
	second := newSchedulableOpenAITestAccount(61, AccountTypeOAuth, "plus")
	pro := newSchedulableOpenAITestAccount(62, AccountTypeOAuth, "pro")
	svc := newTieredSchedulerTestServiceWithAcquireRecorder(t, []Account{first, second, pro}, recorder)

	selection, _, err := svc.SelectAccountWithScheduler(context.Background(), nil, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.Equal(t, int64(62), selection.Account.ID)
	require.Equal(t, []int64{60, 61, 62}, recorder.attempted)
}
```

- [ ] **Step 4: Add sticky PRO preservation test**

Reuse the existing session-sticky test repository. First bind a session hash to a PRO account, then add an available PLUS account and issue the next request with the same session hash:

```go
selection, decision, err := svc.SelectAccountWithScheduler(
	context.Background(), nil, "", "tiered-sticky-session", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false,
)
require.NoError(t, err)
require.Equal(t, pro.ID, selection.Account.ID)
require.Equal(t, openAIAccountScheduleLayerSessionSticky, decision.Layer)
```

Add a companion case where the sticky PRO is runtime-blocked and assert reselection starts at PLUS.

- [ ] **Step 5: Run tests and correct ordering integration**

```powershell
Push-Location backend
go test ./internal/service -run 'TestOpenAIGatewayService_SelectAccountWithScheduler_(Plus|Tiered|Sticky)' -count=1
Pop-Location
```

Expected before the final correction: at least one new behavior test fails. Adjust `buildOpenAISelectionOrder` integration so PLUS hard ordering survives Top-K and weighted randomization. For PLUS, use the already sorted order directly; for PRO, API Key and compatibility pools, retain the existing score/Top-K selection behavior after their pool-specific preordering.

Implement the branch explicitly:

```go
if pool == openAIAccountPoolPlus {
	plan.selectionOrder = append([]openAIAccountCandidateScore(nil), plan.candidates...)
} else {
	plan.selectionOrder = s.buildOpenAISelectionOrder(req, plan)
}
```

Pass the current pool into `buildOpenAIAccountLoadPlan` or apply this order in `tryOpenAIAccountPool`; keep the change local to pool selection.

- [ ] **Step 6: Run the full scheduler test file**

```powershell
Push-Location backend
go test ./internal/service -run 'TestOpenAI|TestOpenAIGatewayService_SelectAccountWithScheduler' -count=1
Pop-Location
```

Expected: PASS.

- [ ] **Step 7: Commit behavioral coverage**

```powershell
git add backend/internal/service/openai_account_scheduler.go backend/internal/service/openai_account_scheduler_test.go backend/internal/service/openai_account_tier.go backend/internal/service/openai_account_tier_test.go
git commit -m "test: cover tiered OpenAI account fallback"
```

---

### Task 8: Add scheduler observability without high-cardinality labels

**Files:**
- Modify: `backend/internal/service/openai_account_scheduler.go`
- Modify: `backend/internal/service/openai_account_scheduler_test.go`

- [ ] **Step 1: Add failing decision tests**

For PLUS, PRO, API Key and compatibility selections, assert:

```go
require.Equal(t, "plus", decision.SelectedPool)
require.Equal(t, "", decision.FallbackFromPool)
require.Equal(t, "fresh", decision.PlusQuotaState)
```

For a PLUS-to-PRO fallback, assert:

```go
require.Equal(t, "pro", decision.SelectedPool)
require.Equal(t, "plus", decision.FallbackFromPool)
require.Equal(t, "", decision.PlusQuotaState)
```

For stale and missing PLUS selections, assert `PlusQuotaState` is respectively `stale` and `missing`.

- [ ] **Step 2: Run observability tests and verify failure**

```powershell
Push-Location backend
go test ./internal/service -run 'TestOpenAIGatewayService_SelectAccountWithScheduler_.*(Decision|Pool|QuotaState)' -count=1
Pop-Location
```

Expected: failure from unset decision fields.

- [ ] **Step 3: Populate decision fields from the internal selection result**

Use helpers that emit only fixed values:

```go
func openAIPlusQuotaStateName(state openAIPlusQuotaState) string {
	switch state {
	case openAIPlusQuotaFresh:
		return "fresh"
	case openAIPlusQuotaStale:
		return "stale"
	default:
		return "missing"
	}
}
```

Populate `SelectedPool`, `FallbackFromPool` and `PlusQuotaState` when constructing `OpenAIAccountScheduleDecision`. Do not add account ID, email, token or model name as a metrics label.

- [ ] **Step 4: Extend existing counters using fixed pool labels**

Add one counter per fixed pool or a bounded map initialized from `openAIAccountPoolOrder`:

```go
type openAIAccountSchedulerMetrics struct {
	// existing counters
	plusSelectTotal   atomic.Int64
	proSelectTotal    atomic.Int64
	apiKeySelectTotal atomic.Int64
	compatSelectTotal atomic.Int64
}
```

Increment exactly one counter from `decision.SelectedPool`. Extend `OpenAIAccountSchedulerMetricsSnapshot` with the four totals and update its snapshot method and tests.

- [ ] **Step 5: Run metrics and decision tests**

```powershell
Push-Location backend
go test ./internal/service -run 'Test.*OpenAIAccountScheduler.*(Metrics|Decision|Pool|QuotaState)' -count=1
Pop-Location
```

Expected: PASS.

- [ ] **Step 6: Commit observability**

```powershell
git add backend/internal/service/openai_account_scheduler.go backend/internal/service/openai_account_scheduler_test.go backend/internal/service/gateway_service.go
git commit -m "feat: expose OpenAI scheduling pool decisions"
```

---

### Task 9: Run complete local regression and inspect migrations

**Files:**
- Verify: `backend/migrations/**`
- Verify: `backend/ent/schema/**`
- Verify: all backend and frontend packages

- [ ] **Step 1: Confirm migration ordering and schema consistency**

```powershell
Get-ChildItem -LiteralPath backend/migrations -Filter '*.sql' |
  Sort-Object Name |
  Select-Object -Last 30 -ExpandProperty Name
git diff 'refs/tags/v0.2.4^{}'..HEAD -- backend/migrations backend/ent/schema
```

Expected: local migrations `145_storefront.sql`, `151_subscription_plan_key_quota.sql` and `152_payment_order_api_key_target.sql` remain present without duplicate numeric prefixes introduced by v0.2.4. If v0.2.4 already uses any of those numbers, rename the local migrations to the next unused sequential numbers and update migration tests or manifests that reference their filenames.

- [ ] **Step 2: Run focused account and scheduler tests without cache**

```powershell
Push-Location backend
go test ./internal/service -run 'Test(OpenAI|AccountSchedulingThreshold|GatewayService_SelectAccount)' -count=1
Pop-Location
```

Expected: PASS.

- [ ] **Step 3: Run all backend tests**

```powershell
make test-backend
```

Expected: `go test ./...` and `golangci-lint run ./...` both pass.

- [ ] **Step 4: Run all configured frontend checks**

```powershell
make test-frontend
```

Expected: lint, typecheck and critical Vitest files pass.

- [ ] **Step 5: Build the complete application**

```powershell
make build
```

Expected: backend and frontend builds pass.

- [ ] **Step 6: Review the branch diff for accidental loss of custom features**

```powershell
git diff --stat main..HEAD
git log --oneline --decorate main..HEAD
rg -n "Storefront|configScriptDownload|ImageGenerationIntent|sanitize.*namespace|tool.*namespace" backend frontend
git diff --check main..HEAD
```

Expected: all named custom features still have source references, the branch contains the upgrade and feature commits, and no whitespace errors are reported.

- [ ] **Step 7: Commit reviewed regression repairs when the index is non-empty**

```powershell
git add --all
git diff --cached --check
git commit -m "fix: complete v0.2.4 scheduling regression"
```

Create this commit only when reviewed repair files are staged.

---

### Task 10: Prepare the local delivery report

**Files:**
- Verify only; do not update production in this task.

- [ ] **Step 1: Capture final revision and test evidence**

```powershell
git status --short --branch
git log --oneline --decorate -10
git rev-parse HEAD
git diff --stat main..HEAD
```

Expected: clean working tree on `codex/upgrade-v0.2.4-tiered-openai-scheduling`.

- [ ] **Step 2: Summarize behavior with explicit evidence**

The delivery report must state:

```text
Upgrade base: Sub2API v0.2.4
New-session order: PLUS -> PRO -> API Key -> compatibility pool
PLUS ordering: fresh snapshot first, then greater five-hour remaining quota
PRO behavior: five-hour quota ignored
Sticky behavior: existing bound account retained until it becomes ineligible
Fallback behavior: exhaust same pool before entering the next pool
Validation: focused scheduler tests, backend suite, frontend checks, complete build
Production state: unchanged
```

- [ ] **Step 3: Stop before production changes**

The project rules require a separate confirmation before image pull, container recreation, restart, `.env` changes or production migration. End this implementation phase after local verification and present the branch, commits and test evidence for review.

---

## Plan self-review result

- Spec coverage: upgrade preservation, four-pool order, PLUS quota ordering, PRO quota exemption, stale/missing handling, sticky behavior, same-tier retry, compatibility fallback, observability and full regression each map to a task.
- Type consistency: pool names and quota states are defined once in `openai_account_tier.go` and reused by scheduler decisions and tests.
- Scope: production deployment remains a separate confirmed phase because it includes image pull, container update and possible forward-only migrations.
- Placeholder scan: all implementation steps name concrete files, symbols, commands and expected outcomes.
