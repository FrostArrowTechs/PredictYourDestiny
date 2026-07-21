package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"predictdestiny/internal/model"
	"predictdestiny/internal/secret"
)

func TestAdminProviderEncryptsAndMasksAPIKey(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin_provider_secret?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.AIProvider{}); err != nil {
		t.Fatal(err)
	}
	cipher, err := secret.New(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	h := &AdminProviderHandler{DB: db, Cipher: cipher}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/providers", h.CreateProvider)
	r.GET("/providers", h.ListProviders)
	r.PUT("/providers/:id", h.UpdateProvider)

	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	created := request(http.MethodPost, "/providers", `{"name":"test","baseUrl":"https://api.example.com/v1","apiKey":"plain-secret","models":"[]","isEnabled":true}`)
	if created.Code != http.StatusCreated || bytes.Contains(created.Body.Bytes(), []byte("plain-secret")) {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var provider model.AIProvider
	if err := db.First(&provider).Error; err != nil {
		t.Fatal(err)
	}
	if !secret.IsEncrypted(provider.APIKey) {
		t.Fatalf("stored key is not encrypted: %q", provider.APIKey)
	}
	original := provider.APIKey

	listed := request(http.MethodGet, "/providers", "")
	var payload struct {
		Providers []ProviderPayload `json:"providers"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Providers) != 1 || payload.Providers[0].APIKey != maskedSecret {
		t.Fatalf("list leaked or omitted mask: %s", listed.Body.String())
	}

	updated := request(http.MethodPut, "/providers/1", `{"apiKey":"********"}`)
	if updated.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", updated.Code, updated.Body.String())
	}
	if err := db.First(&provider, provider.ID).Error; err != nil {
		t.Fatal(err)
	}
	if provider.APIKey != original {
		t.Fatal("masked update replaced the encrypted key")
	}
}

func TestAdminProviderRejectsKeyWithoutEncryptionConfig(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin_provider_no_key?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.AIProvider{}); err != nil {
		t.Fatal(err)
	}
	h := &AdminProviderHandler{DB: db}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/providers", bytes.NewBufferString(`{"name":"test","baseUrl":"https://api.example.com/v1","apiKey":"secret","isEnabled":true}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.CreateProvider(c)
	if c.Writer.Status() != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", c.Writer.Status())
	}
}
