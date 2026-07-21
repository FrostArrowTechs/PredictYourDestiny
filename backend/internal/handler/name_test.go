package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"predictdestiny/internal/model"
)

func TestNameComputeReturns422AndUnknownCharacters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CharacterStroke{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.CharacterStroke{Char: "张", Strokes: 11}).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	handler := NameHandler{DB: db}
	router.POST("/api/name/compute", handler.Compute)
	req := httptest.NewRequest(http.MethodPost, "/api/name/compute", strings.NewReader(`{"fullName":"张罕"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity || !strings.Contains(resp.Body.String(), `"unknownCharacters":["罕"]`) {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
}
