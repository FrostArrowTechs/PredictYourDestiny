package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"predictdestiny/internal/ai"
	"predictdestiny/internal/auth"
	"predictdestiny/internal/model"
)

// fakeGateway is an ai.Gateway stub for handler tests. It records the
// args it was called with and replays canned events for streaming.
type fakeGateway struct {
	chatResp     *ai.Response
	chatErr      error
	streamEvents []ai.StreamEvent
	streamErr    error
	catalog      ai.ModelCatalog

	lastModel string
	lastMsgs  []ai.Message
}

func (g *fakeGateway) Chat(_ context.Context, model string, msgs []ai.Message, _ ai.Options) (*ai.Response, error) {
	g.lastModel = model
	g.lastMsgs = msgs
	if g.chatErr != nil {
		return nil, g.chatErr
	}
	return g.chatResp, nil
}

func (g *fakeGateway) StreamChat(_ context.Context, model string, msgs []ai.Message, _ ai.Options, onEvent func(ai.StreamEvent)) error {
	g.lastModel = model
	g.lastMsgs = msgs
	for _, ev := range g.streamEvents {
		onEvent(ev)
	}
	return g.streamErr
}

func (g *fakeGateway) ListModels() ai.ModelCatalog { return g.catalog }

// ─── helpers ──────────────────────────────────────────────────────

func newTestRouter(h *BaziHandler) *gin.Engine {
	if h.DB == nil {
		db, err := gorm.Open(sqlite.Open("file:bazi_handler_tests?mode=memory&cache=shared"), &gorm.Config{})
		if err != nil {
			panic(err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			panic(err)
		}
		sqlDB.SetMaxOpenConns(1)
		if err := db.AutoMigrate(&model.MembershipTier{}, &model.UserMembership{}, &model.UsageQuota{}); err != nil {
			panic(err)
		}
		if err := db.Where("code = ?", model.TierCodeFree).FirstOrCreate(
			&model.MembershipTier{Code: model.TierCodeFree, Name: "Free", DailyQuota: 100},
		).Error; err != nil {
			panic(err)
		}
		h.DB = db
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(auth.ClaimsKey), &auth.Claims{UserID: 1, Role: "user"})
		c.Next()
	})
	r.POST("/api/bazi/compute", h.Compute)
	r.POST("/api/bazi/interpret", h.Interpret)
	return r
}

func doJSON(t *testing.T, r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ─── /compute ─────────────────────────────────────────────────────

// TestBaziCompute verifies the pure-math endpoint returns the chart
// for a known input. No AI is invoked.
func TestBaziCompute(t *testing.T) {
	h := &BaziHandler{}
	r := newTestRouter(h)
	w := doJSON(t, r, "/api/bazi/compute",
		`{"year":2000,"month":1,"day":1,"hour":12,"minute":0,"gender":1,"lang":"zh-CN"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var res struct {
		Kind string `json:"kind"`
		Data struct {
			Pillars []struct{ GanZhi string } `json:"pillars"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Kind != "bazi" {
		t.Errorf("kind = %q", res.Kind)
	}
	if len(res.Data.Pillars) != 4 {
		t.Fatalf("pillars = %d", len(res.Data.Pillars))
	}
	want := []string{"己卯", "丙子", "戊午", "戊午"}
	for i, p := range res.Data.Pillars {
		if p.GanZhi != want[i] {
			t.Errorf("pillar[%d] = %q, want %q", i, p.GanZhi, want[i])
		}
	}
}

// TestBaziComputeBadInput verifies validation errors map to 400.
func TestBaziComputeBadInput(t *testing.T) {
	h := &BaziHandler{}
	r := newTestRouter(h)
	cases := []string{
		`{"year":1899,"month":1,"day":1,"hour":12,"gender":1}`,
		`{"year":2000,"month":13,"day":1,"gender":1}`,
		`{"year":2000,"month":1,"day":1,"hour":25,"gender":1}`,
		`{"year":2000,"month":1,"day":1,"gender":2}`,
		`{"year":2000,"month":1,"day":1,"hour":12,"gender":1,"longitude":200}`,
		`{}`,
	}
	for _, body := range cases {
		w := doJSON(t, r, "/api/bazi/compute", body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want 400", body, w.Code)
		}
	}
}

func TestBaziDistinguishesUnknownTimeFromMidnight(t *testing.T) {
	r := newTestRouter(&BaziHandler{})
	unknown := doJSON(t, r, "/api/bazi/compute",
		`{"year":2000,"month":1,"day":1,"timePrecision":"unknown","gender":1}`)
	if unknown.Code != http.StatusOK || !strings.Contains(unknown.Body.String(), `"variants":[{`) {
		t.Fatalf("unknown status = %d, body = %s", unknown.Code, unknown.Body.String())
	}

	midnight := doJSON(t, r, "/api/bazi/compute",
		`{"year":2000,"month":1,"day":1,"hour":0,"minute":0,"timePrecision":"minute","gender":1}`)
	if midnight.Code != http.StatusOK {
		t.Fatalf("midnight status = %d, body = %s", midnight.Code, midnight.Body.String())
	}
}

// ─── /interpret (sync) ────────────────────────────────────────────

// TestBaziInterpretSync verifies the handler wires compute → prompt →
// gateway.Chat and returns the AI content + usage as JSON.
func TestBaziInterpretSync(t *testing.T) {
	gw := &fakeGateway{
		catalog: ai.ModelCatalog{Free: []ai.ModelEntry{{ID: "qwen3.7-plus", Tier: ai.TierFree}}},
		chatResp: &ai.Response{
			Content:   "解读内容",
			Reasoning: "思考",
			Model:     "qwen3.7-plus",
			Usage:     ai.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		},
	}
	h := &BaziHandler{Gateway: gw}
	r := newTestRouter(h)
	w := doJSON(t, r, "/api/bazi/interpret",
		`{"year":2000,"month":1,"day":1,"hour":12,"minute":0,"gender":1,"lang":"zh-CN"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var out struct {
		Content   string   `json:"content"`
		Reasoning string   `json:"reasoning"`
		Model     string   `json:"model"`
		Usage     ai.Usage `json:"usage"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Content != "解读内容" {
		t.Errorf("content = %q", out.Content)
	}
	if out.Model != "qwen3.7-plus" {
		t.Errorf("model = %q", out.Model)
	}
	if out.Usage.TotalTokens != 15 {
		t.Errorf("usage total = %d", out.Usage.TotalTokens)
	}
	// the gateway should have received a system + user message
	if len(gw.lastMsgs) != 2 {
		t.Fatalf("gateway got %d msgs, want 2", len(gw.lastMsgs))
	}
	if gw.lastMsgs[0].Role != ai.RoleSystem {
		t.Errorf("first msg role = %q", gw.lastMsgs[0].Role)
	}
	if !strings.Contains(gw.lastMsgs[1].Content, "戊午") {
		t.Error("user prompt should inject the day pillar 戊午")
	}
}

// TestBaziInterpretModelResolution verifies the handler validates the
// client's model pick against both the catalog and effective membership.
func TestBaziInterpretModelResolution(t *testing.T) {
	gw := &fakeGateway{
		catalog: ai.ModelCatalog{
			Free: []ai.ModelEntry{{ID: "free-a", Tier: ai.TierFree}},
			Paid: []ai.ModelEntry{{ID: "paid-a", Tier: ai.TierPaid}},
		},
		chatResp: &ai.Response{Content: "x", Model: "free-a"},
	}
	h := &BaziHandler{Gateway: gw}
	r := newTestRouter(h)

	// no model → brief tier → first free model
	w := doJSON(t, r, "/api/bazi/interpret",
		`{"year":2000,"month":1,"day":1,"hour":12,"minute":0,"gender":1,"lang":"zh-CN"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, %s", w.Code, w.Body.String())
	}
	if gw.lastModel != "free-a" {
		t.Errorf("default model = %q, want free-a", gw.lastModel)
	}

	// explicit paid model is rejected for a free member
	w = doJSON(t, r, "/api/bazi/interpret",
		`{"year":2000,"month":1,"day":1,"hour":12,"minute":0,"gender":1,"lang":"zh-CN","model":"paid-a"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", w.Code, w.Body.String())
	}

	// explicit unknown model is rejected rather than silently substituted
	w = doJSON(t, r, "/api/bazi/interpret",
		`{"year":2000,"month":1,"day":1,"hour":12,"minute":0,"gender":1,"lang":"zh-CN","model":"does-not-exist"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
	}

	// deep prompt request cannot elevate a free user; it uses a free model
	w = doJSON(t, r, "/api/bazi/interpret",
		`{"year":2000,"month":1,"day":1,"hour":12,"minute":0,"gender":1,"lang":"zh-CN","interpretDepth":"deep"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, %s", w.Code, w.Body.String())
	}
	if gw.lastModel != "free-a" {
		t.Errorf("deep tier model = %q, want free-a", gw.lastModel)
	}
}

// TestBaziInterpretNoGateway verifies the 503 when the gateway isn't
// wired (e.g. before the admin configures AI settings).
func TestBaziInterpretNoGateway(t *testing.T) {
	h := &BaziHandler{} // Gateway nil
	r := newTestRouter(h)
	w := doJSON(t, r, "/api/bazi/interpret",
		`{"year":2000,"month":1,"day":1,"hour":12,"minute":0,"gender":1,"lang":"zh-CN"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

// TestBaziInterpretNoModels verifies the 503 when the gateway is
// present but no models are configured.
func TestBaziInterpretNoModels(t *testing.T) {
	gw := &fakeGateway{catalog: ai.ModelCatalog{}} // empty catalog
	h := &BaziHandler{Gateway: gw}
	r := newTestRouter(h)
	w := doJSON(t, r, "/api/bazi/interpret",
		`{"year":2000,"month":1,"day":1,"hour":12,"minute":0,"gender":1,"lang":"zh-CN"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

// ─── /interpret (stream) ──────────────────────────────────────────

// TestBaziInterpretStream verifies the SSE envelope: each delta is a
// "data: {json}\n\n" line, the done event is emitted, and usage is
// carried on the terminal usage chunk.
func TestBaziInterpretStream(t *testing.T) {
	gw := &fakeGateway{
		catalog: ai.ModelCatalog{Free: []ai.ModelEntry{{ID: "qwen3.7-plus", Tier: ai.TierFree}}},
		streamEvents: []ai.StreamEvent{
			{Content: "你好"},
			{Reasoning: "思"},
			{Usage: &ai.Usage{PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6, ReasoningTokens: 1}},
			{Done: true},
		},
	}
	h := &BaziHandler{Gateway: gw}
	r := newTestRouter(h)
	w := doJSON(t, r, "/api/bazi/interpret",
		`{"year":2000,"month":1,"day":1,"hour":12,"minute":0,"gender":1,"lang":"zh-CN","stream":true}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("content-type = %q", ct)
	}

	// parse SSE
	var content, reasoning strings.Builder
	var gotUsage *ai.Usage
	done := false
	scanner := bufio.NewScanner(strings.NewReader(w.Body.String()))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			done = true
			continue
		}
		var ev struct {
			Content   string    `json:"content"`
			Reasoning string    `json:"reasoning"`
			Done      bool      `json:"done"`
			Usage     *ai.Usage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			t.Fatalf("parse sse: %v | %s", err, payload)
		}
		content.WriteString(ev.Content)
		reasoning.WriteString(ev.Reasoning)
		if ev.Usage != nil {
			gotUsage = ev.Usage
		}
		if ev.Done {
			done = true
		}
	}
	if content.String() != "你好" {
		t.Errorf("content = %q", content.String())
	}
	if reasoning.String() != "思" {
		t.Errorf("reasoning = %q", reasoning.String())
	}
	if gotUsage == nil || gotUsage.TotalTokens != 6 {
		t.Errorf("usage = %+v", gotUsage)
	}
	if !done {
		t.Error("no done event in SSE stream")
	}
}

// TestMapAIError verifies sentinel→HTTP-status mapping so the client
// gets the right code, not a blanket 500.
func TestMapAIError(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{ai.ErrNotConfigured, http.StatusServiceUnavailable},
		{ai.ErrKeyInvalid, http.StatusUnauthorized},
		{ai.ErrRateLimited, http.StatusTooManyRequests},
		{ai.ErrInsufficient, http.StatusPaymentRequired},
		{ai.ErrModelNotFound, http.StatusNotFound},
		{ai.ErrTimeout, http.StatusGatewayTimeout},
		{ai.ErrUpstream, http.StatusBadGateway},
	}
	for _, c := range cases {
		if got := mapAIError(c.err); got != c.want {
			t.Errorf("%v: got %d, want %d", c.err, got, c.want)
		}
	}
}
