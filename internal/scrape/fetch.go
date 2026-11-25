// Package scrape fetches web pages safely and extracts recipes from them.
package scrape

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/html/charset"
)

// ErrBlockedAddress is returned when a URL resolves to a private or otherwise
// internal address and private fetching is not allowed.
var ErrBlockedAddress = errors.New("that address points to a private network, which is blocked")

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// Fetcher downloads pages and images. With AllowPrivate false every outbound
// connection is checked at dial time, after DNS resolution, so redirects and
// DNS rebinding cannot be used to reach the local network (SSRF).
type Fetcher struct {
	client       *http.Client
	allowPrivate bool
}

func isPublicIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		switch {
		case ip4[0] == 0, ip4[0] >= 240: // "this network", reserved
			return false
		case ip4[0] == 100 && ip4[1]&0xc0 == 64: // CGNAT 100.64/10
			return false
		case ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 0: // IETF protocol assignments
			return false
		case ip4[0] == 198 && (ip4[1] == 18 || ip4[1] == 19): // benchmarking
			return false
		}
	}
	return true
}

// NewFetcher builds a fetcher. allowPrivate permits LAN addresses and is meant
// only for admin-supplied endpoints (a Mealie server, a local AI endpoint).
func NewFetcher(allowPrivate bool) *Fetcher {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	// With an outbound HTTP proxy configured the dialer only ever sees the
	// proxy's address, so the guard moves to vet(), which checks every host
	// (including each redirect) before a request is made.
	proxied := false
	if u, err := http.ProxyFromEnvironment(&http.Request{URL: &url.URL{Scheme: "https", Host: "example.com"}}); err == nil && u != nil {
		proxied = true
	}
	if !allowPrivate && !proxied {
		dialer.Control = func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if !isPublicIP(net.ParseIP(host)) {
				return ErrBlockedAddress
			}
			return nil
		}
	}
	tr := &http.Transport{
		Proxy:                  http.ProxyFromEnvironment,
		DialContext:            dialer.DialContext,
		ForceAttemptHTTP2:      true,
		TLSHandshakeTimeout:    10 * time.Second,
		ResponseHeaderTimeout:  20 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
		MaxIdleConns:           8,
		IdleConnTimeout:        60 * time.Second,
	}
	f := &Fetcher{allowPrivate: allowPrivate}
	f.client = &http.Client{
		Transport: tr,
		Timeout:   45 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 6 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return errors.New("redirect to unsupported scheme")
			}
			return f.vet(req.Context(), req.URL.Hostname())
		},
	}
	return f
}

// vet resolves a host up front and refuses internal addresses. The dial-time
// check above is the real guard; this one also covers deployments that send
// outbound traffic through an HTTP proxy, where the dialer only sees the proxy.
func (f *Fetcher) vet(ctx context.Context, host string) error {
	if f.allowPrivate {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return ErrBlockedAddress
		}
		return nil
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return ErrBlockedAddress
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil // let the request surface the DNS error
	}
	for _, ip := range ips {
		if !isPublicIP(ip.IP) {
			return ErrBlockedAddress
		}
	}
	return nil
}

// CleanURL validates a user-supplied URL and strips tracking noise.
func CleanURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("no link given")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("that doesn't look like a web link")
	}
	u.User = nil
	u.Fragment = ""
	q := u.Query()
	for k := range q {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "utm_") || lk == "fbclid" || lk == "gclid" || lk == "igshid" || lk == "mc_cid" || lk == "mc_eid" {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()
	return u, nil
}

func (f *Fetcher) get(ctx context.Context, target, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	if err := f.vet(ctx, req.URL.Hostname()); err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", accept)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	res, err := f.client.Do(req)
	if err != nil {
		if errors.Is(err, ErrBlockedAddress) {
			return nil, ErrBlockedAddress
		}
		return nil, err
	}
	return res, nil
}

// Page downloads an HTML page and returns it as UTF-8 along with the final URL.
func (f *Fetcher) Page(ctx context.Context, target string) ([]byte, string, error) {
	res, err := f.get(ctx, target, "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return nil, "", fmt.Errorf("the site answered with HTTP %d", res.StatusCode)
	}
	body := io.LimitReader(res.Body, 8<<20)
	rd, err := charset.NewReader(body, res.Header.Get("Content-Type"))
	if err != nil {
		rd = body
	}
	b, err := io.ReadAll(rd)
	if err != nil && len(b) == 0 {
		return nil, "", err
	}
	return b, res.Request.URL.String(), nil
}

// Bytes downloads a binary resource (an image) up to limit bytes.
func (f *Fetcher) Bytes(ctx context.Context, target string, limit int64) ([]byte, error) {
	res, err := f.get(ctx, target, "image/jpeg,image/png,image/webp,image/gif;q=0.9,*/*;q=0.5")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errors.New("file is too large")
	}
	return b, nil
}

// Do exposes the underlying client for JSON APIs (Mealie, AI endpoints).
func (f *Fetcher) Do(req *http.Request) (*http.Response, error) { return f.client.Do(req) }
