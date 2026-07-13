package model

import "time"

// Setting stores runtime-mutable configuration as key/value pairs.
//
// This is the dynamic-config layer: AI provider base URL & key,
// available model lists, rate limits, feature flags, etc. are all
// stored here so the admin can change them from the dashboard without
// editing .env or restarting the process.
//
// Each value carries a Kind so the UI can render the right editor
// (text / password / number / bool / json) and a Group for tabbing.
type Setting struct {
	ID        uint      `gorm:"primaryKey"`
	Key       string    `gorm:"uniqueIndex;size:128;not null"`
	Value     string    `gorm:"type:text"`
	Kind      string    `gorm:"size:32;default:'text'"`  // text|password|number|bool|json
	Group     string    `gorm:"size:64;index;default:'general'"`
	Label     string    `gorm:"size:128"`                 // human-friendly name (i18n key)
	Hint      string    `gorm:"type:text"`                // help text
	SortOrder int       `gorm:"default:0"`
	UpdatedAt time.Time
}

const (
	SettingGroupAI      = "ai"       // AI gateway: base url, keys, model list
	SettingGroupFeature = "feature"  // feature toggles
	SettingGroupGeneral = "general"  // site name, announcement, etc.
)

const (
	SettingKindText     = "text"
	SettingKindPassword = "password"
	SettingKindNumber   = "number"
	SettingKindBool     = "bool"
	SettingKindJSON     = "json"
)

// Known setting keys. Declared as constants so typos are caught by
// the compiler and readers can grep for usage sites.
const (
	// AI gateway — points at a New-API (OpenAI-compatible) instance.
	SettingAIBaseURL    = "ai.base_url"     // e.g. https://newapi.example.com/v1
	SettingAIAPIKey     = "ai.api_key"      // secret, rendered as password
	SettingAIModels     = "ai.models"       // JSON: [{"id":"deepseek-chat","tier":"free"}, ...]
	SettingAIDefaultModel = "ai.default_model"

	// Rate limits.
	SettingDailyFreeQuota = "quota.daily_free" // number
)
