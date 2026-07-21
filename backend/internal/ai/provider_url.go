package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var ErrUnsafeProviderURL = errors.New("ai: unsafe provider URL")

// ValidateProviderBaseURL accepts only public HTTPS endpoints. The runtime
// transport repeats the address check after DNS resolution to prevent a
// hostname from resolving to loopback, link-local, or private infrastructure.
func ValidateProviderBaseURL(raw string, allowPrivate bool) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%w: invalid URL", ErrUnsafeProviderURL)
	}
	if u.Scheme != "https" && !(allowPrivate && u.Scheme == "http") {
		return fmt.Errorf("%w: HTTPS is required", ErrUnsafeProviderURL)
	}
	if u.User != nil {
		return fmt.Errorf("%w: embedded credentials are not allowed", ErrUnsafeProviderURL)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("%w: query and fragment are not allowed", ErrUnsafeProviderURL)
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("%w: localhost is not allowed", ErrUnsafeProviderURL)
	}
	if addr, err := netip.ParseAddr(host); err == nil && !allowPrivate && !isPublicProviderAddr(addr) {
		return fmt.Errorf("%w: private or special-use address is not allowed", ErrUnsafeProviderURL)
	}
	return nil
}

func isPublicProviderAddr(addr netip.Addr) bool {
	return addr.IsValid() && !addr.IsLoopback() && !addr.IsPrivate() &&
		!addr.IsLinkLocalUnicast() && !addr.IsLinkLocalMulticast() &&
		!addr.IsMulticast() && !addr.IsUnspecified()
}

func newProviderHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		MaxIdleConns:    20,
		IdleConnTimeout: 90 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("provider address: %w", err)
			}
			resolved, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			for _, addr := range resolved {
				if !isPublicProviderAddr(addr) {
					return nil, fmt.Errorf("%w: DNS resolved to %s", ErrUnsafeProviderURL, addr)
				}
			}
			if len(resolved) == 0 {
				return nil, fmt.Errorf("provider host resolved to no addresses")
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(resolved[0].String(), port))
		},
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many provider redirects")
			}
			return ValidateProviderBaseURL(req.URL.String(), false)
		},
	}
}

// CheckProviderHealth performs a cheap authenticated GET /models probe. It
// uses the same SSRF-safe transport as completions and never invokes a model.
func CheckProviderHealth(ctx context.Context, baseURL, apiKey string) (time.Duration, error) {
	if err := ValidateProviderBaseURL(baseURL, false); err != nil {
		return 0, err
	}
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return 0, err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/models"

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	started := time.Now()
	resp, err := newProviderHTTPClient().Do(req)
	duration := time.Since(started)
	if err != nil {
		return duration, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 32<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return duration, fmt.Errorf("provider health check returned HTTP %d", resp.StatusCode)
	}
	return duration, nil
}
