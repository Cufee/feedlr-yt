package youtube

import (
	"net/http"
	"testing"
)

func TestIsShortsURL_HeadStatusOK(t *testing.T) {
	calls := 0
	c := &client{
		http: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.Method != http.MethodHead {
					t.Fatalf("expected HEAD request, got %s", req.Method)
				}
				return mockResponse(http.StatusOK), nil
			}),
		},
	}

	if !c.isShortsURL("video-id") {
		t.Fatal("expected short to be detected")
	}
	if calls != 1 {
		t.Fatalf("expected one request, got %d", calls)
	}
}

func TestIsShortsURL_FallsBackToGetWhenHeadIsUnsupported(t *testing.T) {
	calls := 0
	c := &client{
		http: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				switch req.Method {
				case http.MethodHead:
					return mockResponse(http.StatusMethodNotAllowed), nil
				case http.MethodGet:
					if req.Header.Get("Range") != "bytes=0-0" {
						t.Fatalf("expected range header, got %q", req.Header.Get("Range"))
					}
					return mockResponse(http.StatusOK), nil
				default:
					t.Fatalf("unexpected method: %s", req.Method)
				}
				return nil, nil
			}),
		},
	}

	if !c.isShortsURL("video-id") {
		t.Fatal("expected short to be detected via GET fallback")
	}
	if calls != 2 {
		t.Fatalf("expected two requests, got %d", calls)
	}
}

func TestIsShortsURL_HeadStatusSeeOther(t *testing.T) {
	calls := 0
	c := &client{
		http: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.Method != http.MethodHead {
					t.Fatalf("expected HEAD request, got %s", req.Method)
				}
				return mockResponse(http.StatusSeeOther), nil
			}),
		},
	}

	if c.isShortsURL("video-id") {
		t.Fatal("expected non-short for 303 redirect")
	}
	if calls != 1 {
		t.Fatalf("expected one request, got %d", calls)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func mockResponse(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}
}
