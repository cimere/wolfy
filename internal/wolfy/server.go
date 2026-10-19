package wolfy

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Webhook struct {
	Store  *Store
	Config Config
	Now    func() time.Time
}

func (h *Webhook) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("GET /webhook", h.verify)
	mux.HandleFunc("POST /webhook", h.receive)
	return mux
}
func (h *Webhook) verify(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("hub.mode") != "subscribe" || subtle.ConstantTimeCompare([]byte(q.Get("hub.verify_token")), []byte(h.Config.VerifyToken)) != 1 || q.Get("hub.challenge") == "" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = io.WriteString(w, q.Get("hub.challenge"))
}
func (h *Webhook) receive(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 512<<10)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid payload size", http.StatusRequestEntityTooLarge)
		return
	}
	header := r.Header.Get("X-Hub-Signature-256")
	signature, err := hex.DecodeString(strings.TrimPrefix(header, "sha256="))
	mac := hmac.New(sha256.New, []byte(h.Config.AppSecret))
	_, _ = mac.Write(body)
	if err != nil || !strings.HasPrefix(header, "sha256=") || !hmac.Equal(signature, mac.Sum(nil)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var payload struct {
		Object string `json:"object"`
		Entry  []struct {
			Changes []struct {
				Value struct {
					Metadata struct {
						PhoneID string `json:"phone_number_id"`
					} `json:"metadata"`
					Messages []struct {
						ID        string `json:"id"`
						From      string `json:"from"`
						Timestamp string `json:"timestamp"`
						Type      string `json:"type"`
						Text      struct {
							Body string `json:"body"`
						} `json:"text"`
					} `json:"messages"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.Object != "whatsapp_business_account" {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	now := h.Now().UTC()
	var jobs []Job
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Value.Metadata.PhoneID != h.Config.PhoneID {
				continue
			}
			for _, message := range change.Value.Messages {
				user, err := Phone(message.From)
				if err != nil || message.ID == "" || len(message.ID) > 256 {
					continue
				}
				seconds, err := strconv.ParseInt(message.Timestamp, 10, 64)
				if err != nil {
					continue
				}
				sent := time.Unix(seconds, 0).UTC()
				if sent.After(now.Add(5*time.Minute)) || !sent.After(now.Add(-Retention)) {
					continue
				}
				kind, text := message.Type, message.Text.Body
				if kind != "text" {
					text = ""
				}
				if len(text) > 4000 {
					kind = "oversize"
					text = ""
				}
				if kind == "text" && strings.TrimSpace(text) == "" {
					continue
				}
				jobs = append(jobs, Job{ID: message.ID, User: user, Text: text, Kind: kind, Received: now, MessageTime: sent})
			}
		}
	}
	if err := h.Store.Enqueue(jobs); err != nil {
		if errors.Is(err, ErrQueueFull) {
			w.Header().Set("Retry-After", "60")
		}
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
