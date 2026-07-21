package store

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"predictdestiny/internal/model"
	"predictdestiny/internal/secret"
)

// ProviderStore adapts the enabled default AIProvider to the narrow settings
// interface consumed by the OpenAI-compatible gateway. Every read observes the
// current database state, so an admin switch applies without a process restart.
type ProviderStore struct {
	db     *gorm.DB
	cipher *secret.Cipher
}

func NewProviderStore(db *gorm.DB, cipher *secret.Cipher) *ProviderStore {
	return &ProviderStore{db: db, cipher: cipher}
}

func (s *ProviderStore) current() (model.AIProvider, bool) {
	var provider model.AIProvider
	err := s.db.Where("is_default = ? AND is_enabled = ?", true, true).First(&provider).Error
	return provider, err == nil
}

func (s *ProviderStore) Get(key string) (string, bool) {
	provider, ok := s.current()
	if !ok {
		return "", false
	}
	switch key {
	case model.SettingAIBaseURL:
		return provider.BaseURL, strings.TrimSpace(provider.BaseURL) != ""
	case model.SettingAIAPIKey:
		value, err := s.cipher.Decrypt(provider.APIKey)
		return value, err == nil && strings.TrimSpace(value) != ""
	case model.SettingAIModels:
		return provider.Models, strings.TrimSpace(provider.Models) != ""
	case model.SettingAIDefaultModel:
		return "", false
	default:
		return "", false
	}
}

func (s *ProviderStore) GetDefault(key, fallback string) string {
	if value, ok := s.Get(key); ok && value != "" {
		return value
	}
	return fallback
}

// MigrateLegacyAIProvider imports the old settings-backed gateway only when no
// provider rows exist. It never overwrites provider management data.
func MigrateLegacyAIProvider(db *gorm.DB, settings *SettingStore, cipher *secret.Cipher) (bool, error) {
	var count int64
	if err := db.Model(&model.AIProvider{}).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	baseURL, _ := settings.Get(model.SettingAIBaseURL)
	if strings.TrimSpace(baseURL) == "" {
		return false, nil
	}
	apiKey, _ := settings.Get(model.SettingAIAPIKey)
	if apiKey != "" {
		var err error
		apiKey, err = cipher.Encrypt(apiKey)
		if err != nil {
			return false, fmt.Errorf("encrypt legacy provider key: %w", err)
		}
	}
	models, _ := settings.Get(model.SettingAIModels)
	provider := model.AIProvider{
		Name:      "Migrated legacy provider",
		BaseURL:   baseURL,
		APIKey:    apiKey,
		Models:    models,
		IsDefault: true,
		IsEnabled: true,
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&provider).Error; err != nil {
			return err
		}
		return tx.Model(&model.Setting{}).
			Where("key = ?", model.SettingAIAPIKey).
			Update("value", "").Error
	}); err != nil {
		return false, err
	}
	if err := settings.Reload(); err != nil {
		return false, err
	}
	return true, nil
}

// EncryptPlaintextProviderKeys upgrades legacy rows in place. It deliberately
// refuses to run without a configured key rather than leaving plaintext usable.
func EncryptPlaintextProviderKeys(db *gorm.DB, cipher *secret.Cipher) (int, error) {
	var providers []model.AIProvider
	if err := db.Where("api_key <> ?", "").Find(&providers).Error; err != nil {
		return 0, err
	}
	updated := 0
	for _, provider := range providers {
		if secret.IsEncrypted(provider.APIKey) {
			if _, err := cipher.Decrypt(provider.APIKey); err != nil {
				return updated, fmt.Errorf("validate provider %d key: %w", provider.ID, err)
			}
			continue
		}
		encrypted, err := cipher.Encrypt(provider.APIKey)
		if err != nil {
			return updated, fmt.Errorf("encrypt provider %d key: %w", provider.ID, err)
		}
		if err := db.Model(&model.AIProvider{}).Where("id = ? AND api_key = ?", provider.ID, provider.APIKey).Update("api_key", encrypted).Error; err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}
