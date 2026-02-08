package netproxy

import (
	stdErrors "errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/pkg/errors"
)

const youtubeProxyURLsEnv = "YOUTUBE_PROXY_URLS"
const youtubeProxyScope = "youtube"

type ProxyPool struct {
	proxies []*url.URL
	next    atomic.Uint64
}

func NewProxyPool(raw string) (*ProxyPool, error) {
	values := splitProxyList(raw)
	if len(values) == 0 {
		metrics.ObserveProxyEvent(youtubeProxyScope, "config_empty", nil)
		return nil, nil
	}

	proxies := make([]*url.URL, 0, len(values))
	for _, value := range values {
		parsed, err := url.Parse(value)
		if err != nil {
			metrics.ObserveProxyEvent(youtubeProxyScope, "config_invalid", err)
			metrics.ObserveProxyError(youtubeProxyScope, "config_parse", "invalid_url")
			return nil, errors.Wrapf(err, "invalid proxy url %q", value)
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			cfgErr := errors.Errorf("invalid proxy url %q", value)
			metrics.ObserveProxyEvent(youtubeProxyScope, "config_invalid", cfgErr)
			metrics.ObserveProxyError(youtubeProxyScope, "config_parse", "invalid_url")
			return nil, errors.Errorf("invalid proxy url %q", value)
		}
		switch parsed.Scheme {
		case "http", "https", "socks5", "socks5h":
		default:
			cfgErr := errors.Errorf("unsupported proxy scheme %q", parsed.Scheme)
			metrics.ObserveProxyEvent(youtubeProxyScope, "config_invalid", cfgErr)
			metrics.ObserveProxyError(youtubeProxyScope, "config_parse", "unsupported_scheme")
			return nil, errors.Errorf("unsupported proxy scheme %q", parsed.Scheme)
		}
		proxies = append(proxies, parsed)
	}

	metrics.ObserveProxyEvent(youtubeProxyScope, "config_loaded", nil)
	return &ProxyPool{proxies: proxies}, nil
}

func (p *ProxyPool) Len() int {
	if p == nil {
		return 0
	}
	return len(p.proxies)
}

func (p *ProxyPool) Proxy(_ *http.Request) (*url.URL, error) {
	if p == nil || len(p.proxies) == 0 {
		metrics.ObserveProxyEvent(youtubeProxyScope, "select_direct", nil)
		return nil, nil
	}

	idx := p.next.Add(1) - 1
	metrics.ObserveProxyEvent(youtubeProxyScope, "select_proxy", nil)
	return p.proxies[idx%uint64(len(p.proxies))], nil
}

func splitProxyList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || unicode.IsSpace(r)
	})

	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

var (
	youtubeTransportOnce sync.Once
	youtubeTransport     *http.Transport
	youtubeTransportErr  error
)

func NewYouTubeHTTPClient(timeout time.Duration) (*http.Client, error) {
	transport, err := getYouTubeTransport()
	if err != nil {
		metrics.ObserveProxyEvent(youtubeProxyScope, "transport_init", err)
		return nil, err
	}
	metrics.ObserveProxyEvent(youtubeProxyScope, "transport_init", nil)

	httpClient := &http.Client{
		Transport: &observedRoundTripper{
			base:         transport,
			scope:        youtubeProxyScope,
			proxyEnabled: transport.Proxy != nil,
		},
		Timeout: timeout,
	}
	return httpClient, nil
}

func getYouTubeTransport() (*http.Transport, error) {
	youtubeTransportOnce.Do(func() {
		base, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			youtubeTransportErr = errors.New("default http transport has unexpected type")
			metrics.ObserveProxyError(youtubeProxyScope, "transport_init", "invalid_default_transport")
			return
		}

		transport := base.Clone()

		pool, err := NewProxyPool(os.Getenv(youtubeProxyURLsEnv))
		if err != nil {
			youtubeTransportErr = errors.Wrap(err, "failed to build youtube proxy pool")
			metrics.ObserveProxyError(youtubeProxyScope, "transport_init", "pool_build_failed")
			return
		}

		if pool != nil && pool.Len() > 0 {
			transport.Proxy = pool.Proxy
			transport.MaxIdleConns = 128
			transport.MaxIdleConnsPerHost = 32
		}

		youtubeTransport = transport
	})

	return youtubeTransport, youtubeTransportErr
}

type observedRoundTripper struct {
	base         http.RoundTripper
	scope        string
	proxyEnabled bool
}

func (rt *observedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := rt.base.RoundTrip(req)
	if err == nil || !rt.proxyEnabled {
		return resp, err
	}

	metrics.ObserveProxyError(rt.scope, "request", classifyRequestError(err))
	return resp, err
}

func classifyRequestError(err error) string {
	if err == nil {
		return "unknown"
	}

	var netErr net.Error
	if stdErrors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}

	var dnsErr *net.DNSError
	if stdErrors.As(err, &dnsErr) {
		return "dns"
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "proxyconnect"),
		strings.Contains(msg, "proxy"),
		strings.Contains(msg, "http: server gave http response to https client"):
		return "proxy_connect"
	case strings.Contains(msg, "407"),
		strings.Contains(msg, "authentication"):
		return "auth"
	case strings.Contains(msg, "connection refused"):
		return "refused"
	case strings.Contains(msg, "connection reset"):
		return "reset"
	case strings.Contains(msg, "no route to host"):
		return "unreachable"
	default:
		return "other"
	}
}
