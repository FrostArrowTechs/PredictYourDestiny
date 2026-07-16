package model

import "time"

// AIProvider stores configuration for an OpenAI-compatible API endpoint.
// Admin can add multiple providers (e.g., OpenAI, DeepSeek, 通义千问)
// and switch between them at runtime.
type AIProvider struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"size:64;not null"`     // display name, e.g. "OpenAI", "DeepSeek"
	BaseURL   string    `gorm:"size:255;not null"`    // API base URL, e.g. "https://api.openai.com/v1"
	APIKey    string    `gorm:"size:255"`             // API key (encrypted in production)
	Models    string    `gorm:"type:text"`            // JSON array: [{"id":"gpt-4o","tier":"paid"},...]
	IsDefault bool      `gorm:"default:false"`        // only one provider can be default
	IsEnabled bool      `gorm:"default:true"`         // disabled providers are not used
	SortOrder int       `gorm:"default:0"`            // for UI ordering
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName overrides the default table name.
func (AIProvider) TableName() string {
	return "ai_providers"
}