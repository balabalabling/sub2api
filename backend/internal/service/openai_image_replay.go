package service

import (
	"bytes"
	"container/list"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// store=false forbids upstream item lookup, not self-contained image replay.
// This is deliberately a small, ephemeral, process-local cache, never a store
// of request bodies or credentials. A complete input item can warm it after a
// restart, using the same authenticated user/API key that will continue the turn.
const openAIImageReplayContextKey = "openai_image_replay_enabled"

var errOpenAIImageReplayUnavailable = errors.New("generated image data is unavailable in the short-lived gateway cache. Reattach the complete image data or restart before the image-generation step")

type openAIImageReplayScope struct{ userID, apiKeyID int64 }
type openAIImageReplayKey struct {
	scope openAIImageReplayScope
	id    string
}
type openAIImageReplayEntry struct {
	key        openAIImageReplayKey
	raw        []byte
	expiresAt  time.Time
	generation uint64
	timer      *time.Timer
}
type openAIImageReplayCache struct {
	mu                                                sync.Mutex
	entries                                           map[openAIImageReplayKey]*list.Element
	lru                                               *list.List
	scopeBytes                                        map[openAIImageReplayScope]int
	bytes                                             int
	generation                                        uint64
	ttl                                               time.Duration
	maxBytes, maxScopeBytes, maxItemBytes, maxEntries int
	now                                               func() time.Time
}

func newOpenAIImageReplayCache(ttl time.Duration, maxBytes, maxScopeBytes, maxItemBytes, maxEntries int) *openAIImageReplayCache {
	return &openAIImageReplayCache{entries: make(map[openAIImageReplayKey]*list.Element), lru: list.New(), scopeBytes: make(map[openAIImageReplayScope]int), ttl: ttl, maxBytes: maxBytes, maxScopeBytes: maxScopeBytes, maxItemBytes: maxItemBytes, maxEntries: maxEntries, now: time.Now}
}

// The private list only contains entries inserted by put. Check that invariant
// explicitly rather than relying on unchecked container/list type assertions.
func openAIImageReplayEntryFromElement(elem *list.Element) *openAIImageReplayEntry {
	entry, ok := elem.Value.(*openAIImageReplayEntry)
	if !ok || entry == nil {
		panic("invalid image replay cache entry")
	}
	return entry
}
func (cache *openAIImageReplayCache) removeLocked(elem *list.Element) {
	entry := openAIImageReplayEntryFromElement(elem)
	if entry.timer != nil {
		entry.timer.Stop()
	}
	delete(cache.entries, entry.key)
	cache.lru.Remove(elem)
	size := len(entry.raw)
	cache.bytes -= size
	cache.scopeBytes[entry.key.scope] -= size
	if cache.scopeBytes[entry.key.scope] == 0 {
		delete(cache.scopeBytes, entry.key.scope)
	}
	entry.raw = nil
}
func (cache *openAIImageReplayCache) pruneLocked(now time.Time) {
	for elem := cache.lru.Back(); elem != nil; {
		prev := elem.Prev()
		if !now.Before(openAIImageReplayEntryFromElement(elem).expiresAt) {
			cache.removeLocked(elem)
		}
		elem = prev
	}
}
func (cache *openAIImageReplayCache) put(scope openAIImageReplayScope, id string, raw []byte) bool {
	if cache == nil || scope.userID <= 0 || scope.apiKeyID <= 0 || id == "" || len(id) > 128 || len(raw) == 0 || len(raw) > cache.maxItemBytes || len(raw) > cache.maxScopeBytes || len(raw) > cache.maxBytes || cache.maxEntries <= 0 || cache.ttl <= 0 {
		return false
	}
	key := openAIImageReplayKey{scope: scope, id: strings.Clone(id)}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	now := cache.now()
	cache.pruneLocked(now)
	if old := cache.entries[key]; old != nil {
		cache.removeLocked(old)
	}
	// A noisy principal first evicts its own LRU items, not another user's.
	for cache.scopeBytes[scope]+len(raw) > cache.maxScopeBytes {
		for elem := cache.lru.Back(); elem != nil; elem = elem.Prev() {
			if openAIImageReplayEntryFromElement(elem).key.scope == scope {
				cache.removeLocked(elem)
				break
			}
		}
	}
	for cache.bytes+len(raw) > cache.maxBytes || len(cache.entries) >= cache.maxEntries {
		cache.removeLocked(cache.lru.Back())
	}
	cache.generation++
	generation := cache.generation
	entry := &openAIImageReplayEntry{key: key, raw: bytes.Clone(raw), expiresAt: now.Add(cache.ttl), generation: generation}
	elem := cache.lru.PushFront(entry)
	cache.entries[key] = elem
	cache.bytes += len(raw)
	cache.scopeBytes[scope] += len(raw)
	// Stop timers when entries are replaced/evicted. The closure retains only
	// the key/generation, not image data; expiry releases data even when idle.
	entry.timer = time.AfterFunc(cache.ttl, func() {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		if elem := cache.entries[key]; elem != nil && openAIImageReplayEntryFromElement(elem).generation == generation {
			cache.removeLocked(elem)
		}
	})
	return true
}
func (cache *openAIImageReplayCache) get(scope openAIImageReplayScope, id string) ([]byte, bool) {
	if cache == nil {
		return nil, false
	}
	key := openAIImageReplayKey{scope: scope, id: id}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	elem := cache.entries[key]
	if elem == nil {
		return nil, false
	}
	entry := openAIImageReplayEntryFromElement(elem)
	if !cache.now().Before(entry.expiresAt) {
		cache.removeLocked(elem)
		return nil, false
	}
	cache.lru.MoveToFront(elem)
	return bytes.Clone(entry.raw), true
}
func (cache *openAIImageReplayCache) close() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	for cache.lru.Len() > 0 {
		cache.removeLocked(cache.lru.Back())
	}
}
func (s *OpenAIGatewayService) getOpenAIImageReplayCache() *openAIImageReplayCache {
	s.openaiImageReplayOnce.Do(func() {
		if s.openaiImageReplayCache == nil {
			s.openaiImageReplayCache = newOpenAIImageReplayCache(30*time.Minute, 64<<20, 32<<20, 16<<20, 256)
		}
	})
	return s.openaiImageReplayCache
}
func openAIImageReplayScopeFromContext(c *gin.Context) (openAIImageReplayScope, bool) {
	if c == nil {
		return openAIImageReplayScope{}, false
	}
	key := getAPIKeyFromContext(c)
	if key == nil || key.ID <= 0 || key.UserID <= 0 {
		return openAIImageReplayScope{}, false
	}
	return openAIImageReplayScope{userID: key.UserID, apiKeyID: key.ID}, true
}
func openAIImageReplayEnabled(account *Account, body []byte) bool {
	return account != nil && account.IsOpenAI() && (account.IsOpenAIOAuthLike() || gjson.GetBytes(body, "store").Type == gjson.False)
}
func (s *OpenAIGatewayService) cacheOpenAIImageReplayItem(scope openAIImageReplayScope, item gjson.Result) {
	if !item.IsObject() || item.Get("type").String() != "image_generation_call" {
		return
	}
	id := item.Get("id").String()
	result := item.Get("result")
	if !strings.HasPrefix(id, "ig_") || len(id) > 128 || result.Type != gjson.String || strings.TrimSpace(result.String()) == "" || len(result.String()) > 16<<20 {
		return
	}
	switch item.Get("status").String() {
	case "failed", "cancelled":
		return
	}
	// Bound auxiliary prompt data before serializing a large image item.
	if len(result.Raw)+len(item.Get("revised_prompt").Raw) > 16<<20 {
		return
	}
	// Whitelist replay fields: internal metadata, URLs and credentials never
	// become cache state. Missing/in_progress terminal statuses are completed.
	replay := map[string]any{"type": "image_generation_call", "id": id, "status": "completed", "result": result.String()}
	if prompt := item.Get("revised_prompt"); prompt.Type == gjson.String {
		replay["revised_prompt"] = prompt.String()
	}
	raw, err := json.Marshal(replay)
	if err == nil {
		s.getOpenAIImageReplayCache().put(scope, id, raw)
	}
}
func (s *OpenAIGatewayService) prepareOpenAIImageReplay(c *gin.Context, account *Account, body []byte) ([]byte, bool, error) {
	if s == nil {
		return body, false, nil
	}
	enabled := openAIImageReplayEnabled(account, body)
	if c != nil {
		c.Set(openAIImageReplayContextKey, enabled)
	}
	scope, scoped := openAIImageReplayScopeFromContext(c)
	if !enabled || !scoped {
		return body, false, nil
	}
	// Avoid parsing ordinary text/large tool payloads on the normal hot path.
	if !bytes.Contains(body, []byte(`"image_generation_call"`)) && !bytes.Contains(body, []byte(`"item_reference"`)) {
		return body, false, nil
	}
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return body, false, nil
	}
	items := input.Array()
	// Seed all complete items first, so references can precede the complete item.
	for _, item := range items {
		s.cacheOpenAIImageReplayItem(scope, item)
	}
	hasPrevious := strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String()) != ""
	rebuilt := make([]string, 0, len(items))
	changed := false
	for _, item := range items {
		typ := item.Get("type").String()
		id := item.Get("id").String()
		needsImage := typ == "image_generation_call" && (item.Get("result").Type != gjson.String || strings.TrimSpace(item.Get("result").String()) == "")
		reference := typ == "item_reference" && strings.HasPrefix(id, "ig_")
		if !needsImage && !reference {
			rebuilt = append(rebuilt, item.Raw)
			continue
		}
		cached, hit := s.getOpenAIImageReplayCache().get(scope, id)
		if !hit {
			// A live WS/previous-response context can still resolve the item remotely.
			// Do not reject a legitimate connected continuation solely on a cache miss.
			if hasPrevious {
				rebuilt = append(rebuilt, item.Raw)
				continue
			}
			return body, false, errOpenAIImageReplayUnavailable
		}
		if reference {
			rebuilt = append(rebuilt, string(cached))
		} else {
			updated, err := sjson.SetRawBytes([]byte(item.Raw), "result", []byte(gjson.GetBytes(cached, "result").Raw))
			if err != nil {
				return body, false, fmt.Errorf("restore image replay result: %w", err)
			}
			rebuilt = append(rebuilt, string(updated))
		}
		changed = true
	}
	if !changed {
		return body, false, nil
	}
	return replaceOpenAIRawInput(body, input, rebuilt), true, nil
}
func (s *OpenAIGatewayService) rememberOpenAIImageReplay(c *gin.Context, account *Account, payload []byte, eventType string) {
	if s == nil || account == nil || !account.IsOpenAI() {
		return
	}
	scope, ok := openAIImageReplayScopeFromContext(c)
	if !ok {
		return
	}
	enabled, _ := c.Get(openAIImageReplayContextKey)
	if !account.IsOpenAIOAuthLike() && enabled != true {
		return
	}
	if !bytes.Contains(payload, []byte(`"image_generation_call"`)) {
		return
	}
	if typ := gjson.GetBytes(payload, "type").String(); typ != "" {
		eventType = typ
	}
	switch eventType {
	case "response.output_item.done":
		s.cacheOpenAIImageReplayItem(scope, gjson.GetBytes(payload, "item"))
	case "response.completed", "response.done":
		for _, item := range gjson.GetBytes(payload, "response.output").Array() {
			s.cacheOpenAIImageReplayItem(scope, item)
		}
	case "":
		for _, item := range gjson.GetBytes(payload, "output").Array() {
			s.cacheOpenAIImageReplayItem(scope, item)
		}
	}
}
func (s *OpenAIGatewayService) rememberOpenAIImageReplayBody(c *gin.Context, account *Account, body []byte) {
	if bodyHasSSEFraming(body) {
		forEachOpenAISSEFrame(string(body), func(eventType string, data []byte) { s.rememberOpenAIImageReplay(c, account, data, eventType) })
	} else {
		s.rememberOpenAIImageReplay(c, account, body, "")
	}
}
func (s *OpenAIGatewayService) prepareOpenAIImageReplayHTTP(c *gin.Context, account *Account, body []byte) ([]byte, error) {
	normalized, _, err := s.prepareOpenAIImageReplay(c, account, body)
	if err != nil {
		if c != nil {
			MarkOpsClientBusinessLimited(c, "local_image_replay_unavailable")
			setOpsUpstreamError(c, http.StatusBadRequest, err.Error(), "")
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "code": "image_replay_unavailable", "param": "input", "message": err.Error()}})
		}
		return body, err
	}
	return normalized, nil
}
