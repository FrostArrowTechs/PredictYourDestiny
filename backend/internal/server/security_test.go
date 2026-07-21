package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersAndProductionHSTS(t *testing.T) {
	router := New(Deps{Production: true, AllowedOrigins: []string{"https://admin.example.com"}})
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	for name, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := w.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if w.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("production response omitted HSTS")
	}
}

func TestCORSUsesExplicitAllowlist(t *testing.T) {
	router := New(Deps{AllowedOrigins: []string{"https://admin.example.com"}})
	request := func(origin string) http.Header {
		req := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", http.MethodGet)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Header()
	}
	if got := request("https://admin.example.com").Get("Access-Control-Allow-Origin"); got != "https://admin.example.com" {
		t.Fatalf("allowed origin header = %q", got)
	}
	if got := request("https://evil.example.com").Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("disallowed origin was accepted: %q", got)
	}
}
