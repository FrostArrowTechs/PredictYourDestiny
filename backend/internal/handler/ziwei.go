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

// ZiweiHandler exposes the 紫微斗数 endpoints.
//
// POST /api/ziwei/compute   — chart compute, no AI tokens
// POST /api/ziwei/interpret — AI reading with streaming support
type ZiweiHandler struct {
	Gateway ai.Gateway
	DB      *gorm.DB
}

// ziweiComputeReq is the input for a Ziwei chart.
type ziweiComputeReq struct {
	fortune.BirthContext
	Gender             int    `json:"gender" binding:"min=0,max=1"`
	Lang               string `json:"lang"`
	InterpretDepth     string `json:"interpretDepth"`
	Model              string `json:"model"`
	Stream             bool   `json:"stream"`
	ZiweiLeapMonthRule string `json:"ziweiLeapMonthRule"`
}

func (r ziweiComputeReq) toFortuneInput() (fortune.Input, error) {
	birth, hour, minute, err := r.BirthContext.RequiredClock()
	if err != nil {
		return fortune.Input{}, err
	}
	if err := validateBirthYear(birth, 1900, 2100); err != nil {
		return fortune.Input{}, err
	}
	lang := r.Lang
	if lang == "" {
		lang = "zh-CN"
	}
	return fortune.Input{
		Birth:              &birth,
		Year:               birth.Year,
		Month:              birth.Month,
		Day:                birth.Day,
		Hour:               hour,
		Minute:             minute,
		Gender:             fortune.Gender(r.Gender),
		ZiweiLeapMonthRule: r.ZiweiLeapMonthRule,
		Lang:               lang,
		InterpretDepth:     r.InterpretDepth,
	}, nil
}

func (r ziweiComputeReq) toUncertaintyInput() (fortune.Input, error) {
	birth, err := r.BirthContext.Normalized()
	if err != nil {
		return fortune.Input{}, err
	}
	if err := validateBirthYear(birth, 1900, 2100); err != nil {
		return fortune.Input{}, err
	}
	lang := r.Lang
	if lang == "" {
		lang = "zh-CN"
	}
	return fortune.Input{Birth: &birth, Year: birth.Year, Month: birth.Month, Day: birth.Day,
		Gender: fortune.Gender(r.Gender), ZiweiLeapMonthRule: r.ZiweiLeapMonthRule, Lang: lang, InterpretDepth: r.InterpretDepth}, nil
}

// Compute returns the Ziwei chart.
func (h *ZiweiHandler) Compute(c *gin.Context) {
	var req ziweiComputeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.computeChart(req, true)
	if err != nil {
		writeBirthComputeError(c, err)
		return
	}
	writeComputedResult(c, h.DB, fortune.KindZiwei, req, res)
}

func (h *ZiweiHandler) computeChart(req ziweiComputeReq, allowUncertainty bool) (*fortune.Result, error) {
	eng, ok := fortune.Fortune(fortune.KindZiwei)
	if !ok {
		return nil, fmt.Errorf("ziwei engine unavailable")
	}
	var input fortune.Input
	var err error
	if allowUncertainty {
		input, err = req.toUncertaintyInput()
	} else {
		input, err = req.toFortuneInput()
	}
	if err != nil {
		return nil, err
	}
	if allowUncertainty {
		return fortune.ComputeWithBirthUncertainty(eng, input)
	}
	result, err := eng.Compute(input)
	fortune.AttachBirthMetadata(result, input)
	return result, err
}

// Interpret computes the chart then asks AI for a reading.
func (h *ZiweiHandler) Interpret(c *gin.Context) {
	var req ziweiComputeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.computeChart(req, false)
	if err != nil {
		writeBirthComputeError(c, err)
		return
	}

	if h.Gateway == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI gateway not configured"})
		return
	}

	input, err := req.toFortuneInput()
	if err != nil {
		writeBirthComputeError(c, err)
		return
	}
	spec, err := prompt.ZiweiBuild(input, res)
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

func (h *ZiweiHandler) resolveModel(requested string, spec *fortune.PromptSpec) string {
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

func (h *ZiweiHandler) interpretSync(c *gin.Context, model string, msgs []ai.Message, opts ai.Options) {
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

func (h *ZiweiHandler) interpretStream(c *gin.Context, model string, msgs []ai.Message, opts ai.Options) {
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
