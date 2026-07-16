package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"predictdestiny/internal/ai"
	"predictdestiny/internal/ai/prompt"
	"predictdestiny/internal/fortune"
)

// AstrologyHandler exposes Western natal chart endpoints.
//
// POST /api/astrology/compute   — planetary position calculation, no AI
// POST /api/astrology/interpret — AI reading with streaming support
type AstrologyHandler struct {
	Gateway ai.Gateway
}

// astrologyComputeReq is the input for natal chart calculation.
type astrologyComputeReq struct {
	Year     int     `json:"year" binding:"required,min=1900,max=2100"`
	Month    int     `json:"month" binding:"required,min=1,max=12"`
	Day      int     `json:"day" binding:"required,min=1,max=31"`
	Hour     int     `json:"hour" binding:"min=0,max=23"`
	Minute   int     `json:"minute" binding:"min=0,max=59"`
	Longitude float64 `json:"longitude"`
	Lang     string  `json:"lang"`
	InterpretDepth string `json:"interpretDepth"`
	Model    string  `json:"model"`
	Stream   bool    `json:"stream"`
}

// Compute returns the natal chart data without AI interpretation.
func (h *AstrologyHandler) Compute(c *gin.Context) {
	var req astrologyComputeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	eng, ok := fortune.Fortune(fortune.KindAstrology)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "astrology engine unavailable"})
		return
	}

	// Build Input from request
	input := fortune.Input{
		Year:      req.Year,
		Month:     req.Month,
		Day:       req.Day,
		Hour:      req.Hour,
		Minute:    req.Minute,
		Longitude: req.Longitude,
		Lang:      req.Lang,
	}

	res, err := eng.Compute(input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res.Data)
}

// Interpret returns AI interpretation of the natal chart.
func (h *AstrologyHandler) Interpret(c *gin.Context) {
	var req astrologyComputeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	eng, ok := fortune.Fortune(fortune.KindAstrology)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "astrology engine unavailable"})
		return
	}

	// Build Input from request
	input := fortune.Input{
		Year:      req.Year,
		Month:     req.Month,
		Day:       req.Day,
		Hour:      req.Hour,
		Minute:    req.Minute,
		Longitude: req.Longitude,
		Lang:      req.Lang,
		InterpretDepth: req.InterpretDepth,
	}

	res, err := eng.Compute(input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.Gateway == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI gateway not configured"})
		return
	}

	spec, err := prompt.AstrologyBuild(input, res)
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

// resolveModel picks the model to use.
func (h *AstrologyHandler) resolveModel(requested string, spec *fortune.PromptSpec) string {
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

func (h *AstrologyHandler) interpretSync(c *gin.Context, model string, msgs []ai.Message, opts ai.Options) {
	ctx := context.Background()
	resp, err := h.Gateway.Chat(ctx, model, msgs, opts)
	if err != nil {
		c.JSON(mapGenericAIError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"content":   resp.Content,
		"reasoning": resp.Reasoning,
		"model":     resp.Model,
		"usage":     resp.Usage,
	})
}

func (h *AstrologyHandler) interpretStream(c *gin.Context, model string, msgs []ai.Message, opts ai.Options) {
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
			c.JSON(mapGenericAIError(err), gin.H{"error": err.Error()})
		}
	}
}