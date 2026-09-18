package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIAccountPoolFor(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
		want    openAIAccountPool
	}{
		{name: "plus", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": " PLUS "}}, want: openAIAccountPoolPlus},
		{name: "team", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "Team"}}, want: openAIAccountPoolPlus},
		{name: "pro", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "Pro"}}, want: openAIAccountPoolPro},
		{name: "chatgptpro", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "chatgptpro"}}, want: openAIAccountPoolPro},
		{name: "chatgpt pro alias", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "chatgpt_pro"}}, want: openAIAccountPoolPro},
		{name: "prolite", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "prolite"}}, want: openAIAccountPoolPro},
		{name: "team pro", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "self_serve_business_prolite"}}, want: openAIAccountPoolPro},
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
		{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "self_serve_business_prolite"}},
		{ID: 6, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": "business"}},
	}

	pools := partitionOpenAIAccountsByPool(accounts)
	require.Equal(t, []int64{3, 4}, accountIDsForTierTest(pools[openAIAccountPoolPlus]))
	require.Equal(t, []int64{2, 5}, accountIDsForTierTest(pools[openAIAccountPoolPro]))
	require.Equal(t, []int64{1}, accountIDsForTierTest(pools[openAIAccountPoolAPIKey]))
	require.Equal(t, []int64{6}, accountIDsForTierTest(pools[openAIAccountPoolCompat]))
}

func TestEvaluateAccountSchedulingThresholdTeamProIgnoresFiveHourQuota(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	teamPro := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"plan_type": "self_serve_business_prolite",
		},
		Extra: map[string]any{
			"codex_5h_used_percent": 100.0,
			"codex_5h_reset_at":     now.Add(2 * time.Hour).Format(time.RFC3339),
		},
	}

	decision := EvaluateAccountSchedulingThreshold(teamPro, map[string]int{PlatformOpenAI: 80}, now)
	require.False(t, decision.ShouldPause)
}

func TestOpenAIPlusQuotaRank(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	fresh := &Account{Extra: map[string]any{
		"codex_5h_used_percent":  25.0,
		"codex_usage_updated_at": now.Add(-time.Minute).Format(time.RFC3339),
	}}
	stale := &Account{Extra: map[string]any{
		"codex_5h_used_percent":  10.0,
		"codex_usage_updated_at": now.Add(-9 * time.Hour).Format(time.RFC3339),
	}}

	require.Equal(t, openAIPlusQuotaFresh, openAIPlusQuotaRankFor(fresh, now).State)
	require.InDelta(t, 75.0, openAIPlusQuotaRankFor(fresh, now).RemainingPercent, 0.001)
	require.Equal(t, openAIPlusQuotaStale, openAIPlusQuotaRankFor(stale, now).State)
	require.Equal(t, openAIPlusQuotaMissing, openAIPlusQuotaRankFor(&Account{}, now).State)
	require.True(t, openAIPlusAccountHasHeadroom(stale, now))
	require.False(t, openAIPlusAccountHasHeadroom(&Account{Extra: map[string]any{
		"codex_5h_used_percent":  100.0,
		"codex_usage_updated_at": now.Add(-time.Minute).Format(time.RFC3339),
	}}, now))
}

func TestOpenAIPlusLongQuotaExhausted(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		extra map[string]any
		want  bool
	}{
		{
			name: "weekly one percent remaining",
			extra: map[string]any{
				"codex_7d_used_percent": 99.0,
				"codex_7d_reset_at":     now.Add(24 * time.Hour).Format(time.RFC3339),
			},
			want: true,
		},
		{
			name: "monthly one percent remaining",
			extra: map[string]any{
				"codex_monthly_used_percent": 99.5,
				"codex_30d_reset_at":         now.Add(10 * 24 * time.Hour).Format(time.RFC3339),
			},
			want: true,
		},
		{
			name: "weekly reset does not block",
			extra: map[string]any{
				"codex_7d_used_percent": 100.0,
				"codex_7d_reset_at":     now.Add(-time.Minute).Format(time.RFC3339),
			},
			want: false,
		},
		{
			name: "two percent remaining stays eligible",
			extra: map[string]any{
				"codex_7d_used_percent": 98.0,
				"codex_7d_reset_at":     now.Add(24 * time.Hour).Format(time.RFC3339),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{Extra: tt.extra}
			require.Equal(t, tt.want, openAIPlusLongQuotaExhausted(account, now))
			require.Equal(t, !tt.want, openAIPlusAccountHasHeadroom(account, now))
		})
	}
}

func TestSortOpenAIAccountCandidatesForPool(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	freshLow := &Account{ID: 2, Priority: 5, Extra: map[string]any{
		"codex_5h_used_percent":  60.0,
		"codex_usage_updated_at": now.Add(-time.Minute).Format(time.RFC3339),
	}}
	freshHigh := &Account{ID: 3, Priority: 9, Extra: map[string]any{
		"codex_5h_used_percent":  20.0,
		"codex_usage_updated_at": now.Add(-time.Minute).Format(time.RFC3339),
	}}
	stale := &Account{ID: 1, Priority: 0, Extra: map[string]any{
		"codex_5h_used_percent":  5.0,
		"codex_usage_updated_at": now.Add(-9 * time.Hour).Format(time.RFC3339),
	}}
	candidates := []openAIAccountCandidateScore{
		{account: stale, loadInfo: &AccountLoadInfo{}, priority: stale.Priority},
		{account: freshLow, loadInfo: &AccountLoadInfo{}, priority: freshLow.Priority},
		{account: freshHigh, loadInfo: &AccountLoadInfo{LoadRate: 99}, priority: freshHigh.Priority},
	}

	sortOpenAIAccountCandidatesForPool(candidates, openAIAccountPoolPlus, now)
	require.Equal(t, []int64{3, 2, 1}, accountIDsForTierCandidates(candidates))
}

func TestSortOpenAIAccountCandidatesForPoolProIgnoresFiveHourQuota(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	lowPriority := &Account{ID: 1, Priority: 5, Extra: map[string]any{"codex_5h_used_percent": 0.0}}
	highPriority := &Account{ID: 2, Priority: 0, Extra: map[string]any{"codex_5h_used_percent": 100.0}}
	candidates := []openAIAccountCandidateScore{
		{account: lowPriority, loadInfo: &AccountLoadInfo{LoadRate: 0}, priority: lowPriority.Priority},
		{account: highPriority, loadInfo: &AccountLoadInfo{LoadRate: 90}, priority: highPriority.Priority},
	}

	sortOpenAIAccountCandidatesForPool(candidates, openAIAccountPoolPro, now)
	require.Equal(t, []int64{2, 1}, accountIDsForTierCandidates(candidates))
}

func TestEvaluateAccountSchedulingThresholdProIgnoresFiveHourQuota(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	pro := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"plan_type": "pro",
		},
		Extra: map[string]any{
			"codex_5h_used_percent": 100.0,
			"codex_5h_reset_at":     now.Add(2 * time.Hour).Format(time.RFC3339),
		},
	}

	decision := EvaluateAccountSchedulingThreshold(pro, map[string]int{PlatformOpenAI: 80}, now)
	require.False(t, decision.ShouldPause)
}

func accountIDsForTierTest(accounts []*Account) []int64 {
	ids := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.ID)
	}
	return ids
}

func accountIDsForTierCandidates(candidates []openAIAccountCandidateScore) []int64 {
	ids := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.account.ID)
	}
	return ids
}
