package service

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

func imageReplayTestContext(userID, keyID int64) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("api_key", &APIKey{ID: keyID, UserID: userID})
	return c
}
func imageReplayTestService(t *testing.T) *OpenAIGatewayService {
	t.Helper()
	cache := newOpenAIImageReplayCache(time.Hour, 4096, 2048, 1024, 20)
	t.Cleanup(cache.close)
	return &OpenAIGatewayService{openaiImageReplayCache: cache}
}
func TestOpenAIImageReplayHydratesWithoutChangingOtherInput(t *testing.T) {
	s := imageReplayTestService(t)
	c := imageReplayTestContext(1, 2)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	s.rememberOpenAIImageReplay(c, account, []byte(`{"type":"response.output_item.done","item":{"type":"image_generation_call","id":"ig_one","status":"completed","result":"cG5n","revised_prompt":"a cat"}}`), "")
	body := []byte(`{"model":"gpt-6.1-sol","store":false,"nonce":900719925474099312345,"input":[{"type":"function_call","call_id":"call_keep","name":"echo","arguments":"{}"},{"type":"function_call_output","call_id":"call_keep","output":"ok"},{"type":"image_generation_call","id":"ig_one","status":"completed"},{"role":"user","content":"continue"}]}`)
	out, changed, err := s.prepareOpenAIImageReplay(c, account, body)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "cG5n", gjson.GetBytes(out, "input.2.result").String())
	require.Equal(t, "900719925474099312345", gjson.GetBytes(out, "nonce").Raw)
	for _, path := range []string{"input.0", "input.1", "input.3"} {
		require.Equal(t, gjson.GetBytes(body, path).Raw, gjson.GetBytes(out, path).Raw)
	}
	require.False(t, gjson.GetBytes(out, "store").Bool())
}
func TestOpenAIImageReplayRecoveryAndItemReference(t *testing.T) {
	s := imageReplayTestService(t)
	c := imageReplayTestContext(1, 2)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	full := []byte(`{"input":[{"type":"image_generation_call","id":"ig_recover","status":"completed","result":"cmVhbA=="}]}`)
	out, changed, err := s.prepareOpenAIImageReplay(c, account, full)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, full, out)
	out, changed, err = s.prepareOpenAIImageReplay(c, account, []byte(`{"input":[{"type":"item_reference","id":"ig_recover"}]}`))
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "image_generation_call", gjson.GetBytes(out, "input.0.type").String())
	require.Equal(t, "cmVhbA==", gjson.GetBytes(out, "input.0.result").String())
}
func TestOpenAIImageReplayScopeAndMissingData(t *testing.T) {
	s := imageReplayTestService(t)
	c := imageReplayTestContext(1, 2)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	s.rememberOpenAIImageReplay(c, account, []byte(`{"output":[{"type":"image_generation_call","id":"ig_private","result":"cHJpdmF0ZQ=="}]}`), "")
	body := []byte(`{"input":[{"type":"item_reference","id":"ig_private"}]}`)
	for _, other := range []*gin.Context{imageReplayTestContext(9, 2), imageReplayTestContext(1, 9)} {
		out, changed, err := s.prepareOpenAIImageReplay(other, account, body)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Reattach")
		require.False(t, changed)
		require.Equal(t, body, out)
	}
	stored := []byte(`{"store":true,"input":[{"type":"item_reference","id":"ig_private"}]}`)
	out, changed, err := s.prepareOpenAIImageReplay(c, &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, stored)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, stored, out)
	linked := []byte(`{"previous_response_id":"resp_live","input":[{"type":"item_reference","id":"ig_missing"}]}`)
	out, changed, err = s.prepareOpenAIImageReplay(c, account, linked)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, linked, out)
}
func TestOpenAIImageReplayNoScopeAndNoImageAreNoOps(t *testing.T) {
	s := imageReplayTestService(t)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	for _, body := range []string{`{"input":"hi"}`, `{"input":[{"type":"item_reference","id":"fc_keep"}]}`, `{"input":[{"role":"user","content":"image_generation_call ig_fake"}]}`} {
		out, changed, err := s.prepareOpenAIImageReplay(imageReplayTestContext(1, 2), account, []byte(body))
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, body, string(out))
	}
	body := []byte(`{"input":[{"type":"image_generation_call","id":"ig_missing"}]}`)
	out, changed, err := s.prepareOpenAIImageReplay(nil, account, body)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, body, out)
}
func TestOpenAIImageReplayCacheLimitsTTLAndConcurrentAccess(t *testing.T) {
	cache := newOpenAIImageReplayCache(time.Hour, 12, 8, 6, 3)
	t.Cleanup(cache.close)
	now := time.Now()
	cache.now = func() time.Time { return now }
	a := openAIImageReplayScope{userID: 1, apiKeyID: 1}
	b := openAIImageReplayScope{userID: 2, apiKeyID: 2}
	require.True(t, cache.put(a, "ig_a", []byte("1111")))
	require.True(t, cache.put(a, "ig_b", []byte("2222")))
	_, ok := cache.get(a, "ig_a")
	require.True(t, ok)
	require.True(t, cache.put(a, "ig_c", []byte("3333")))
	_, ok = cache.get(a, "ig_b")
	require.False(t, ok, "scope LRU")
	require.True(t, cache.put(b, "ig_d", []byte("4444")))
	require.False(t, cache.put(a, "ig_large", []byte("1234567")))
	_, ok = cache.get(b, "ig_a")
	require.False(t, ok)
	now = now.Add(time.Hour)
	_, ok = cache.get(a, "ig_a")
	require.False(t, ok, "expiry boundary")
	cache.mu.Lock()
	storedBytes := cache.bytes
	cache.mu.Unlock()
	require.LessOrEqual(t, storedBytes, 12)
	parallel := newOpenAIImageReplayCache(time.Minute, 4096, 2048, 1024, 30)
	t.Cleanup(parallel.close)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				parallel.put(a, "ig_x", []byte(`{"result":"data"}`))
				raw, hit := parallel.get(a, "ig_x")
				if hit && !json.Valid(raw) {
					t.Error("invalid cache payload")
				}
			}
		}()
	}
	wg.Wait()
}

func TestOpenAIImageReplayRealRecoveryFixture(t *testing.T) {
	path := os.Getenv("SUB2API_IMAGE_REPLAY_RECOVERY_PAYLOAD")
	if path == "" {
		t.Skip("optional local recovery fixture")
	}
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(body, "store").Bool())
	originalResult := gjson.GetBytes(body, "input.0.result").String()
	require.NotEmpty(t, originalResult)
	s := &OpenAIGatewayService{}
	t.Cleanup(func() {
		if s.openaiImageReplayCache != nil {
			s.openaiImageReplayCache.close()
		}
	})
	c := imageReplayTestContext(1, 1)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	out, changed, err := s.prepareOpenAIImageReplay(c, account, body)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, body, out)
	missing, err := sjson.DeleteBytes(body, "input.0.result")
	require.NoError(t, err)
	out, changed, err = s.prepareOpenAIImageReplay(c, account, missing)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, originalResult, gjson.GetBytes(out, "input.0.result").String())
	require.Equal(t, gjson.GetBytes(body, "input.0.id").String(), gjson.GetBytes(out, "input.0.id").String())
	_, _, err = s.prepareOpenAIImageReplay(imageReplayTestContext(1, 9), account, missing)
	require.ErrorIs(t, err, errOpenAIImageReplayUnavailable)
}
func TestOpenAIImageReplayTimerExpiryAndDefensiveCopies(t *testing.T) {
	cache := newOpenAIImageReplayCache(200*time.Millisecond, 32, 32, 16, 2)
	t.Cleanup(cache.close)
	scope := openAIImageReplayScope{userID: 1, apiKeyID: 2}
	input := []byte("image")
	require.True(t, cache.put(scope, "ig_one", input))
	input[0] = 'X'
	out, ok := cache.get(scope, "ig_one")
	require.True(t, ok)
	require.Equal(t, "image", string(out))
	out[0] = 'Y'
	again, ok := cache.get(scope, "ig_one")
	require.True(t, ok)
	require.Equal(t, "image", string(again))
	require.Eventually(t, func() bool {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		return len(cache.entries) == 0 && cache.bytes == 0 && len(cache.scopeBytes) == 0
	}, time.Second, time.Millisecond, "expired images must be removed even without another request")
}

func TestOpenAIImageReplayMissingDoesNotPenalizeUpstreamHealth(t *testing.T) {
	s := &OpenAIGatewayService{}
	account := &Account{ID: 9, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	require.False(t, s.ReportOpenAIAccountScheduleResult(account, "gpt-6.1-sol", false, nil, fmt.Errorf("client context: %w", errOpenAIImageReplayUnavailable)))
	require.Nil(t, s.openaiScheduler, "a local cache miss must not create/update scheduler health")
	require.Nil(t, s.openaiAccountStats)
}
func TestOpenAIImageReplayEscapedImageReference(t *testing.T) {
	s := imageReplayTestService(t)
	c := imageReplayTestContext(1, 2)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	full := []byte(`{"input":[{"type":"image_generation_call","id":"ig_escape","result":"cG5n"}]}`)
	_, _, err := s.prepareOpenAIImageReplay(c, account, full)
	require.NoError(t, err)
	out, changed, err := s.prepareOpenAIImageReplay(c, account, []byte(`{"input":[{"type":"item_reference","id":"\u0069g_escape"}]}`))
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "cG5n", gjson.GetBytes(out, "input.0.result").String())
}
