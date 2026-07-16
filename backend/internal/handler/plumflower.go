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

// PlumFlowerHandler exposes the 梅花易数 endpoints.
type PlumFlowerHandler struct {
	Gateway ai.Gateway
}

type plumflowerComputeReq struct {
	Year           int    `json:"year"`
	Month          int    `json:"month"`
	Day            int    `json:"day"`
	Hour           int    `json:"hour"`
	Question       string `json:"question"` // optional: for number method, "12,34,56"
	Lang           string `json:"lang"`
	InterpretDepth string `json:"interpretDepth"`
	Model          string `json:"model"`
	Stream         bool   `json:"stream"`
}

func (r plumflowerComputeReq) toFortuneInput() fortune.Input {
	lang := r.Lang
	if lang == "" {
		lang = "zh-CN"
	}
	return fortune.Input{
		Year:           r.Year,
		Month:          r.Month,
		Day:            r.Day,
		Hour:           r.Hour,
		Question:       r.Question,
		Lang:           lang,
		InterpretDepth: r.InterpretDepth,
	}
}

func (h *PlumFlowerHandler) Compute(c *gin.Context) {
	var req plumflowerComputeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	eng, ok := fortune.Fortune(fortune.KindPlumFlower)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "plumflower engine unavailable"})
		return
	}
	res, err := eng.Compute(req.toFortuneInput())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *PlumFlowerHandler) Interpret(c *gin.Context) {
	var req plumflowerComputeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	eng, ok := fortune.Fortune(fortune.KindPlumFlower)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "plumflower engine unavailable"})
		return
	}
	res, err := eng.Compute(req.toFortuneInput())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.Gateway == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI gateway not configured"})
		return
	}

	spec, err := prompt.PlumFlowerBuild(req.toFortuneInput(), res)
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

func (h *PlumFlowerHandler) resolveModel(requested string, spec *fortune.PromptSpec) string {
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

func (h *PlumFlowerHandler) interpretSync(c *gin.Context, model string, msgs []ai.Message, opts ai.Options) {
	resp, err := h.Gateway.Chat(c.Request.Context(), model, msgs, opts)
	if err != nil {
		c.JSON(mapGenericAIError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"content": resp.Content, "reasoning": resp.Reasoning,
		"model": resp.Model, "usage": resp.Usage,
	})
}

func (h *PlumFlowerHandler) interpretStream(c *gin.Context, model string, msgs []ai.Message, opts ai.Options) {
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
		var s string
		if ev.Usage != nil {
			s = fmt.Sprintf(`{"content":%q,"reasoning":%q,"usage":{"prompt_tokens":%d,"completion_tokens":%d,"total_tokens":%d,"reasoning_tokens":%d}}`,
				ev.Content, ev.Reasoning, ev.Usage.PromptTokens, ev.Usage.CompletionTokens, ev.Usage.TotalTokens, ev.Usage.ReasoningTokens)
		} else {
			s = fmt.Sprintf(`{"content":%q,"reasoning":%q}`, ev.Content, ev.Reasoning)
		}
		writeEvent(s)
	})
	if err != nil {
		if c.Writer.Written() {
			writeEvent(fmt.Sprintf(`{"error":%q}`, err.Error()))
		} else {
			c.JSON(mapGenericAIError(err), gin.H{"error": err.Error()})
		}
	}
}
