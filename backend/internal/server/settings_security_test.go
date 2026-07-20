package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"predictdestiny/internal/auth"
	"predictdestiny/internal/model"
	"predictdestiny/internal/store"
)

func TestSettingsRouteRequiresAdminAndMasksSecrets(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:settings_security?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Setting{}); err != nil {
		t.Fatal(err)
	}
	settings, err := store.NewSettingStore(db, []model.Setting{
		{Key: model.SettingAIAPIKey, Value: "sk-sensitive", Kind: model.SettingKindPassword},
	})
	if err != nil {
		t.Fatal(err)
	}

	auth.InitJWT("settings-security-test-secret")
	userToken, err := auth.SignToken(1, "user@example.com", "user")
	if err != nil {
		t.Fatal(err)
	}
	adminToken, err := auth.SignToken(2, "admin@example.com", "admin")
	if err != nil {
		t.Fatal(err)
	}

	router := New(Deps{DB: db, Settings: settings})
	request := func(method, body, token string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/settings", strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		router.ServeHTTP(w, req)
		return w
	}

	if got := request(http.MethodGet, "", "").Code; got != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want 401", got)
	}
	if got := request(http.MethodGet, "", userToken).Code; got != http.StatusForbidden {
		t.Fatalf("ordinary user status = %d, want 403", got)
	}

	adminResponse := request(http.MethodGet, "", adminToken)
	if adminResponse.Code != http.StatusOK {
		t.Fatalf("admin status = %d, body = %s", adminResponse.Code, adminResponse.Body.String())
	}
	var payload struct {
		Items []model.Setting `json:"items"`
	}
	if err := json.Unmarshal(adminResponse.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) != 1 || payload.Items[0].Value != "********" {
		t.Fatalf("masked settings = %+v", payload.Items)
	}
	if strings.Contains(adminResponse.Body.String(), "sk-sensitive") {
		t.Fatal("admin response leaked the stored secret")
	}

	maskedUpdate := request(http.MethodPut, `{"items":{"ai.api_key":"********"}}`, adminToken)
	if maskedUpdate.Code != http.StatusOK {
		t.Fatalf("masked update status = %d, body = %s", maskedUpdate.Code, maskedUpdate.Body.String())
	}
	if value, _ := settings.Get(model.SettingAIAPIKey); value != "sk-sensitive" {
		t.Fatalf("masked update overwrote secret with %q", value)
	}
}
