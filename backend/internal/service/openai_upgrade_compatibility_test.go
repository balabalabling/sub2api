package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIUpgradePlanPoolIdentity(t *testing.T) {
	for _, tc := range []struct {
		plan string
		pool openAIAccountPool
	}{
		{"promax", openAIAccountPoolPro},
		{"Pro_Max", openAIAccountPoolPro},
		{"self_serve_business_usage_based", openAIAccountPoolCompat},
		{"business", openAIAccountPoolCompat},
		{"enterprise", openAIAccountPoolCompat},
		{"unknown", openAIAccountPoolCompat},
	} {
		t.Run(tc.plan, func(t *testing.T) {
			account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"plan_type": tc.plan}}
			require.Equal(t, tc.pool, openAIAccountPoolFor(account))
		})
	}
}

func TestOpenAIUpgradeProMaxIgnoresFiveHourQuota(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	account := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"plan_type": "promax"},
		Extra: map[string]any{
			"codex_5h_used_percent": 100.0,
			"codex_5h_reset_at":     now.Add(time.Hour).Format(time.RFC3339),
		},
	}
	decision := EvaluateAccountSchedulingThreshold(account, map[string]int{PlatformOpenAI: 80}, now)
	require.False(t, decision.ShouldPause)
}

func TestOpenAIUpgradeRouteAndCompositeOwnershipAreBothRequired(t *testing.T) {
	for _, tc := range []struct {
		name       string
		allowed    map[int64]struct{}
		mapping    map[string]any
		composite  bool
		wantReason string
	}{
		{name: "owner outside local route", allowed: map[int64]struct{}{2: {}}, mapping: map[string]any{"public-model": "upstream-model"}, composite: true, wantReason: "route_filtered"},
		{name: "local route member does not own alias", allowed: map[int64]struct{}{1: {}}, composite: true, wantReason: "account_model_not_owned"},
		{name: "local route filter without composite", allowed: map[int64]struct{}{2: {}}, wantReason: "route_filtered"},
		{name: "owner inside local route", allowed: map[int64]struct{}{1: {}}, mapping: map[string]any{"public-model": "upstream-model"}, composite: true},
		{name: "empty local filter still enforces ownership", composite: true, wantReason: "account_model_not_owned"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.composite {
				ctx = WithCompositeRouteDecision(ctx, CompositeRouteDecision{Matched: true, TargetPlatform: PlatformOpenAI, PublicModel: "public-model", UpstreamModel: "upstream-model", Source: CompositeRouteSourceAccount})
			}
			account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": tc.mapping}}
			scheduler := &defaultOpenAIAccountScheduler{}
			compatible, reason := scheduler.isAccountRequestCompatibleReason(ctx, account, OpenAIAccountScheduleRequest{AllowedAccountIDs: tc.allowed})
			require.Equal(t, tc.wantReason, reason)
			require.Equal(t, tc.wantReason == "", compatible)
		})
	}
}
