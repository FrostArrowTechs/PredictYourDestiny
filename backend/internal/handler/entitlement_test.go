package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"predictdestiny/internal/ai"
	"predictdestiny/internal/auth"
	"predictdestiny/internal/model"
)

func TestEntitlementResponseAndExpiredFallback(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:entitlement_response?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.MembershipTier{}, &model.UserMembership{}); err != nil {
		t.Fatal(err)
	}
	free := model.MembershipTier{Code: model.TierCodeFree, Name: "Free", DailyQuota: 5, Features: `["basic_interpret"]`}
	basic := model.MembershipTier{Code: model.TierCodeBasic, Name: "Basic", DailyQuota: 20, Features: `["basic_interpret","deep_interpret"]`}
	if err := db.Create(&free).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&basic).Error; err != nil {
		t.Fatal(err)
	}
	expired := time.Now().Add(-time.Hour)
	membership := model.UserMembership{UserID: 10, TierID: basic.ID, ExpiresAt: &expired}
	if err := db.Create(&membership).Error; err != nil {
		t.Fatal(err)
	}

	gateway := &fakeGateway{catalog: ai.ModelCatalog{
		Free: []ai.ModelEntry{{ID: "free-model", Tier: ai.TierFree}},
		Paid: []ai.ModelEntry{{ID: "paid-model", Tier: ai.TierPaid}},
	}}
	handler := &EntitlementHandler{DB: db, Gateway: gateway}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(auth.ClaimsKey), &auth.Claims{UserID: 10, Role: "user"})
		c.Next()
	})
	router.GET("/entitlements", handler.Get)

	get := func() EntitlementResponse {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/entitlements", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
		}
		var response EntitlementResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response
	}

	fallback := get()
	if fallback.EffectiveTier != model.TierCodeFree || !fallback.FellBackToFree {
		t.Fatalf("expired response = %+v", fallback)
	}
	if len(fallback.AvailableModels) != 1 || fallback.AvailableModels[0].ID != "free-model" {
		t.Fatalf("free models = %+v", fallback.AvailableModels)
	}

	future := time.Now().Add(time.Hour)
	if err := db.Model(&membership).Updates(map[string]any{"tier_id": basic.ID, "expires_at": future}).Error; err != nil {
		t.Fatal(err)
	}
	active := get()
	if active.EffectiveTier != model.TierCodeBasic || active.FellBackToFree {
		t.Fatalf("active response = %+v", active)
	}
	if len(active.AvailableModels) != 2 {
		t.Fatalf("paid models = %+v", active.AvailableModels)
	}
	if len(active.Features) != 2 || active.ExpiresAt == nil {
		t.Fatalf("active features/expiry = %+v", active)
	}
}
