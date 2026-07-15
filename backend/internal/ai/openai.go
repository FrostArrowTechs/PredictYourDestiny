package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/tidwall/gjson"

	"predictdestiny/internal/model"
)

// OpenAIGateway talks to an OpenAI-compatible Chat Completions
// endpoint (New-API, Azure OpenAI, vLLM, Ollama, …) via the official
// openai-go SDK. It is the only production Gateway implementation
// today.
//
// It holds no state of its own beyond an *http.Client (for tuning
// timeouts) and a SettingsReader (for reading base_url / key / model
// list). A new openai.Client is built per request because the base
// URL / key / model list may change at runtime through the settings
// table — building the SDK client is cheap (it's just a config
// holder), so doing it per call keeps config live without a reload
// step.
//
// SettingsReader is an interface rather than the concrete
// *store.SettingStore so the gateway can be unit-tested against a
// fake reader without spinning up PostgreSQL. The production store
// satisfies it implicitly.
type OpenAIGateway struct {
	HTTP     *http.Client
	Settings SettingsReader
}

// SettingsReader is the slice of *store.SettingStore the gateway
// actually needs: typed read access to the dynamic-config table.
// Defined here (not in store) so the ai package owns its dependency
// contract — the store stays free of ai-specific assumptions.
type SettingsReader interface {
	Get(key string) (string, bool)
	GetDefault(key, fallback string) string
}

// NewOpenAIGateway builds a gateway with a sane HTTP client: idle
// conns are pooled and reused, but there is no overall timeout —
// streaming responses can legitimately run for minutes, so the
// caller's context is the only deadline that matters.
func NewOpenAIGateway(s SettingsReader) *OpenAIGateway {
	return &OpenAIGateway{
		HTTP: &http.Client{
			Timeout: 0, // rely on ctx for the deadline
			Transport: &http.Transport{
				MaxIdleConns:        20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		Settings: s,
	}
}

// configured reports whether the gateway can make a call. The handler
// checks this before invoking so it can return a friendly 503 instead
// of a generic upstream error.
func (g *OpenAIGateway) configured() error {
	base, _ := g.Settings.Get(model.SettingAIBaseURL)
	if strings.TrimSpace(base) == "" {
		return ErrNotConfigured
	}
	if _, ok := g.Settings.Get(model.SettingAIAPIKey); !ok {
		return ErrNotConfigured
	}
	return nil
}

// resolveModel picks the model id: the argument wins, else the
// configured default free model.
func (g *OpenAIGateway) resolveModel(modelID string) string {
	if modelID != "" {
		return modelID
	}
	if v, ok := g.Settings.Get(model.SettingAIDefaultModel); ok && v != "" {
		return v
	}
	cat := g.ListModels()
	if len(cat.Free) > 0 {
		return cat.Free[0].ID
	}
	if len(cat.Paid) > 0 {
		return cat.Paid[0].ID
	}
	return ""
}

// client builds a fresh openai.Client wired to the current settings.
// The SDK client is just config — constructing one per request costs
// nothing measurable and means a settings change is picked up on the
// very next call without a reload hook.
func (g *OpenAIGateway) client() (openai.Client, error) {
	if err := g.configured(); err != nil {
		return openai.Client{}, err
	}
	base, _ := g.Settings.Get(model.SettingAIBaseURL)
	key, _ := g.Settings.Get(model.SettingAIAPIKey)
	return openai.NewClient(
		option.WithBaseURL(strings.TrimRight(base, "/")),
		option.WithAPIKey(key),
		option.WithHTTPClient(g.HTTP),
	), nil
}

// toSDKMessages converts our Message slice into the SDK's union param.
// We only emit system / user / assistant text messages — the product
// has no tool-calling surface yet.
func toSDKMessages(msgs []Message) []openai.ChatCompletionMessageParamUnion {
	out := make([]openai.ChatCompletionMessageParamUnion, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case RoleSystem:
			out = append(out, openai.ChatCompletionMessageParamUnion{
				OfSystem: &openai.ChatCompletionSystemMessageParam{
					Content: openai.ChatCompletionSystemMessageParamContentUnion{
						OfString: openai.String(m.Content),
					},
				},
			})
		case RoleAssistant:
			out = append(out, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &openai.ChatCompletionAssistantMessageParam{
					Content: openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openai.String(m.Content),
					},
				},
			})
		default: // RoleUser and anything unknown
			out = append(out, openai.ChatCompletionMessageParamUnion{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(m.Content),
					},
				},
			})
		}
	}
	return out
}

// params builds the ChatCompletionNewParams from our Options + msgs.
func params(modelID string, msgs []Message, opts Options, stream bool) openai.ChatCompletionNewParams {
	p := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(modelID),
		Messages: toSDKMessages(msgs),
	}
	if opts.MaxTokens > 0 {
		p.MaxTokens = openai.Int(int64(opts.MaxTokens))
	}
	if opts.Temperature != 0 {
		p.Temperature = openai.Float(opts.Temperature)
	}
	if opts.TopP != 0 {
		p.TopP = openai.Float(opts.TopP)
	}
	if len(opts.Stop) > 0 {
		p.Stop = openai.ChatCompletionNewParamsStopUnion{
			OfStringArray: opts.Stop,
		}
	}
	if stream {
		p.StreamOptions = openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: openai.Bool(true), // get a final usage chunk
		}
	}
	return p
}

// ─── Chat (synchronous) ───────────────────────────────────────────

func (g *OpenAIGateway) Chat(ctx context.Context, modelID string, msgs []Message, opts Options) (*Response, error) {
	cli, err := g.client()
	if err != nil {
		return nil, err
	}
	resolved := g.resolveModel(modelID)
	if resolved == "" {
		return nil, fmt.Errorf("%w: no model resolved (set ai.default_model or pass a model)", ErrNotConfigured)
	}

	completion, err := cli.Chat.Completions.New(ctx, params(resolved, msgs, opts, false))
	if err != nil {
		return nil, translateError(err)
	}

	out := &Response{Model: completion.Model}
	if len(completion.Choices) > 0 {
		out.Content = completion.Choices[0].Message.Content
	}
	out.Reasoning = extractReasoning(completion.RawJSON(), false)
	out.Usage = usageFromSDK(completion.Usage)
	return out, nil
}

// ─── StreamChat (SSE) ─────────────────────────────────────────────

func (g *OpenAIGateway) StreamChat(ctx context.Context, modelID string, msgs []Message, opts Options, onEvent func(StreamEvent)) error {
	cli, err := g.client()
	if err != nil {
		return err
	}
	resolved := g.resolveModel(modelID)
	if resolved == "" {
		return fmt.Errorf("%w: no model resolved", ErrNotConfigured)
	}

	stream := cli.Chat.Completions.NewStreaming(ctx, params(resolved, msgs, opts, true))
	defer stream.Close()

	for stream.Next() {
		chunk := stream.Current()
		// content + non-standard reasoning_content come from the
		// chunk's raw JSON; the SDK only exposes Content.
		ev := StreamEvent{}
		if len(chunk.Choices) > 0 {
			ev.Content = chunk.Choices[0].Delta.Content
			ev.Reasoning = gjson.Get(chunk.RawJSON(), "choices.0.delta.reasoning_content").String()
		}
		// usage chunk: choices empty, usage populated
		if u, ok := usageFromChunk(chunk); ok {
			ev.Usage = &u
		}
		// Skip spurious empty opening events (role-only chunk) so
		// the UI doesn't render blank tokens; forward anything that
		// carries text OR usage.
		if ev.Content == "" && ev.Reasoning == "" && ev.Usage == nil {
			continue
		}
		onEvent(ev)
	}
	if err := stream.Err(); err != nil {
		// context cancel is not a gateway error — surface it raw so
		// the handler can map it to the right status.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return translateError(err)
	}
	onEvent(StreamEvent{Done: true})
	return nil
}

// ─── ListModels ───────────────────────────────────────────────────

// ListModels parses the ai.models setting JSON into a tier-grouped
// catalog. Malformed JSON yields an empty catalog rather than an
// error — the admin UI is where errors should surface, not the chat
// path. Entries with an empty ID are dropped; unknown tiers default
// to free so a typo doesn't hide a model.
func (g *OpenAIGateway) ListModels() ModelCatalog {
	raw, ok := g.Settings.Get(model.SettingAIModels)
	if !ok || strings.TrimSpace(raw) == "" {
		return ModelCatalog{}
	}
	var entries []ModelEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return ModelCatalog{}
	}
	var cat ModelCatalog
	for _, e := range entries {
		if e.ID == "" {
			continue
		}
		if e.Tier == TierPaid {
			cat.Paid = append(cat.Paid, e)
		} else {
			e.Tier = TierFree
			cat.Free = append(cat.Free, e)
		}
	}
	return cat
}

// ─── helpers ──────────────────────────────────────────────────────

// extractReasoning pulls the non-standard `reasoning_content` field
// out of a ChatCompletion's raw JSON. Qwen3 / DeepSeek-R1 style
// models emit it alongside `content`. fromMessage=true reads from
// choices[0].message.reasoning_content (non-stream); false reads from
// choices[0].delta.reasoning_content (stream) — but we handle stream
// inline above, so here we only need the message path.
func extractReasoning(raw string, _ bool) string {
	if raw == "" {
		return ""
	}
	r := gjson.Get(raw, "choices.0.message.reasoning_content")
	if !r.Exists() {
		return ""
	}
	return r.String()
}

// usageFromSDK converts the SDK's CompletionUsage into our Usage. The
// reasoning-token breakdown is part of the standard schema now.
func usageFromSDK(u openai.CompletionUsage) Usage {
	return Usage{
		PromptTokens:     int(u.PromptTokens),
		CompletionTokens: int(u.CompletionTokens),
		TotalTokens:      int(u.TotalTokens),
		ReasoningTokens:  int(u.CompletionTokensDetails.ReasoningTokens),
	}
}

// usageFromChunk returns the usage from a streaming chunk when the
// chunk is the terminal usage carrier (choices empty, usage
// populated). ok=false otherwise.
func usageFromChunk(chunk openai.ChatCompletionChunk) (Usage, bool) {
	// Usage is only populated on the final chunk when
	// stream_options.include_usage was set. The SDK types Usage as a
	// non-pointer struct that's always "present"; we detect a real
	// payload by checking whether TotalTokens > 0.
	if chunk.Usage.TotalTokens == 0 {
		return Usage{}, false
	}
	return usageFromSDK(chunk.Usage), true
}

// translateError maps an openai-go error onto our sentinel errors so
// the handler can render the right HTTP status. Non-API errors (network,
// context) pass through unchanged.
func translateError(err error) error {
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusUnauthorized:
			return fmt.Errorf("%w: %s", ErrKeyInvalid, apiErr.Message)
		case http.StatusTooManyRequests:
			return fmt.Errorf("%w: %s", ErrRateLimited, apiErr.Message)
		case http.StatusPaymentRequired:
			return fmt.Errorf("%w: %s", ErrInsufficient, apiErr.Message)
		case http.StatusNotFound:
			return fmt.Errorf("%w: %s", ErrModelNotFound, apiErr.Message)
		default:
			return fmt.Errorf("%w: HTTP %d: %s", ErrUpstream, apiErr.StatusCode, apiErr.Message)
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	return err
}
