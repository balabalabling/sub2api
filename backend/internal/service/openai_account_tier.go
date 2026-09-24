package service

import (
	"sort"
	"strings"
	"time"
)

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

const (
	// Preserve the final 1% of a PLUS long-window quota. Quota snapshots are
	// learned from upstream responses, so admission needs one extra percentage
	// point of headroom to keep a request admitted at 98% from consuming through
	// the protected reserve before the next snapshot is persisted.
	openAIPlusLongQuotaProtectedReservePercent     = 1.0
	openAIPlusLongQuotaSnapshotLagBufferPercent    = 1.0
	openAIPlusLongQuotaMinAdmissionHeadroomPercent = openAIPlusLongQuotaProtectedReservePercent + openAIPlusLongQuotaSnapshotLagBufferPercent
)

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
	switch normalizeOpenAIPlanType(account.GetCredential("plan_type")) {
	case "plus", "team":
		return openAIAccountPoolPlus
	case "pro", "chatgptpro", "prolite", "selfservebusinessprolite":
		return openAIAccountPoolPro
	default:
		return openAIAccountPoolCompat
	}
}

// normalizeOpenAIPlanType follows the frontend's official ChatGPT plan aliases
// while keeping the upstream credential value unchanged.
func normalizeOpenAIPlanType(value string) string {
	return strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(value)))
}

func partitionOpenAIAccountsByPool(accounts []*Account) map[openAIAccountPool][]*Account {
	pools := make(map[openAIAccountPool][]*Account, len(openAIAccountPoolOrder))
	for _, account := range accounts {
		pools[openAIAccountPoolFor(account)] = append(pools[openAIAccountPoolFor(account)], account)
	}
	return pools
}

func openAIPlusQuotaRankFor(account *Account, now time.Time) openAIPlusQuotaRank {
	if account == nil || len(account.Extra) == 0 {
		return openAIPlusQuotaRank{State: openAIPlusQuotaMissing}
	}
	used, ok := resolveAccountExtraNumber(account.Extra, "codex_5h_used_percent", "codex_secondary_used_percent")
	if !ok {
		return openAIPlusQuotaRank{State: openAIPlusQuotaMissing}
	}
	window5h, _ := openAICanonicalQuotaWindows(account.Extra, now)
	if window5h.reset || openAIQuotaHeadroomSnapshotStale(account.Extra, now) {
		return openAIPlusQuotaRank{State: openAIPlusQuotaStale}
	}
	return openAIPlusQuotaRank{
		State:            openAIPlusQuotaFresh,
		RemainingPercent: 100 * (1 - clamp01(used/100)),
	}
}

// openAIPlusAccountHasHeadroom keeps the tiered sticky-selection hook in place
// while allowing PLUS accounts to continue receiving requests after their
// locally cached quota reaches 99% or 100%. The upstream account remains the
// source of truth for any hard rejection; the scheduler no longer preemptively
// removes it from the PLUS pool.
func openAIPlusAccountHasHeadroom(account *Account, now time.Time) bool {
	return true
}

// openAIPlusLongQuotaExhausted reports the legacy admission-reserve condition
// for diagnostics and regression coverage. It is intentionally no longer used
// to gate scheduling: cached quota metadata does not preemptively stop PLUS
// traffic.
func openAIPlusLongQuotaExhausted(account *Account, now time.Time) bool {
	if account == nil || len(account.Extra) == 0 {
		return false
	}
	windows := []struct {
		name string
		keys []string
	}{
		{name: "7d", keys: []string{"codex_7d_used_percent", "codex_weekly_used_percent"}},
		{name: "30d", keys: []string{"codex_30d_used_percent", "codex_monthly_used_percent"}},
	}
	for _, window := range windows {
		used, ok := resolveAccountExtraNumber(account.Extra, window.keys...)
		if !ok || openAIQuotaWindowReset(account.Extra, window.name, now) {
			continue
		}
		usedPercent := 100 * clamp01(used/100)
		if usedPercent >= 100-openAIPlusLongQuotaMinAdmissionHeadroomPercent {
			return true
		}
	}
	return false
}

func openAIAccountCandidateBaseBetter(left, right openAIAccountCandidateScore) bool {
	if left.priority != right.priority {
		return left.priority < right.priority
	}
	if left.loadInfo.LoadRate != right.loadInfo.LoadRate {
		return left.loadInfo.LoadRate < right.loadInfo.LoadRate
	}
	if left.loadInfo.WaitingCount != right.loadInfo.WaitingCount {
		return left.loadInfo.WaitingCount < right.loadInfo.WaitingCount
	}
	if left.errorRate != right.errorRate {
		return left.errorRate < right.errorRate
	}
	if left.hasTTFT && right.hasTTFT && left.ttft != right.ttft {
		return left.ttft < right.ttft
	}
	if left.hasTTFT != right.hasTTFT {
		return left.hasTTFT
	}
	return left.account.ID < right.account.ID
}

func sortOpenAIAccountCandidatesForPool(candidates []openAIAccountCandidateScore, pool openAIAccountPool, now time.Time) {
	if pool != openAIAccountPoolPlus && pool != openAIAccountPoolPro {
		return
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if pool == openAIAccountPoolPlus {
			leftQuota := openAIPlusQuotaRankFor(left.account, now)
			rightQuota := openAIPlusQuotaRankFor(right.account, now)
			if leftQuota.State != rightQuota.State {
				return leftQuota.State < rightQuota.State
			}
			if leftQuota.State == openAIPlusQuotaFresh && leftQuota.RemainingPercent != rightQuota.RemainingPercent {
				return leftQuota.RemainingPercent > rightQuota.RemainingPercent
			}
		}
		return openAIAccountCandidateBaseBetter(left, right)
	})
}

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
