package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/tidwall/gjson"

	"predictdestiny/internal/fortune"
	"predictdestiny/internal/model"
)

// fakeSettings is a SettingsReader backed by an in-memory map, so the
// gateway can be unit-tested without PostgreSQL. It implements the
// exact same Get / GetDefault the production *store.SettingStore
// exposes — nothing else — so tests exercise the real gateway code,
// not a parallel reimplementation.
type fakeSettings map[string]string

func (f fakeSettings) Get(key string) (string, bool) { v, ok := f[key]; return v, ok }
func (f fakeSettings) GetDefault(key, fb string) string {
	if v, ok := f.Get(key); ok && v != "" {
		return v
	}
	return fb
}

// newGateway builds an OpenAIGateway whose Settings come from a fake
// map and whose HTTP traffic is pointed at the given test server (via
// the ai.base_url setting, the same path production uses).
func newGateway(t *testing.T, srv *httptest.Server, extra map[string]string) *OpenAIGateway {
	t.Helper()
	settings := fakeSettings{
		model.SettingAIBaseURL: srv.URL + "/v1",
		model.SettingAIAPIKey:  "k-test",
	}
	for k, v := range extra {
		settings[k] = v
	}
	return &OpenAIGateway{HTTP: srv.Client(), Settings: settings, allowPrivateNetwork: true}
}

// ─── Chat (synchronous) ───────────────────────────────────────────

// TestChatSync verifies the non-streaming path returns content, the
// optional reasoning_content field, and token usage — end to end
// through the real OpenAIGateway.Chat against an httptest server.
func TestChatSync(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer k-test" {
			t.Errorf("auth header = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id":"c1","object":"chat.completion","model":"test-model","created":1,
			"choices":[{"index":0,"message":{"role":"assistant","content":"你好","reasoning_content":"思考过程"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8,"completion_tokens_details":{"reasoning_tokens":2}}
		}`)
	}))
	defer srv.Close()

	g := newGateway(t, srv, map[string]string{model.SettingAIDefaultModel: "test-model"})
	resp, err := g.Chat(context.Background(), "", []Message{{Role: RoleUser, Content: "hi"}}, Options{})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if resp.Content != "你好" {
		t.Errorf("content = %q", resp.Content)
	}
	if resp.Reasoning != "思考过程" {
		t.Errorf("reasoning = %q", resp.Reasoning)
	}
	if resp.Usage.PromptTokens != 5 || resp.Usage.CompletionTokens != 3 || resp.Usage.TotalTokens != 8 {
		t.Errorf("usage = %+v", resp.Usage)
	}
	if resp.Usage.ReasoningTokens != 2 {
		t.Errorf("reasoning tokens = %d", resp.Usage.ReasoningTokens)
	}
}

// TestChatErrorMapping verifies upstream HTTP statuses map to the
// sentinel errors so the handler can render the right HTTP code.
func TestChatErrorMapping(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrKeyInvalid},
		{http.StatusTooManyRequests, ErrRateLimited},
		{http.StatusPaymentRequired, ErrInsufficient},
		{http.StatusNotFound, ErrModelNotFound},
		{http.StatusInternalServerError, ErrUpstream},
	}
	for _, c := range cases {
		t.Run(http.StatusText(c.status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.status)
				fmt.Fprintf(w, `{"error":{"message":"boom","type":"x","code":"%d"}}`, c.status)
			}))
			defer srv.Close()
			g := newGateway(t, srv, map[string]string{model.SettingAIDefaultModel: "test-model"})
			_, err := g.Chat(context.Background(), "test-model", []Message{{Role: RoleUser, Content: "hi"}}, Options{})
			if !errors.Is(err, c.want) {
				t.Errorf("status %d: got %v, want %v", c.status, err, c.want)
			}
		})
	}
}

// TestChatNotConfigured verifies the friendly guard fires before any
// HTTP traffic when base_url / key are missing.
func TestChatNotConfigured(t *testing.T) {
	g := NewOpenAIGateway(fakeSettings{})
	_, err := g.Chat(context.Background(), "m", []Message{{Role: RoleUser, Content: "hi"}}, Options{})
	if !errors.Is(err, ErrNotConfigured) {
		t.Errorf("got %v, want ErrNotConfigured", err)
	}
}

// TestChatNoModelResolved: configured but no model passed and no
// default / catalog → ErrNotConfigured (the resolveModel guard).
func TestChatNoModelResolved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not have called upstream")
	}))
	defer srv.Close()
	// configured (base+key) but no model catalog and no default
	g := &OpenAIGateway{HTTP: srv.Client(), allowPrivateNetwork: true, Settings: fakeSettings{
		model.SettingAIBaseURL: srv.URL + "/v1",
		model.SettingAIAPIKey:  "k",
	}}
	_, err := g.Chat(context.Background(), "", []Message{{Role: RoleUser, Content: "hi"}}, Options{})
	if !errors.Is(err, ErrNotConfigured) {
		t.Errorf("got %v, want ErrNotConfigured (no model resolved)", err)
	}
}

// ─── StreamChat (SSE) ─────────────────────────────────────────────

// TestStream verifies the streaming path concatenates content deltas,
// surfaces the optional reasoning_content delta, and ends with a
// terminal usage chunk + Done event.
func TestStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		writeChunk := func(s string) {
			fmt.Fprintf(w, "data: %s\n\n", s)
			if flusher != nil {
				flusher.Flush()
			}
		}
		writeChunk(`{"id":"s","model":"test-model","created":1,"object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`)
		writeChunk(`{"id":"s","model":"test-model","created":1,"object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"你"},"finish_reason":null}]}`)
		writeChunk(`{"id":"s","model":"test-model","created":1,"object":"chat.completion.chunk","choices":[{"index":0,"delta":{"reasoning_content":"思"},"finish_reason":null}]}`)
		writeChunk(`{"id":"s","model":"test-model","created":1,"object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"好"},"finish_reason":null}]}`)
		writeChunk(`{"id":"s","model":"test-model","created":1,"object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6,"completion_tokens_details":{"reasoning_tokens":1}}}`)
		fmt.Fprint(w, "data: [DONE]\n\n")
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer srv.Close()

	g := newGateway(t, srv, map[string]string{model.SettingAIDefaultModel: "test-model"})

	var content, reasoning strings.Builder
	var gotUsage *Usage
	done := false
	err := g.StreamChat(context.Background(), "test-model", []Message{{Role: RoleUser, Content: "hi"}}, Options{}, func(ev StreamEvent) {
		content.WriteString(ev.Content)
		reasoning.WriteString(ev.Reasoning)
		if ev.Usage != nil {
			gotUsage = ev.Usage
		}
		if ev.Done {
			done = true
		}
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if content.String() != "你好" {
		t.Errorf("content = %q", content.String())
	}
	if reasoning.String() != "思" {
		t.Errorf("reasoning = %q", reasoning.String())
	}
	if gotUsage == nil || gotUsage.TotalTokens != 6 || gotUsage.ReasoningTokens != 1 {
		t.Errorf("usage = %+v", gotUsage)
	}
	if !done {
		t.Error("no Done event")
	}
}

// TestStreamError verifies a non-2xx status during the stream
// handshake is mapped to a sentinel error.
func TestStreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"message":"slow down","type":"rate_limit","code":"429"}}`)
	}))
	defer srv.Close()
	g := newGateway(t, srv, map[string]string{model.SettingAIDefaultModel: "test-model"})
	err := g.StreamChat(context.Background(), "test-model", []Message{{Role: RoleUser, Content: "hi"}}, Options{}, func(ev StreamEvent) {})
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("got %v, want ErrRateLimited", err)
	}
}

// TestStreamContextCancel verifies ctx cancellation propagates and is
// surfaced as context.Canceled (not wrapped in ErrUpstream).
func TestStreamContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		// send one chunk then block until the connection tears down
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"s","model":"test-model","created":1,"object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"你"},"finish_reason":null}]}`)
		if flusher != nil {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer srv.Close()
	g := newGateway(t, srv, map[string]string{model.SettingAIDefaultModel: "test-model"})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := g.StreamChat(ctx, "test-model", []Message{{Role: RoleUser, Content: "hi"}}, Options{}, func(ev StreamEvent) {})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("got %v, want context.DeadlineExceeded", err)
	}
}

// ─── ListModels (settings parsing) ────────────────────────────────

// TestListModels verifies the catalog parser groups by tier and
// tolerates malformed JSON by returning an empty catalog.
func TestListModels(t *testing.T) {
	g := NewOpenAIGateway(fakeSettings{
		model.SettingAIModels: `[{"id":"qwen3.7-plus","tier":"free","label":"Qwen"},{"id":"claude-opus","tier":"paid","label":"Opus"},{"id":"","tier":"free"}]`,
	})
	cat := g.ListModels()
	if len(cat.Free) != 1 || cat.Free[0].ID != "qwen3.7-plus" {
		t.Errorf("free = %+v", cat.Free)
	}
	if len(cat.Paid) != 1 || cat.Paid[0].ID != "claude-opus" {
		t.Errorf("paid = %+v", cat.Paid)
	}
	if _, ok := cat.Find("qwen3.7-plus"); !ok {
		t.Error("Find failed for known model")
	}
	if _, ok := cat.Find("nope"); ok {
		t.Error("Find should fail for unknown model")
	}

	// malformed JSON → empty catalog
	g2 := NewOpenAIGateway(fakeSettings{model.SettingAIModels: `{not json`})
	if cat2 := g2.ListModels(); len(cat2.All()) != 0 {
		t.Errorf("malformed JSON should yield empty catalog, got %+v", cat2)
	}

	// unknown tier defaults to free so a typo doesn't hide a model
	g3 := NewOpenAIGateway(fakeSettings{model.SettingAIModels: `[{"id":"x","tier":"weird"}]`})
	if cat3 := g3.ListModels(); len(cat3.Free) != 1 || cat3.Free[0].ID != "x" {
		t.Errorf("unknown tier should fall to free, got %+v", cat3)
	}

	// unset → empty catalog
	g4 := NewOpenAIGateway(fakeSettings{})
	if cat4 := g4.ListModels(); len(cat4.All()) != 0 {
		t.Errorf("unset models should yield empty catalog, got %+v", cat4)
	}
}

// ─── resolveModel precedence ──────────────────────────────────────

// TestResolveModel checks the model resolution precedence:
// explicit > default > first free > first paid > empty.
func TestResolveModel(t *testing.T) {
	withDefault := NewOpenAIGateway(fakeSettings{
		model.SettingAIModels:       `[{"id":"free-a","tier":"free"},{"id":"paid-a","tier":"paid"}]`,
		model.SettingAIDefaultModel: "default-model",
	})
	if got := withDefault.resolveModel("explicit"); got != "explicit" {
		t.Errorf("explicit: got %q", got)
	}
	if got := withDefault.resolveModel(""); got != "default-model" {
		t.Errorf("default: got %q", got)
	}

	noDefault := NewOpenAIGateway(fakeSettings{
		model.SettingAIModels: `[{"id":"free-a","tier":"free"},{"id":"paid-a","tier":"paid"}]`,
	})
	if got := noDefault.resolveModel(""); got != "free-a" {
		t.Errorf("first-free fallback: got %q", got)
	}

	paidOnly := NewOpenAIGateway(fakeSettings{
		model.SettingAIModels: `[{"id":"paid-a","tier":"paid"}]`,
	})
	if got := paidOnly.resolveModel(""); got != "paid-a" {
		t.Errorf("first-paid fallback: got %q", got)
	}

	empty := NewOpenAIGateway(fakeSettings{})
	if got := empty.resolveModel(""); got != "" {
		t.Errorf("no models: got %q", got)
	}
}

// ─── configured guard ─────────────────────────────────────────────

// TestConfigured verifies the not-configured guard fires on missing
// base URL or key.
func TestConfigured(t *testing.T) {
	if err := (NewOpenAIGateway(fakeSettings{})).configured(); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("empty: got %v, want ErrNotConfigured", err)
	}
	if err := (NewOpenAIGateway(fakeSettings{model.SettingAIBaseURL: "https://x/v1"})).configured(); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("missing key: got %v", err)
	}
	if err := (NewOpenAIGateway(fakeSettings{
		model.SettingAIBaseURL: "https://x/v1",
		model.SettingAIAPIKey:  "k",
	})).configured(); err != nil {
		t.Errorf("configured: got %v", err)
	}
}

// ─── seams ────────────────────────────────────────────────────────

// TestMessagesFromPrompt verifies the fortune→ai seam wires system +
// user through correctly.
func TestMessagesFromPrompt(t *testing.T) {
	p := &fortune.PromptSpec{System: "S", User: "U", Tier: fortune.TierFree}
	msgs := MessagesFromPrompt(p)
	if len(msgs) != 2 {
		t.Fatalf("got %d msgs", len(msgs))
	}
	if msgs[0].Role != RoleSystem || msgs[0].Content != "S" {
		t.Errorf("system msg = %+v", msgs[0])
	}
	if msgs[1].Role != RoleUser || msgs[1].Content != "U" {
		t.Errorf("user msg = %+v", msgs[1])
	}
}

// TestFromFortuneTier verifies the tier mapping shim.
func TestFromFortuneTier(t *testing.T) {
	if FromFortuneTier("paid") != TierPaid {
		t.Error("paid should map to paid")
	}
	if FromFortuneTier("free") != TierFree {
		t.Error("free should map to free")
	}
	if FromFortuneTier("nonsense") != TierFree {
		t.Error("unknown should default to free")
	}
}

// ─── SDK protocol smoke (paranoia: confirm SDK parses the shapes we
// rely on, independent of the gateway wrapper) ───────────────────

// TestSDKProtocolParse drives the raw SDK against a server that emits
// the Qwen-style reasoning_content + usage, confirming the SDK's
// RawJSON() lets us pull non-standard fields via gjson. This guards
// against an SDK upgrade silently dropping RawJSON support.
func TestSDKProtocolParse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"c","object":"chat.completion","model":"m","created":1,
		"choices":[{"index":0,"message":{"role":"assistant","content":"A","reasoning_content":"R"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2,"completion_tokens_details":{"reasoning_tokens":1}}}`)
	}))
	defer srv.Close()

	cli := openai.NewClient(option.WithBaseURL(srv.URL+"/"), option.WithAPIKey("k"))
	completion, err := cli.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model: openai.ChatModel("m"),
		Messages: []openai.ChatCompletionMessageParamUnion{{OfUser: &openai.ChatCompletionUserMessageParam{
			Content: openai.ChatCompletionUserMessageParamContentUnion{OfString: openai.String("hi")},
		}}},
	})
	if err != nil {
		t.Fatalf("sdk: %v", err)
	}
	if gjson.Get(completion.RawJSON(), "choices.0.message.reasoning_content").String() != "R" {
		t.Error("reasoning_content not extractable via gjson")
	}
	if completion.Usage.CompletionTokensDetails.ReasoningTokens != 1 {
		t.Errorf("reasoning tokens = %d", completion.Usage.CompletionTokensDetails.ReasoningTokens)
	}
}
