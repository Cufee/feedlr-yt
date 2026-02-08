package netproxy

import (
	"net/http"
	"testing"
)

func TestNewProxyPool_AllowsArbitraryProxyCount(t *testing.T) {
	raw := `
		http://10.0.0.1:8080,
		http://10.0.0.2:8080;
		http://10.0.0.3:8080
		http://10.0.0.4:8080
		http://10.0.0.5:8080
		http://10.0.0.6:8080
		http://10.0.0.7:8080
	`

	pool, err := NewProxyPool(raw)
	if err != nil {
		t.Fatalf("expected proxy pool to parse, got error: %v", err)
	}
	if pool == nil {
		t.Fatal("expected proxy pool")
	}
	if pool.Len() != 7 {
		t.Fatalf("expected 7 proxies, got %d", pool.Len())
	}
}

func TestProxyPool_RoundRobin(t *testing.T) {
	pool, err := NewProxyPool("http://a:1,http://b:2,http://c:3")
	if err != nil {
		t.Fatalf("build pool: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, "https://www.youtube.com", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	expectedHosts := []string{"a:1", "b:2", "c:3", "a:1", "b:2", "c:3"}
	for i, host := range expectedHosts {
		u, err := pool.Proxy(req)
		if err != nil {
			t.Fatalf("proxy selection %d failed: %v", i, err)
		}
		if u == nil {
			t.Fatalf("proxy selection %d returned nil", i)
		}
		if u.Host != host {
			t.Fatalf("proxy selection %d expected %s, got %s", i, host, u.Host)
		}
	}
}

func TestNewProxyPool_EmptyInput(t *testing.T) {
	pool, err := NewProxyPool(" \n\t ")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if pool != nil {
		t.Fatal("expected nil pool when proxy list is empty")
	}
}
