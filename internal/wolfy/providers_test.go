package wolfy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestOpenAIRequestAndUsage(t *testing.T) {
	server := newMemoryServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["store"] != false || body["max_output_tokens"] != float64(500) || body["input"] != "question" {
			t.Error(body)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing authentication")
		}
		_, _ = io.WriteString(w, `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"Risposta"}]}],"usage":{"input_tokens":100,"output_tokens":20}}`)
	}))
	defer server.Close()
	model := &OpenAI{Client: server.Client(t), URL: server.URL, APIKey: "test-key", Model: "test"}
	r, err := model.Generate(context.Background(), ModelRequest{Input: "question", Instructions: "rules", MaxOutput: 500})
	if err != nil || r.Text != "Risposta" || !r.UsageKnown || r.InputTokens != 100 {
		t.Fatal(r, err)
	}
}
func TestIncompleteResponseStillReportsBillableUsage(t *testing.T) {
	server := newMemoryServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":"incomplete","usage":{"input_tokens":50,"output_tokens":500}}`)
	}))
	defer server.Close()
	m := &OpenAI{Client: server.Client(t), URL: server.URL}
	r, err := m.Generate(context.Background(), ModelRequest{})
	if err == nil || !r.UsageKnown || r.OutputTokens != 500 {
		t.Fatal(r, err)
	}
}
func TestMetaDistinguishesRejectionFromUnknownOutcome(t *testing.T) {
	for _, tc := range []struct {
		status                  int
		body                    string
		certain, retry, success bool
	}{
		{200, `{"messages":[{"id":"message"}]}`, false, false, true},
		{200, `{}`, false, false, false},
		{429, `{}`, true, true, false},
		{401, `{}`, true, false, false},
		{500, `{}`, false, false, false},
	} {
		server := newMemoryServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			data, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(data), `"preview_url":false`) {
				t.Error("link previews enabled")
			}
			w.WriteHeader(tc.status)
			_, _ = io.WriteString(w, tc.body)
		}))
		m := &Meta{Client: server.Client(t), URL: server.URL, Token: "test"}
		result, err := m.Send(context.Background(), "399000000001", "hello")
		server.Close()
		if result.Certain != tc.certain || result.Retry != tc.retry || (err == nil) != tc.success {
			t.Fatal(tc, result, err)
		}
	}
}
func TestHTTPClientDoesNotFollowRedirects(t *testing.T) {
	hit := false
	target := newMemoryServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit = true }))
	defer target.Close()
	redirect := newMemoryServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer redirect.Close()
	m := &Meta{Client: memoryClient(t, redirect, target), URL: redirect.URL, Token: "secret"}
	_, err := m.Send(context.Background(), "399000000001", "test")
	if err == nil || hit {
		t.Fatal("followed credential-bearing redirect")
	}
}
