package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"predictdestiny/internal/ai"
	"predictdestiny/internal/ai/prompt"
	"predictdestiny/internal/fortune"
)

// BaziHandler exposes the bazi compute + AI interpret endpoints.
//
// /api/bazi/compute  — pure chart math, anonymous-ok, zero tokens.
// /api/bazi/interpret — AI reading via the configured gateway.
//
// The handler stays thin: parse → engine.Compute → (prompt + gateway
// for interpret) → render. Auth/quota gating arrives in stage 4; for
// now interpret is open so the pipeline can be validated end-to-end.
type BaziHandler struct {
	Gateway ai.Gateway
	DB      *gorm.DB
}

// ─── request / response envelopes ─────────────────────────────────

// baziComputeReq is the input for both /compute and /interpret:
// /interpret re-runs Compute from the same birth data rather than
// trusting a client-supplied chart, so the AI always reasons over a
// fresh, validated result.
type baziComputeReq struct {
	Year   int `json:"year"    binding:"required,min=1"`
	Month  int `json:"month"   binding:"required,min=1"`
	Day    int `json:"day"     binding:"required,min=1"`
	Hour   int `json:"hour"`
	Minute int `json:"minute"`
	// gender 0 = female is a valid value, so we can't use `required`
	// (which rejects the zero value). min/max accept both 0 and 1.
	Gender         int     `json:"gender"  binding:"min=0,max=1"`
	Longitude      float64 `json:"longitude"`
	Lang           string  `json:"lang"`
	InterpretDepth string  `json:"interpretDepth"`
	Model          string  `json:"model"`
	Stream         bool    `json:"stream"`
}

// toFortuneInput maps the wire request into the engine's Input. Lang
// defaults to zh-CN so prompts always have a language to render in.
func (r baziComputeReq) toFortuneInput() fortune.Input {
	lang := r.Lang
	if lang == "" {
		lang = "zh-CN"
	}
	return fortune.Input{
		Year: r.Year, Month: r.Month, Day: r.Day,
		Hour: r.Hour, Minute: r.Minute,
		Gender:         fortune.Gender(r.Gender),
		Longitude:      r.Longitude,
		Lang:           lang,
		InterpretDepth: r.InterpretDepth,
	}
}

// ─── POST /api/bazi/compute ───────────────────────────────────────

// Compute runs the bazi engine and returns the structured chart. No
// AI is involved — this is the free, anonymous-safe endpoint.
func (h *BaziHandler) Compute(c *gin.Context) {
	var req baziComputeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.computeChart(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// computeChart is shared by Compute and Interpret: it parses the
// request, runs the engine, and returns the structured Result. A
// non-nil error means the input was bad (4xx-appropriate).
func (h *BaziHandler) computeChart(req baziComputeReq) (*fortune.Result, error) {
	eng, ok := fortune.Fortune(fortune.KindBazi)
	if !ok {
		return nil, fmt.Errorf("bazi engine unavailable")
	}
	return eng.Compute(req.toFortuneInput())
}

// ─── POST /api/bazi/interpret ─────────────────────────────────────

// Interpret computes the chart then asks the AI gateway for a
// reading. With stream=true it emits SSE delta events as the model
// generates; otherwise it returns one JSON object with the full text
// + usage.
//
// The SSE event format is "data: {json}\n\n" per delta and a final
// "data: [DONE]\n\n", matching what most OpenAI-compatible frontends
// expect. Each delta JSON carries content, optional reasoning, and
// (on the terminal event) usage.
func (h *BaziHandler) Interpret(c *gin.Context) {
	var req baziComputeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.computeChart(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.Gateway == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI gateway not configured"})
		return
	}

	spec, err := prompt.BaziBuild(req.toFortuneInput(), res)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// model resolution: client choice > prompt-recommended tier's
	// first model > gateway default. We do NOT blindly trust the
	// client's model id — it must exist in the catalog.
	model, authorized := authorizeAIRequest(c, h.DB, h.Gateway, req.Model, spec.Tier)
	if !authorized {
		return
	}
	if model == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no AI model configured (set ai.models)"})
		return
	}

	msgs := ai.MessagesFromPrompt(spec)
	opts := ai.Options{Temperature: 0.7}

	if req.Stream {
		h.interpretStream(c, model, msgs, opts)
		return
	}
	h.interpretSync(c, model, msgs, opts)
}

// resolveModel validates the client's model pick against the catalog
// and falls back to the prompt-tier's first model when unset. Returns
// "" if nothing is configured.
func (h *BaziHandler) resolveModel(requested string, spec *fortune.PromptSpec) string {
	cat := h.Gateway.ListModels()
	if requested != "" {
		if _, ok := cat.Find(requested); ok {
			return requested
		}
		// unknown id → fall through to tier default rather than
		// sending the gateway a model it may reject
	}
	// pick the first model of the prompt's tier
	tier := ai.FromFortuneTier(spec.Tier)
	if tier == ai.TierPaid && len(cat.Paid) > 0 {
		return cat.Paid[0].ID
	}
	if len(cat.Free) > 0 {
		return cat.Free[0].ID
	}
	if len(cat.Paid) > 0 {
		return cat.Paid[0].ID
	}
	return ai.ResolveModel(h.Gateway, "")
}

// interpretSync does one blocking completion and returns the full
// text + usage as JSON.
func (h *BaziHandler) interpretSync(c *gin.Context, model string, msgs []ai.Message, opts ai.Options) {
	resp, err := h.Gateway.Chat(c.Request.Context(), model, msgs, opts)
	if err != nil {
		c.JSON(mapAIError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"content":   resp.Content,
		"reasoning": resp.Reasoning,
		"model":     resp.Model,
		"usage":     resp.Usage,
	})
}

// interpretStream forwards gateway deltas to the client as SSE. The
// handler writes headers once and then blocks on StreamChat; ctx
// cancellation (client disconnect) propagates to the gateway.
func (h *BaziHandler) interpretStream(c *gin.Context, model string, msgs []ai.Message, opts ai.Options) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no") // nginx: don't buffer
	c.Writer.WriteHeader(http.StatusOK)
	flusher, _ := c.Writer.(http.Flusher)

	writeEvent := func(payload string) {
		fmt.Fprintf(c.Writer, "data: %s\n\n", payload)
		if flusher != nil {
			flusher.Flush()
		}
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	err := h.Gateway.StreamChat(ctx, model, msgs, opts, func(ev ai.StreamEvent) {
		if ev.Done {
			writeEvent(`{"done":true}`)
			return
		}
		// build a compact JSON delta; reasoning + usage optional.
		var json string
		if ev.Usage != nil {
			json = fmt.Sprintf(`{"content":%q,"reasoning":%q,"usage":{"prompt_tokens":%d,"completion_tokens":%d,"total_tokens":%d,"reasoning_tokens":%d}}`,
				ev.Content, ev.Reasoning,
				ev.Usage.PromptTokens, ev.Usage.CompletionTokens,
				ev.Usage.TotalTokens, ev.Usage.ReasoningTokens)
		} else {
			json = fmt.Sprintf(`{"content":%q,"reasoning":%q}`,
				ev.Content, ev.Reasoning)
		}
		writeEvent(json)
	})
	if err != nil {
		// If we already started streaming, we can only emit an
		// error event (headers are gone). Otherwise it's safe to
		// return a JSON error.
		if c.Writer.Written() {
			writeEvent(fmt.Sprintf(`{"error":%q}`, err.Error()))
		} else {
			c.JSON(mapAIError(err), gin.H{"error": err.Error()})
		}
	}
}

// mapAIError translates the gateway sentinel errors into HTTP
// statuses so the client gets the right code, not a blanket 500.
func mapAIError(err error) int {
	switch {
	case errors.Is(err, ai.ErrNotConfigured):
		return http.StatusServiceUnavailable
	case errors.Is(err, ai.ErrKeyInvalid):
		return http.StatusUnauthorized
	case errors.Is(err, ai.ErrRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, ai.ErrInsufficient):
		return http.StatusPaymentRequired
	case errors.Is(err, ai.ErrModelNotFound):
		return http.StatusNotFound
	case errors.Is(err, ai.ErrTimeout):
		return http.StatusGatewayTimeout
	default:
		return http.StatusBadGateway
	}
}
