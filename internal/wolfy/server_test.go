package wolfy

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func webhookBody(now time.Time, phoneID, id, text string) []byte {
	data, _ := json.Marshal(map[string]any{"object": "whatsapp_business_account", "entry": []any{map[string]any{"changes": []any{map[string]any{"value": map[string]any{
		"metadata": map[string]string{"phone_number_id": phoneID}, "messages": []any{map[string]any{"id": id, "from": "399000000001", "timestamp": strconv.FormatInt(now.Unix(), 10), "type": "text", "text": map[string]string{"body": text}}},
	}}}}}})
	return data
}
func signedRequest(body []byte, secret string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	return req
}
func TestWebhookSignaturePhoneAndDeduplication(t *testing.T) {
	s := testStore(t)
	now := time.Now().UTC()
	h := &Webhook{Store: s, Config: Config{AppSecret: "secret", VerifyToken: "verify", PhoneID: "ours"}, Now: func() time.Time { return now }}
	for _, tc := range []struct {
		name, secret, phone string
		status              int
	}{
		{"forged", "wrong", "ours", 403}, {"other-number", "secret", "other", 200}, {"valid", "secret", "ours", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := webhookBody(now, tc.phone, tc.name, "ciao")
			response := httptest.NewRecorder()
			h.Handler().ServeHTTP(response, signedRequest(body, tc.secret))
			if response.Code != tc.status {
				t.Fatal(response.Code)
			}
			_, err := s.GetJob(tc.name)
			if (tc.name == "valid") != (err == nil) {
				t.Fatal("unexpected persistence", err)
			}
		})
	}
	response := httptest.NewRecorder()
	h.Handler().ServeHTTP(response, signedRequest(webhookBody(now, "ours", "valid", "modified"), "secret"))
	j, _ := s.GetJob("valid")
	if j.Text != "ciao" {
		t.Fatal("duplicate replaced original")
	}
	response = httptest.NewRecorder()
	h.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/webhook?hub.mode=subscribe&hub.verify_token=verify&hub.challenge=123", nil))
	if response.Code != 200 || response.Body.String() != "123" {
		t.Fatal(response.Code, response.Body.String())
	}
}
func TestWebhookRejectsOldAndOversizeBodies(t *testing.T) {
	s := testStore(t)
	now := time.Now().UTC()
	h := &Webhook{Store: s, Config: Config{AppSecret: "secret", PhoneID: "ours"}, Now: func() time.Time { return now }}
	response := httptest.NewRecorder()
	h.Handler().ServeHTTP(response, signedRequest(webhookBody(now.Add(-Retention), "ours", "old", "ciao"), "secret"))
	if _, err := s.GetJob("old"); err == nil {
		t.Fatal("expired event queued")
	}
	response = httptest.NewRecorder()
	h.Handler().ServeHTTP(response, signedRequest(bytes.Repeat([]byte("x"), 513<<10), "secret"))
	if response.Code != 413 {
		t.Fatal(response.Code)
	}
}
