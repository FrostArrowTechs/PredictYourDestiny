package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestIDMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(requestIDMiddleware())
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString("requestID"))
	})

	t.Run("preserves valid client id", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(requestIDHeader, "client-request-123")
		r.ServeHTTP(w, req)
		if got := w.Header().Get(requestIDHeader); got != "client-request-123" {
			t.Fatalf("response request ID = %q", got)
		}
		if w.Body.String() != "client-request-123" {
			t.Fatalf("context request ID = %q", w.Body.String())
		}
	})

	t.Run("replaces oversized id", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(requestIDHeader, strings.Repeat("x", 129))
		r.ServeHTTP(w, req)
		got := w.Header().Get(requestIDHeader)
		if got == "" || len(got) > 128 {
			t.Fatalf("generated request ID = %q", got)
		}
	})
}
