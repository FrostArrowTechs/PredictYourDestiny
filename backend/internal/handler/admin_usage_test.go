package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"predictdestiny/internal/model"
)

func TestAdminAIPricesAndUsageSummary(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin_ai_usage?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.AIProvider{}, &model.AIModelPriceVersion{}, &model.AIUsageLedger{}); err != nil {
		t.Fatal(err)
	}
	provider := model.AIProvider{Name: "Compatible", BaseURL: "https://api.example.com/v1", Models: `[{"id":"economy","tier":"free"}]`, IsEnabled: true}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatal(err)
	}
	h := &AdminAIUsageHandler{DB: db}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/prices", h.CreatePrice)
	router.GET("/prices", h.ListPrices)
	router.GET("/summary", h.UsageSummary)

	create := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/prices", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		return out
	}
	effective := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	body := `{"providerId":1,"model":"economy","version":"2026-07","inputCostMicrosPerMillion":1000000,"outputCostMicrosPerMillion":2000000,"reasoningCostMicrosPerMillion":500000,"effectiveFrom":"` + effective + `"}`
	if out := create(body); out.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", out.Code, out.Body.String())
	}
	if out := create(body); out.Code != http.StatusConflict {
		t.Fatalf("duplicate status=%d body=%s", out.Code, out.Body.String())
	}
	unknown := `{"providerId":1,"model":"typo","version":"v1","effectiveFrom":"` + effective + `"}`
	if out := create(unknown); out.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown model status=%d body=%s", out.Code, out.Body.String())
	}
	negative := `{"providerId":1,"model":"economy","version":"negative","inputCostMicrosPerMillion":-1,"effectiveFrom":"` + effective + `"}`
	if out := create(negative); out.Code != http.StatusUnprocessableEntity {
		t.Fatalf("negative price status=%d body=%s", out.Code, out.Body.String())
	}

	priceID := uint(1)
	actual := int64(4)
	now := time.Now().UTC()
	rows := []model.AIUsageLedger{
		{UserID: 1, ProviderID: &provider.ID, ProviderName: provider.Name, Model: "economy", RequestID: "r1", Status: "succeeded", PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30, PriceVersionID: &priceID, PricingStatus: "priced", EstimatedCostMicros: 3, ActualCostMicros: &actual, StartedAt: now, CompletedAt: now},
		{UserID: 1, ProviderID: &provider.ID, ProviderName: provider.Name, Model: "economy", RequestID: "r2", Status: "failed", PricingStatus: "unpriced", StartedAt: now, CompletedAt: now},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/summary?from="+now.Format("2006-01-02")+"&to="+now.Format("2006-01-02"), nil))
	if out.Code != http.StatusOK {
		t.Fatalf("summary status=%d body=%s", out.Code, out.Body.String())
	}
	var result struct {
		Totals usageTotals  `json:"totals"`
		Groups []usageGroup `json:"groups"`
	}
	if err := json.Unmarshal(out.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Totals.Requests != 2 || result.Totals.Unpriced != 1 || result.Totals.TotalTokens != 30 ||
		result.Totals.EstimatedCostMicros != 3 || result.Totals.ActualCostMicros != 4 || result.Totals.ActualCostCount != 1 ||
		len(result.Groups) != 1 {
		t.Fatalf("unexpected summary: %+v groups=%+v", result.Totals, result.Groups)
	}
}
