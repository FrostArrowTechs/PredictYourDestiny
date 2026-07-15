package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"predictdestiny/internal/ai"
	"predictdestiny/internal/ai/prompt"
	"predictdestiny/internal/fortune"
)

// CompatibilityHandler exposes the compatibility endpoints.
type CompatibilityHandler struct {
	Gateway ai.Gateway
}

// subjectReq is one person's birth data.
type subjectReq struct {
	Year  int `json:"year" binding:"required,min=1900,max=2100"`
	Month int `json:"month" binding:"required,min=1,max=12"`
	Day   int `json:"day" binding:"required,min=1,max=31"`
}

// compatibilityComputeReq is the input for compatibility calculation.
type compatibilityComputeReq struct {
	Year           int         `json:"year" binding:"required,min=1900,max=2100"`
	Month          int         `json:"month" binding:"required,min=1,max=12"`
	Day            int         `json:"day" binding:"required,min=1,max=31"`
	Second         *subjectReq `json:"second" binding:"required"`
	Lang           string      `json:"lang"`
	InterpretDepth string      `json:"interpretDepth"`
	Model          string      `json:"model"`
	Stream         bool        `json:"stream"`
}

// toFortuneInput maps the request to engine input.
func (r compatibilityComputeReq) toFortuneInput() fortune.Input {
	lang := r.Lang
	if lang == "" {
		lang = "zh-CN"
	}
	in := fortune.Input{
		Year:           r.Year,
		Month:          r.Month,
		Day:            r.Day,
		Lang:           lang,
		InterpretDepth: r.InterpretDepth,
	}
	if r.Second != nil {
		in.Second = &fortune.Input{
			Year:  r.Second.Year,
			Month: r.Second.Month,
			Day:   r.Second.Day,
		}
	}
	return in
}

// Compute returns compatibility between two subjects.
func (h *CompatibilityHandler) Compute(c *gin.Context) {
	var req compatibilityComputeReq
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

func (h *CompatibilityHandler) computeChart(req compatibilityComputeReq) (*fortune.Result, error) {
	eng, ok := fortune.Fortune(fortune.KindCompatibility)
	if !ok {
		return nil, fmt.Errorf("compatibility engine unavailable")
	}
	return eng.Compute(req.toFortuneInput())
}

// Interpret computes compatibility then asks AI for detailed analysis.
func (h *CompatibilityHandler) Interpret(c *gin.Context) {
	var req compatibilityComputeReq
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

	spec, err := prompt.CompatibilityBuild(req.toFortuneInput(), res)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	model := h.resolveModel(req.Model, spec)
	if model == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no AI model configured"})
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

func (h *CompatibilityHandler) resolveModel(requested string, spec *fortune.PromptSpec) string {
	cat := h.Gateway.ListModels()
	if requested != "" {
		if _, ok := cat.Find(requested); ok {
			return requested
		}
	}
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

func (h *CompatibilityHandler) interpretSync(c *gin.Context, model string, msgs []ai.Message, opts ai.Options) {
	resp, err := h.Gateway.Chat(c.Request.Context(), model, msgs, opts)
	if err != nil {
		c.JSON(mapCompatAIError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"content":   resp.Content,
		"reasoning": resp.Reasoning,
		"model":     resp.Model,
		"usage":     resp.Usage,
	})
}

func (h *CompatibilityHandler) interpretStream(c *gin.Context, model string, msgs []ai.Message, opts ai.Options) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
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
		var json string
		if ev.Usage != nil {
			json = fmt.Sprintf(`{"content":%q,"reasoning":%q,"usage":{"prompt_tokens":%d,"completion_tokens":%d,"total_tokens":%d,"reasoning_tokens":%d}}`,
				ev.Content, ev.Reasoning,
				ev.Usage.PromptTokens, ev.Usage.CompletionTokens,
				ev.Usage.TotalTokens, ev.Usage.ReasoningTokens)
		} else {
			json = fmt.Sprintf(`{"content":%q,"reasoning":%q}`, ev.Content, ev.Reasoning)
		}
		writeEvent(json)
	})
	if err != nil {
		if c.Writer.Written() {
			writeEvent(fmt.Sprintf(`{"error":%q}`, err.Error()))
		} else {
			c.JSON(mapCompatAIError(err), gin.H{"error": err.Error()})
		}
	}
}

func mapCompatAIError(err error) int {
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