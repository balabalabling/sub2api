package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIImageReplayHTTPForwardAcrossResponseFormats(t *testing.T) {
	for _, oauth := range []bool{false, true} {
		for _, passthrough := range []bool{false, true} {
			for _, stream := range []bool{false, true} {
				name := "API-key"
				if oauth {
					name = "OAuth"
				}
				if passthrough {
					name += "/passthrough"
				} else {
					name += "/normal"
				}
				if stream {
					name += "/SSE"
				} else {
					name += "/JSON"
				}
				t.Run(name, func(t *testing.T) {
					cfg := &config.Config{}
					cfg.Security.URLAllowlist.Enabled = false
					s := imageReplayTestService(t)
					s.cfg = cfg
					s.toolCorrector = NewCodexToolCorrector()
					account := &Account{ID: 99, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://example.com"}, Extra: map[string]any{"use_responses_api": true, "openai_passthrough": passthrough}}
					if oauth {
						account.Type = AccountTypeOAuth
						account.Credentials = map[string]any{"access_token": "oauth-test", "chatgpt_account_id": "chatgpt-test"}
					}
					image := `{"type":"image_generation_call","id":"ig_http","status":"completed","result":"cG5n"}`
					response1 := `{"id":"resp_first","model":"gpt-6.1-sol","status":"completed","output":[` + image + `],"usage":{"input_tokens":1,"output_tokens":1}}`
					response2 := `{"id":"resp_second","model":"gpt-6.1-sol","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1,"output_tokens":1}}`
					makeResponse := func(body string, first bool) *http.Response {
						contentType := "application/json"
						if stream || oauth {
							contentType = "text/event-stream"
							prefix := ""
							if first {
								prefix = `data: {"type":"response.output_item.done","item":` + image + "}\n\n"
							}
							body = prefix + `data: {"type":"response.completed","response":` + body + "}\n\n"
						}
						return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(body))}
					}
					upstream := &httpUpstreamRecorder{responses: []*http.Response{makeResponse(response1, true), makeResponse(response2, false)}}
					s.httpUpstream = upstream
					send := func(input string) {
						body := []byte(`{"model":"gpt-6.1-sol","store":false,"stream":` + map[bool]string{true: "true", false: "false"}[stream] + `,"input":` + input + `}`)
						c := imageReplayTestContext(1, 2)
						c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
						SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
						result, err := s.Forward(context.Background(), c, account, body)
						require.NoError(t, err)
						require.NotNil(t, result)
					}
					send(`[{"role":"user","content":"draw"}]`)
					send(`[{"type":"image_generation_call","id":"ig_http","status":"completed"},{"role":"user","content":"continue"}]`)
					require.Len(t, upstream.bodies, 2)
					require.Equal(t, "cG5n", gjson.GetBytes(upstream.lastBody, "input.0.result").String())
					require.False(t, gjson.GetBytes(upstream.lastBody, "store").Bool())
				})
			}
		}
	}
}

func TestOpenAIImageReplayHTTPMissingReturns400WithoutUpstreamRetry(t *testing.T) {
	s := imageReplayTestService(t)
	s.cfg = &config.Config{}
	upstream := &httpUpstreamRecorder{}
	s.httpUpstream = upstream
	c := imageReplayTestContext(1, 2)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "oauth-test"}}
	_, err := s.Forward(context.Background(), c, account, []byte(`{"model":"gpt-6.1-sol","store":false,"input":[{"type":"image_generation_call","id":"ig_lost"}]}`))
	require.ErrorIs(t, err, errOpenAIImageReplayUnavailable)
	require.Equal(t, http.StatusBadRequest, c.Writer.Status())
	require.Empty(t, upstream.requests)
	rec := c.Writer // The response has already been written, not left for a 502 mapper.
	require.True(t, rec.Written())
}

// Gate the second upstream terminal event on the actual second client request;
// this avoids timing-dependent mock events bypassing request hydration.
type imageReplayWSTestConn struct {
	*openAIWSCaptureConn
	secondWrite chan struct{}
	once        sync.Once
}

func (c *imageReplayWSTestConn) WriteJSON(ctx context.Context, value any) error {
	if err := c.openAIWSCaptureConn.WriteJSON(ctx, value); err != nil {
		return err
	}
	c.mu.Lock()
	n := len(c.writes)
	c.mu.Unlock()
	if n >= 2 {
		c.once.Do(func() { close(c.secondWrite) })
	}
	return nil
}
func (c *imageReplayWSTestConn) ReadMessage(ctx context.Context) ([]byte, error) {
	c.mu.Lock()
	last := len(c.events) == 1
	c.mu.Unlock()
	if last {
		select {
		case <-c.secondWrite:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return c.openAIWSCaptureConn.ReadMessage(ctx)
}
func (c *imageReplayWSTestConn) WriteFrame(ctx context.Context, _ coderws.MessageType, payload []byte) error {
	return c.WriteJSON(ctx, json.RawMessage(payload))
}
func (c *imageReplayWSTestConn) ReadFrame(ctx context.Context) (coderws.MessageType, []byte, error) {
	payload, err := c.ReadMessage(ctx)
	return coderws.MessageText, payload, err
}

func TestOpenAIImageReplayWebSocketTwoTurns(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		name := "ctx_pool"
		if passthrough {
			name = "passthrough"
		}
		t.Run(name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Security.URLAllowlist.Enabled = false
			cfg.Security.URLAllowlist.AllowInsecureHTTP = true
			ws := &cfg.Gateway.OpenAIWS
			ws.Enabled = true
			ws.OAuthEnabled = true
			ws.APIKeyEnabled = true
			ws.ResponsesWebsocketsV2 = true
			ws.ModeRouterV2Enabled = true
			ws.IngressModeDefault = OpenAIWSIngressModeCtxPool
			ws.MaxConnsPerAccount = 1
			ws.MaxIdlePerAccount = 1
			ws.QueueLimitPerConn = 8
			ws.DialTimeoutSeconds = 3
			ws.ReadTimeoutSeconds = 3
			ws.WriteTimeoutSeconds = 3
			capture := &imageReplayWSTestConn{openAIWSCaptureConn: &openAIWSCaptureConn{events: [][]byte{
				[]byte(`{"type":"response.output_item.done","item":{"type":"image_generation_call","id":"ig_ws","status":"completed","result":"cG5n"}}`),
				[]byte(`{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.1","output":[],"usage":{"input_tokens":1,"output_tokens":1}}}`),
				[]byte(`{"type":"response.completed","response":{"id":"resp_2","model":"gpt-5.1","output":[],"usage":{"input_tokens":1,"output_tokens":1}}}`),
			}}, secondWrite: make(chan struct{})}
			dialer := &openAIWSSingleConnDialer{conn: capture}
			pool := newOpenAIWSConnPool(cfg)
			pool.setClientDialerForTest(dialer)
			s := imageReplayTestService(t)
			s.cfg = cfg
			s.httpUpstream = &httpUpstreamRecorder{}
			s.cache = &stubGatewayCache{}
			s.openaiWSResolver = NewOpenAIWSProtocolResolver(cfg)
			s.toolCorrector = NewCodexToolCorrector()
			s.openaiWSPool = pool
			s.openaiWSPassthroughDialer = dialer
			t.Cleanup(s.CloseOpenAIWSPool)
			account := &Account{ID: 111, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "sk-test"}, Extra: map[string]any{"responses_websockets_v2_enabled": true}}
			if passthrough {
				account.Extra["openai_apikey_responses_websockets_v2_mode"] = OpenAIWSIngressModePassthrough
			}
			errCh := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := coderws.Accept(w, r, nil)
				if err != nil {
					errCh <- err
					return
				}
				defer conn.CloseNow()
				_, first, err := conn.Read(r.Context())
				if err != nil {
					errCh <- err
					return
				}
				c := imageReplayTestContext(1, 2)
				c.Request = r.Clone(r.Context())
				errCh <- s.ProxyResponsesWebSocketFromClient(r.Context(), c, conn, account, "sk-test", first, nil)
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
			require.NoError(t, err)
			defer client.CloseNow()
			first := `{"type":"response.create","model":"gpt-5.1","store":false,"input":[{"role":"user","content":"draw"}]}`
			require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(first)))
			for {
				_, payload, err := client.Read(ctx)
				require.NoError(t, err)
				if gjson.GetBytes(payload, "type").String() == "response.completed" {
					break
				}
			}
			second := `{"type":"response.create","model":"gpt-5.1","store":false,"input":[{"type":"image_generation_call","id":"ig_ws","status":"completed"},{"role":"user","content":"continue"}]}`
			require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(second)))
			for {
				_, payload, err := client.Read(ctx)
				require.NoError(t, err)
				if gjson.GetBytes(payload, "type").String() == "response.completed" {
					break
				}
			}
			_ = client.Close(coderws.StatusNormalClosure, "done")
			select {
			case err := <-errCh:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal("websocket did not finish")
			}
			capture.mu.Lock()
			writes := append([]map[string]any(nil), capture.writes...)
			capture.mu.Unlock()
			require.Len(t, writes, 2)
			raw, err := json.Marshal(writes[1])
			require.NoError(t, err)
			require.Equal(t, "cG5n", gjson.GetBytes(raw, "input.0.result").String())
		})
	}
}
