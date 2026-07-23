// Package fortune hosts the calculation engines for every divination
// system the product supports (bazi, zodiac, huangli, dream,
// compatibility, constellation, tarot, …).
//
// Each engine implements FortuneEngine, a tiny contract with three
// methods:
//
//	Name        — stable identifier, also the FortuneRecord.Kind value
//	Compute     — pure function: structured input → structured result
//	BuildPrompt — turn a result into the AI prompt payload
//
// Engines are intentionally framework-free: they take plain structs,
// return plain structs, and never touch *gin.Context or *gorm.DB.
// That keeps them trivially testable (the bazi engine, for example,
// is verified against published reference charts in bazi_test.go)
// and lets the handler layer compose them with caching, auth and
// quota checks without the engine knowing about any of it.
//
// The same Compute → BuildPrompt → AI gateway pipeline is reused for
// every system, so a new fortune module is mostly a matter of writing
// a Compute implementation plus a prompt template.
package fortune

// Kind enumerates the engine identifiers. These double as the
// FortuneRecord.Kind column and the route segment under /api/<kind>.
const (
	KindBazi          = "bazi"
	KindZodiac        = "zodiac"
	KindHuangli       = "huangli"
	KindDream         = "dream"
	KindCompatibility = "compatibility"
	KindWeighbone     = "weighbone"
	KindDivination    = "divination"
	KindPlumFlower    = "plumflower"
	KindConstellation = "constellation"
	KindTarot         = "tarot"
	KindName          = "name"
	KindAstrology     = "astrology"
	KindZiwei         = "ziwei"
)

// Gender mirrors the convention used by lunar-go: 1 = male, 0 = female.
type Gender int

const (
	GenderFemale Gender = 0
	GenderMale   Gender = 1
)

// String returns the Chinese label for the gender.
func (g Gender) String() string {
	if g == GenderMale {
		return "男"
	}
	return "女"
}

// Input is the union input for any engine. Engines read only the
// fields relevant to them; the rest are ignored. Keeping a single
// struct avoids a per-engine request envelope while still letting the
// handler validate once.
type Input struct {
	Birth *BirthContext `json:"birthContext,omitempty"`
	// Birth date / time of the subject (solar / Gregorian).
	Year   int `json:"year"`
	Month  int `json:"month"`
	Day    int `json:"day"`
	Hour   int `json:"hour"`   // 0-23
	Minute int `json:"minute"` // 0-59

	// Gender of the subject (used by bazi da-yun direction).
	Gender Gender `json:"gender"`
	// ZiweiLeapMonthRule is required only when the converted lunar date falls
	// in a leap month. It prevents silently choosing between competing schools.
	ZiweiLeapMonthRule string `json:"ziweiLeapMonthRule,omitempty"`

	// Longitude of the birth place in degrees (east positive).
	// Used for true-solar-time correction. When 0, no correction is
	// applied and the input time is treated as Beijing time (UTC+8),
	// which is lunar-go's native assumption.
	Longitude float64 `json:"longitude"`

	// Lang is the user's preferred response language (zh-CN / zh-TW /
	// en-US …) so prompt builders can localize AI output. Engines that
	// emit Chinese-only structured data may ignore it.
	Lang string `json:"lang"`

	// Free-form question for engines that take one (tarot, dream).
	Question string `json:"question,omitempty"`

	// Structured name input. FullName/Question remains a legacy compatibility
	// path; new callers should provide surname and givenName explicitly.
	Surname          string `json:"surname,omitempty"`
	GivenName        string `json:"givenName,omitempty"`
	SurnameConfirmed bool   `json:"surnameConfirmed,omitempty"`
	Script           string `json:"script,omitempty"`
	StrokeStandard   string `json:"strokeStandard,omitempty"`

	// InterpretDepth hints the prompt builder at the desired detail
	// level: "brief" (free tier) or "deep" (paid tier). Engines ignore
	// it; only BuildPrompt consumes it.
	InterpretDepth string `json:"interpretDepth,omitempty"`

	// Second subject for compatibility engines.
	Second *Input `json:"second,omitempty"`
}

// Result is the structured, AI-free output of an engine. The Kind
// field lets a generic caller (e.g. the history viewer) dispatch on
// type without a Go type switch. The engine-specific payload lives in
// Data as a typed struct stored under the "chart" key — see each
// engine's package for the exact shape.
//
// Meta carries lightweight provenance: the lunar/solar dates that
// produced the chart, the true-solar-time offset applied, etc. It is
// flat string→string so it serializes cleanly and is easy to display.
type Result struct {
	Kind           string            `json:"kind"`
	Data           any               `json:"data"`
	Meta           map[string]string `json:"meta,omitempty"`
	ResultMetadata *ResultMetadata   `json:"resultMetadata,omitempty"`
}

// PromptSpec is the payload an engine emits to ask the AI gateway for
// an interpretation. System sets the persona and output rules; User
// injects the structured chart. RecommendedModel is a hint — the
// handler may override it based on quota / tier.
//
// Tier recommends a model class: "free" or "paid". Engines set this
// from InterpretDepth so the gateway can enforce access control.
type PromptSpec struct {
	System           string `json:"system"`
	User             string `json:"user"`
	Tier             string `json:"tier"` // free|paid
	RecommendedModel string `json:"recommendedModel,omitempty"`
}

const (
	TierFree = "free"
	TierPaid = "paid"
)

// PromptDepth values for Input.InterpretDepth.
const (
	DepthBrief = "brief"
	DepthDeep  = "deep"
)

// FortuneEngine is the contract every divination system implements.
// Implementations must be safe for concurrent use (Compute is called
// from HTTP handlers); they should be stateless or hold only
// read-only references.
//
// Note: BuildPrompt is intentionally NOT on this interface. Prompt
// templates live in internal/ai/prompt (a package that imports this
// one for the chart types), so having engines call back into the
// prompt package would create an import cycle. Instead the handler
// layer orchestrates: engine.Compute → prompt.<Kind>Build → gateway.
// That keeps the calc engines free of any AI dependency and the
// prompt templates in exactly one place (no duplicated phrasing).
type FortuneEngine interface {
	// Name returns the engine's stable identifier (one of the Kind*
	// constants). Used as FortuneRecord.Kind and as a route segment.
	Name() string

	// Compute turns a validated Input into a structured Result. It
	// must be deterministic and side-effect free. Errors only arise
	// from genuinely bad input (out-of-range dates, missing second
	// subject for compatibility, …); a 200-result never carries an
	// error.
	Compute(in Input) (*Result, error)
}

// registry holds engines keyed by Name(). Populated by init() in each
// engine file. Fortune() looks one up by kind — the handler uses this
// to dispatch /api/<kind>/compute without a switch.
var registry = map[string]FortuneEngine{}

// Register adds an engine to the registry. Called from each engine
// package's init(). Panics on duplicate registration, which would
// indicate a programming error rather than a runtime condition.
func Register(e FortuneEngine) {
	name := e.Name()
	if _, dup := registry[name]; dup {
		panic("fortune: duplicate engine registration: " + name)
	}
	registry[name] = e
}

// Fortune returns the engine for a kind, or ok=false if none is
// registered. Handlers use this to dispatch by URL segment.
func Fortune(kind string) (FortuneEngine, bool) {
	e, ok := registry[kind]
	return e, ok
}

// Engines returns every registered engine, useful for diagnostics and
// for an eventual /api/engines meta endpoint.
func Engines() []FortuneEngine {
	out := make([]FortuneEngine, 0, len(registry))
	for _, e := range registry {
		out = append(out, e)
	}
	return out
}
