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

// NameHandler exposes name analysis endpoints.
//
// POST /api/name/compute   — pure Five格 calculation, no AI tokens
// POST /api/name/interpret — AI reading with streaming support
type NameHandler struct {
	Gateway ai.Gateway
	DB      *gorm.DB
}

// nameComputeReq is the input for name analysis.
type nameComputeReq struct {
	FullName       string `json:"fullName" binding:"required,min=2"`
	Gender         int    `json:"gender" binding:"min=0,max=1"`
	Lang           string `json:"lang"`
	InterpretDepth string `json:"interpretDepth"`
	Model          string `json:"model"`
	Stream         bool   `json:"stream"`
}

// Compute runs the Five格 calculation and returns the structured result.
func (h *NameHandler) Compute(c *gin.Context) {
	var req nameComputeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	eng := fortune.NameEngine{DB: h.DB}

	// Build Input from request (use Question field for full name)
	input := fortune.Input{
		Question:       req.FullName,
		Lang:           req.Lang,
		InterpretDepth: req.InterpretDepth,
	}

	res, err := eng.Compute(input)
	if err != nil {
		writeNameComputeError(c, err)
		return
	}

	c.JSON(http.StatusOK, res)
}

// Interpret computes the Five格 then asks AI for a reading.
func (h *NameHandler) Interpret(c *gin.Context) {
	var req nameComputeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	eng := fortune.NameEngine{DB: h.DB}

	// Build Input from request
	input := fortune.Input{
		Question:       req.FullName,
		Lang:           req.Lang,
		InterpretDepth: req.InterpretDepth,
	}

	res, err := eng.Compute(input)
	if err != nil {
		writeNameComputeError(c, err)
		return
	}

	if h.Gateway == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI gateway not configured"})
		return
	}

	spec, err := prompt.NameBuild(input, res)
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
	opts := ai.Options{Temperature: 0.7}

	if req.Stream {
		h.interpretStream(c, model, msgs, opts)
		return
	}
	h.interpretSync(c, model, msgs, opts)
}

func writeNameComputeError(c *gin.Context, err error) {
	var unknown *fortune.UnknownCharactersError
	switch {
	case errors.As(err, &unknown):
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":             "name contains characters missing from the configured stroke dictionary",
			"unknownCharacters": unknown.Characters,
		})
	case errors.Is(err, fortune.ErrStrokeDictionaryUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

// resolveModel picks the model to use.
func (h *NameHandler) resolveModel(requested string, spec *fortune.PromptSpec) string {
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

func (h *NameHandler) interpretSync(c *gin.Context, model string, msgs []ai.Message, opts ai.Options) {
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

func (h *NameHandler) interpretStream(c *gin.Context, model string, msgs []ai.Message, opts ai.Options) {
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
