package wolfy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// Exercise real request construction/decoding without opening sockets. This
// keeps all integration tests offline and usable in restricted environments.
type memoryServer struct {
	URL     string
	Handler http.Handler
}

var serverSequence atomic.Uint64

func newMemoryServer(handler http.Handler) *memoryServer {
	return &memoryServer{URL: fmt.Sprintf("https://test-%d.invalid", serverSequence.Add(1)), Handler: handler}
}
func (*memoryServer) Close()                             {}
func (s *memoryServer) Client(t *testing.T) *http.Client { return memoryClient(t, s) }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func memoryClient(t *testing.T, servers ...*memoryServer) *http.Client {
	t.Helper()
	client := HTTPClient()
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		for _, s := range servers {
			if r.URL.Scheme+"://"+r.URL.Host == s.URL {
				out := httptest.NewRecorder()
				s.Handler.ServeHTTP(out, r)
				response := out.Result()
				response.Request = r
				return response, nil
			}
		}
		t.Errorf("unexpected external request: %s", r.URL.Host)
		return nil, fmt.Errorf("unexpected upstream host")
	})
	return client
}
