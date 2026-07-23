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

// AstrologyHandler exposes Western natal chart endpoints.
//
// POST /api/astrology/compute   — planetary position calculation, no AI
// POST /api/astrology/interpret — AI reading with streaming support
type AstrologyHandler struct {
	Gateway ai.Gateway
	DB      *gorm.DB
}

// astrologyComputeReq is the input for natal chart calculation.
type astrologyComputeReq struct {
	fortune.BirthContext
	Lang           string `json:"lang"`
	InterpretDepth string `json:"interpretDepth"`
	Model          string `json:"model"`
	Stream         bool   `json:"stream"`
}

func (r astrologyComputeReq) toFortuneInput() (fortune.Input, error) {
	birth, hour, minute, err := r.BirthContext.RequiredClock()
	if err != nil {
		return fortune.Input{}, err
	}
	if err := validateBirthYear(birth, 1900, 2100); err != nil {
		return fortune.Input{}, err
	}
	longitude := 0.0
	if birth.Longitude != nil {
		longitude = *birth.Longitude
	}
	return fortune.Input{
		Birth: &birth,
		Year:  birth.Year, Month: birth.Month, Day: birth.Day,
		Hour: hour, Minute: minute,
		Longitude: longitude, Lang: r.Lang, InterpretDepth: r.InterpretDepth,
	}, nil
}

func (r astrologyComputeReq) toUncertaintyInput() (fortune.Input, error) {
	birth, err := r.BirthContext.Normalized()
	if err != nil {
		return fortune.Input{}, err
	}
	if err := validateBirthYear(birth, 1900, 2100); err != nil {
		return fortune.Input{}, err
	}
	longitude := 0.0
	if birth.Longitude != nil {
		longitude = *birth.Longitude
	}
	return fortune.Input{Birth: &birth, Year: birth.Year, Month: birth.Month, Day: birth.Day,
		Longitude: longitude, Lang: r.Lang, InterpretDepth: r.InterpretDepth}, nil
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

	input, err := req.toUncertaintyInput()
	if err != nil {
		writeBirthComputeError(c, err)
		return
	}

	res, err := fortune.ComputeWithBirthUncertainty(eng, input)
	if err != nil {
		writeBirthComputeError(c, err)
		return
	}
	writeComputedResult(c, h.DB, fortune.KindAstrology, req, res)
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

	input, err := req.toFortuneInput()
	if err != nil {
		writeBirthComputeError(c, err)
		return
	}

	res, err := eng.Compute(input)
	if err != nil {
		writeBirthComputeError(c, err)
		return
	}
	fortune.AttachBirthMetadata(res, input)

	if h.Gateway == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI gateway not configured"})
		return
	}

	spec, err := prompt.AstrologyBuild(input, res)
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
	ctx := c.Request.Context()
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
