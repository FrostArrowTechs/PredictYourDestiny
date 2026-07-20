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

// DreamHandler exposes the dream interpretation endpoints.
//
// /api/dream/compute  — keyword search against reference table, free.
// /api/dream/interpret — AI reading based on matched traditional meanings.
type DreamHandler struct {
	Gateway ai.Gateway
	DB      *gorm.DB
}

// dreamComputeReq is the input for both /compute and /interpret.
type dreamComputeReq struct {
	Question       string `json:"question" binding:"required"`
	Lang           string `json:"lang"`
	InterpretDepth string `json:"interpretDepth"`
	Model          string `json:"model"`
	Stream         bool   `json:"stream"`
}

// toFortuneInput maps the wire request into the engine's Input.
func (r dreamComputeReq) toFortuneInput() fortune.Input {
	lang := r.Lang
	if lang == "" {
		lang = "zh-CN"
	}
	return fortune.Input{
		Question:       r.Question,
		Lang:           lang,
		InterpretDepth: r.InterpretDepth,
	}
}

// Compute runs the dream engine and returns matched traditional meanings.
func (h *DreamHandler) Compute(c *gin.Context) {
	var req dreamComputeReq
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

// computeChart runs the dream engine with the DB dependency.
func (h *DreamHandler) computeChart(req dreamComputeReq) (*fortune.Result, error) {
	eng := fortune.DreamEngine{DB: h.DB}
	return eng.Compute(req.toFortuneInput())
}

// Interpret computes matches then asks the AI gateway for a personalized reading.
func (h *DreamHandler) Interpret(c *gin.Context) {
	var req dreamComputeReq
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

	spec, err := prompt.DreamBuild(req.toFortuneInput(), res)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	model, authorized := authorizeAIRequest(c, h.DB, h.Gateway, req.Model, spec.Tier)
	if !authorized {
		return
	}
	if model == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no AI model configured"})
		return
	}

	msgs := ai.MessagesFromPrompt(spec)
	opts := ai.Options{Temperature: 0.8} // slightly higher for creative interpretation

	if req.Stream {
		h.interpretStream(c, model, msgs, opts)
		return
	}
	h.interpretSync(c, model, msgs, opts)
}

// resolveModel validates and resolves the model ID.
func (h *DreamHandler) resolveModel(requested string, spec *fortune.PromptSpec) string {
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

// interpretSync does one blocking completion.
func (h *DreamHandler) interpretSync(c *gin.Context, model string, msgs []ai.Message, opts ai.Options) {
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

// interpretStream forwards gateway deltas to the client as SSE.
func (h *DreamHandler) interpretStream(c *gin.Context, model string, msgs []ai.Message, opts ai.Options) {
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
			json = fmt.Sprintf(`{"content":%q,"reasoning":%q}`,
				ev.Content, ev.Reasoning)
		}
		writeEvent(json)
	})
	if err != nil {
		if c.Writer.Written() {
			writeEvent(fmt.Sprintf(`{"error":%q}`, err.Error()))
		} else {
			c.JSON(mapAIError(err), gin.H{"error": err.Error()})
		}
	}
}

// mapAIError translates gateway errors to HTTP statuses.
func mapDreamAIError(err error) int {
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
