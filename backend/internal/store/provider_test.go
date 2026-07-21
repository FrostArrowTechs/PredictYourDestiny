package store

import (
	"encoding/base64"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"predictdestiny/internal/model"
	"predictdestiny/internal/secret"
)

func providerTestCipher(t *testing.T) *secret.Cipher {
	t.Helper()
	cipher, err := secret.New(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	return cipher
}

func TestOnlyOneDefaultProvider(t *testing.T) {
	db := providerTestDB(t, "provider_unique_default")
	first := model.AIProvider{Name: "first", BaseURL: "https://first.example/v1", IsDefault: true, IsEnabled: true}
	second := model.AIProvider{Name: "second", BaseURL: "https://second.example/v1", IsDefault: true, IsEnabled: true}
	if err := db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&second).Error; err == nil || !strings.Contains(strings.ToLower(err.Error()), "unique") {
		t.Fatalf("second default error = %v, want unique constraint", err)
	}

	nonDefault := model.AIProvider{Name: "fallback", BaseURL: "https://fallback.example/v1", IsEnabled: true}
	if err := db.Create(&nonDefault).Error; err != nil {
		t.Fatalf("non-default provider was rejected: %v", err)
	}
}

func providerTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Setting{}, &model.AIProvider{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestProviderStoreFollowsEnabledDefault(t *testing.T) {
	db := providerTestDB(t, "provider_runtime")
	cipher := providerTestCipher(t)
	firstKey, _ := cipher.Encrypt("first-secret")
	secondKey, _ := cipher.Encrypt("second-secret")
	first := model.AIProvider{Name: "first", BaseURL: "https://first.example/v1", APIKey: firstKey, Models: `[{"id":"free-a","tier":"free"}]`, IsDefault: true, IsEnabled: true}
	second := model.AIProvider{Name: "second", BaseURL: "https://second.example/v1", APIKey: secondKey, Models: `[{"id":"paid-b","tier":"paid"}]`, IsEnabled: true}
	if err := db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	store := NewProviderStore(db, cipher)
	if got, _ := store.Get(model.SettingAIBaseURL); got != first.BaseURL {
		t.Fatalf("initial base URL = %q", got)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.AIProvider{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&second).Update("is_default", true).Error
	}); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.Get(model.SettingAIBaseURL); got != second.BaseURL {
		t.Fatalf("switched base URL = %q", got)
	}

	if err := db.Model(&second).Update("is_enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Get(model.SettingAIBaseURL); ok {
		t.Fatal("disabled default provider remained available")
	}
}

func TestMigrateLegacyAIProviderOnce(t *testing.T) {
	db := providerTestDB(t, "provider_migration")
	settings, err := NewSettingStore(db, []model.Setting{
		{Key: model.SettingAIBaseURL, Value: "https://legacy.example/v1"},
		{Key: model.SettingAIAPIKey, Value: "legacy-secret", Kind: model.SettingKindPassword},
		{Key: model.SettingAIModels, Value: `[{"id":"legacy-model","tier":"free"}]`},
	})
	if err != nil {
		t.Fatal(err)
	}
	cipher := providerTestCipher(t)
	migrated, err := MigrateLegacyAIProvider(db, settings, cipher)
	if err != nil || !migrated {
		t.Fatalf("first migration: migrated=%v err=%v", migrated, err)
	}
	migrated, err = MigrateLegacyAIProvider(db, settings, cipher)
	if err != nil || migrated {
		t.Fatalf("second migration: migrated=%v err=%v", migrated, err)
	}
	var providers int64
	if err := db.Model(&model.AIProvider{}).Count(&providers).Error; err != nil {
		t.Fatal(err)
	}
	if providers != 1 {
		t.Fatalf("provider rows = %d, want 1", providers)
	}
	var provider model.AIProvider
	if err := db.First(&provider).Error; err != nil {
		t.Fatal(err)
	}
	if provider.APIKey == "legacy-secret" || !secret.IsEncrypted(provider.APIKey) {
		t.Fatal("legacy API key was not encrypted")
	}
	if legacy, ok := settings.Get(model.SettingAIAPIKey); ok && legacy != "" {
		t.Fatal("legacy plaintext API key was not cleared")
	}
}

func TestEncryptPlaintextProviderKeys(t *testing.T) {
	db := providerTestDB(t, "provider_key_migration")
	provider := model.AIProvider{Name: "legacy", BaseURL: "https://api.example.com/v1", APIKey: "plaintext", IsEnabled: true}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatal(err)
	}
	cipher := providerTestCipher(t)
	count, err := EncryptPlaintextProviderKeys(db, cipher)
	if err != nil || count != 1 {
		t.Fatalf("EncryptPlaintextProviderKeys() = %d, %v", count, err)
	}
	if err := db.First(&provider, provider.ID).Error; err != nil {
		t.Fatal(err)
	}
	plain, err := cipher.Decrypt(provider.APIKey)
	if err != nil || plain != "plaintext" {
		t.Fatalf("migrated key = %q, %v", plain, err)
	}
}
