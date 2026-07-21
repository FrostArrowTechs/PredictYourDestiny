package ai

import (
	"errors"
	"net/http"
	"testing"
)

func TestValidateProviderBaseURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		ok   bool
	}{
		{"public HTTPS", "https://api.example.com/v1", true},
		{"HTTP", "http://api.example.com/v1", false},
		{"localhost", "https://localhost/v1", false},
		{"loopback", "https://127.0.0.1/v1", false},
		{"private IPv4", "https://10.0.0.1/v1", false},
		{"link local", "https://169.254.169.254/latest", false},
		{"private IPv6", "https://[fd00::1]/v1", false},
		{"credentials", "https://user:pass@api.example.com/v1", false},
		{"query", "https://api.example.com/v1?key=value", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProviderBaseURL(tt.url, false)
			if (err == nil) != tt.ok {
				t.Fatalf("ValidateProviderBaseURL(%q) = %v, ok=%v", tt.url, err, tt.ok)
			}
		})
	}
}

func TestProviderRedirectRejectsPrivateTarget(t *testing.T) {
	client := newProviderHTTPClient()
	req, err := http.NewRequest(http.MethodGet, "https://127.0.0.1/private", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = client.CheckRedirect(req, nil)
	if !errors.Is(err, ErrUnsafeProviderURL) {
		t.Fatalf("redirect error = %v, want ErrUnsafeProviderURL", err)
	}
}
