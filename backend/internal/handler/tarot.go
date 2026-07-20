package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"predictdestiny/internal/ai"
	"predictdestiny/internal/ai/prompt"
	"predictdestiny/internal/fortune"
)

// TarotHandler exposes the tarot (塔罗) endpoints.
//
// POST /api/tarot/draw      — draw a spread, no AI tokens
// POST /api/tarot/interpret — draw then AI reading with streaming
type TarotHandler struct {
	Gateway ai.Gateway
	DB      *gorm.DB
}

// tarotDrawReq is the input for a tarot reading.
type tarotDrawReq struct {
	// Spread selects the spread: "single" / "three" / "celtic".
	// Empty defaults to "three".
	Spread string `json:"spread"`
	// Question is the user's question (optional).
	Question string `json:"question"`
	// Common fields
	Lang           string `json:"lang"`
	InterpretDepth string `json:"interpretDepth"`
	Model          string `json:"model"`
	Stream         bool   `json:"stream"`
}

// toFortuneInput encodes the spread id into the question via the
// "[spread]" prefix convention the engine understands.
func (r tarotDrawReq) toFortuneInput() fortune.Input {
	lang := r.Lang
	if lang == "" {
		lang = "zh-CN"
	}
	// Validate spread id; fall back to the engine default.
	spread := r.Spread
	if spread != "" {
		if _, ok := fortune.SpreadDef(spread); !ok {
			spread = ""
		}
	}
	question := r.Question
	if spread != "" {
		question = fmt.Sprintf("[%s]%s", spread, question)
	}
	return fortune.Input{
		Question:       question,
		Lang:           lang,
		InterpretDepth: r.InterpretDepth,
	}
}

// Draw returns the drawn cards for the requested spread.
func (h *TarotHandler) Draw(c *gin.Context) {
	var req tarotDrawReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.drawCards(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *TarotHandler) drawCards(req tarotDrawReq) (*fortune.Result, error) {
	if h.DB == nil {
		return nil, fmt.Errorf("tarot: database not configured")
	}
	eng := fortune.TarotEngine{DB: h.DB}
	return eng.Compute(req.toFortuneInput())
}

// Interpret draws cards then asks AI for a reading. Also registered as
// /api/tarot/compute for route-name consistency with other engines.
func (h *TarotHandler) Compute(c *gin.Context) { h.Draw(c) }

// Interpret draws cards then asks AI for a reading.
func (h *TarotHandler) Interpret(c *gin.Context) {
	var req tarotDrawReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.drawCards(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.Gateway == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI gateway not configured"})
		return
	}

	spec, err := prompt.TarotBuild(req.toFortuneInput(), res)
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
	opts := ai.Options{Temperature: 0.8} // slightly higher for evocative readings

	if req.Stream {
		h.interpretStream(c, model, msgs, opts)
		return
	}
	h.interpretSync(c, model, msgs, opts)
}

func (h *TarotHandler) resolveModel(requested string, spec *fortune.PromptSpec) string {
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

func (h *TarotHandler) interpretSync(c *gin.Context, model string, msgs []ai.Message, opts ai.Options) {
	resp, err := h.Gateway.Chat(c.Request.Context(), model, msgs, opts)
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

func (h *TarotHandler) interpretStream(c *gin.Context, model string, msgs []ai.Message, opts ai.Options) {
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
