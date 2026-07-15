// Package ai wraps the LLM gateway that powers every interpretation.
//
// The product talks to a New-API (OpenAI-compatible) instance, so the
// whole package is built around the OpenAI Chat Completions wire
// format. The Gateway interface abstracts that one protocol so a
// future provider (Anthropic native, Bedrock …) can slot in without
// the handler layer changing.
//
// Configuration is dynamic, not bootstrap: base_url, api_key and the
// model list live in the settings table and are read through
// SettingStore on every call. That lets the admin rotate keys or add
// a model without restarting the process. The gateway holds no
// secrets of its own — it borrows them from the store for the
// duration of a request.
package ai

import (
	"context"
	"errors"

	"predictdestiny/internal/fortune"
)

// Role names a message author. We keep the OpenAI vocabulary so the
// client never has to translate.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is one turn in a chat. Content is plain text; the gateway
// does not currently send tool/function calls.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// Options tunes a single completion. Zero values mean "let the
// provider pick a default". Temperature 0 → deterministic.
type Options struct {
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

// Usage records token accounting returned by the provider. Both
// streaming and non-streaming paths populate it (streaming uses
// stream_options.include_usage to get a final usage chunk).
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	// ReasoningTokens counts the model's hidden chain-of-thought
	// (Qwen3 / DeepSeek-R1 style). It is a subset of
	// CompletionTokens; we surface it for cost visibility.
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

// Response is the result of a non-streaming Chat. Content is the
// concatenated assistant text; Reasoning is the optional CoT stream
// (kept separately so the UI can hide it behind a fold).
type Response struct {
	Content   string `json:"content"`
	Reasoning string `json:"reasoning,omitempty"`
	Model     string `json:"model"`
	Usage     Usage  `json:"usage"`
}

// StreamEvent is one token-ish delta emitted during StreamChat.
// Content / Reasoning carry incremental text (either may be empty);
// Done=true marks the final event, which also carries Usage.
type StreamEvent struct {
	Content   string `json:"content,omitempty"`
	Reasoning string `json:"reasoning,omitempty"`
	Done      bool   `json:"done"`
	Usage     *Usage `json:"usage,omitempty"`
	Err       error  `json:"-"`
}

// Sentinel errors. Callers can errors.Is() against these to render
// the right HTTP status (401 key invalid, 429 rate-limited, 402 out
// of credit, …). The underlying provider message is attached via
// Error wrapping so it still shows up in logs.
var (
	ErrNotConfigured = errors.New("ai: gateway not configured (set ai.base_url and ai.api_key)")
	ErrKeyInvalid    = errors.New("ai: api key rejected")
	ErrRateLimited   = errors.New("ai: rate limited")
	ErrInsufficient  = errors.New("ai: insufficient credit / quota")
	ErrTimeout       = errors.New("ai: request timed out")
	ErrModelNotFound = errors.New("ai: model not found")
	ErrUpstream      = errors.New("ai: upstream error")
)

// Gateway is the contract every AI provider implementation satisfies.
// Implementations must be safe for concurrent use.
type Gateway interface {
	// Chat performs a synchronous completion and returns the full
	// assistant reply. ctx cancellation propagates to the upstream
	// HTTP request. model may be "" to use the configured default.
	Chat(ctx context.Context, model string, msgs []Message, opts Options) (*Response, error)

	// StreamChat performs a streaming completion, pushing deltas to
	// onEvent as they arrive. The call blocks until the stream ends
	// (Done event) or ctx is cancelled. A non-nil error from any
	// event's Err field, or returned directly, should be treated as
	// terminal. onEvent is never called concurrently with itself.
	StreamChat(ctx context.Context, model string, msgs []Message, opts Options, onEvent func(StreamEvent)) error

	// ListModels returns the models the admin has configured, grouped
	// by tier (free / paid). The list comes from the ai.models
	// setting, not from a live /v1/models probe — the admin curates
	// which models users may pick.
	ListModels() ModelCatalog
}

// ModelTier labels a model's access class.
type ModelTier string

const (
	TierFree ModelTier = "free"
	TierPaid ModelTier = "paid"
)

// ModelEntry is one row of the ai.models setting JSON.
type ModelEntry struct {
	ID    string    `json:"id"`    // provider model id, e.g. "qwen3.7-plus"
	Label string    `json:"label"` // human-friendly, e.g. "通义千问 3.7 Plus"
	Tier  ModelTier `json:"tier"`  // free | paid
}

// ModelCatalog is the parsed ai.models setting, grouped for the UI.
type ModelCatalog struct {
	Free []ModelEntry `json:"free"`
	Paid []ModelEntry `json:"paid"`
}

// All flattens the catalog into a single slice (handy for validation).
func (c ModelCatalog) All() []ModelEntry {
	out := make([]ModelEntry, 0, len(c.Free)+len(c.Paid))
	out = append(out, c.Free...)
	out = append(out, c.Paid...)
	return out
}

// Find returns the entry for id, or ok=false. Used by the handler to
// validate the client's model pick and to enforce tier access.
func (c ModelCatalog) Find(id string) (ModelEntry, bool) {
	for _, m := range c.All() {
		if m.ID == id {
			return m, true
		}
	}
	return ModelEntry{}, false
}

// ResolveModel returns the model id to actually call: model if non-empty,
// else the configured default free model. The handler passes the
// client's choice through here so the gateway never has to guess.
func ResolveModel(g Gateway, model string) string {
	if model != "" {
		return model
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

// FromFortuneTier maps a fortune.PromptSpec.Tier ("free"/"paid") to a
// ModelTier. The strings line up today; the helper exists so a future
// divergence (e.g. a "demo" tier) only needs one fix.
func FromFortuneTier(t string) ModelTier {
	if t == string(TierPaid) {
		return TierPaid
	}
	return TierFree
}

// MessagesFromPrompt converts a fortune.PromptSpec into the []Message
// the gateway expects. This is the single seam between the fortune
// engines and the AI layer, so engines never import this package.
func MessagesFromPrompt(p *fortune.PromptSpec) []Message {
	return []Message{
		{Role: RoleSystem, Content: p.System},
		{Role: RoleUser, Content: p.User},
	}
}
