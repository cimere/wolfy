package wolfy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func officialRanges() []valueRange {
	return []valueRange{
		{Values: [][]string{{"id", "kind", "date", "time", "venue_id", "opponent", "status", "notes", "updated_at"}, {"training-1", "allenamento", "2026-10-19", "20:00", "gym", "", "confermato", "", "2026-10-18T12:00:00+02:00"}}},
		{Values: [][]string{{"id", "name", "address"}, {"gym", "Palestra demo", "Via Esempio 1"}}},
		{Values: [][]string{{"question", "answer", "updated_at"}, {"Cosa porto?", "Scarpe e borraccia", "2026-10-18T12:00:00+02:00"}}},
		{Values: [][]string{{"key", "value"}, {"contact", "Responsabile dimostrativo"}}},
	}
}
func TestSheetsValidatesAndSeparatesAuthorization(t *testing.T) {
	ranges := officialRanges()
	auth := valueRange{Values: [][]string{{"phone", "active"}, {"+399000000001", "TRUE"}, {"399000000002", "FALSE"}}}
	server := newMemoryServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("write to Sheets")
		}
		values := ranges
		if len(r.URL.Query()["ranges"]) == 1 {
			values = []valueRange{auth}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"valueRanges": values})
	}))
	defer server.Close()
	s := &Sheets{Client: server.Client(t), BaseURL: server.URL + "/", SheetID: "sheet"}
	ok, err := s.Authorized(context.Background(), "399000000001")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	ok, err = s.Authorized(context.Background(), "399000000002")
	if err != nil || ok {
		t.Fatal(ok, err)
	}
	k, err := s.Knowledge(context.Background())
	if err != nil || len(k.Events) != 1 {
		t.Fatal(k, err)
	}
	data, _ := json.Marshal(k)
	if strings.Contains(string(data), "399000000001") {
		t.Fatal("authorization leaked into knowledge")
	}
	ranges[0].Values[1][6] = "annullato"
	k, err = s.Knowledge(context.Background())
	if err != nil || k.Events[0].Status != "annullato" {
		t.Fatal("stale calendar", err)
	}
	ranges[0].Values[1][4] = "missing"
	if _, err = s.Knowledge(context.Background()); err == nil {
		t.Fatal("invalid venue accepted")
	}
	auth.Values = append(auth.Values, []string{"399000000001", "TRUE"})
	if _, err = s.Authorized(context.Background(), "399000000001"); err == nil {
		t.Fatal("duplicate access row accepted")
	}
}
func TestMalformedSheetRowsAreRejected(t *testing.T) {
	for _, r := range []valueRange{
		{}, {Values: [][]string{{"wrong"}}}, {Values: [][]string{{"a"}, {strings.Repeat("x", 1001)}}}, {Values: [][]string{{"a"}, {"x"}, {"y"}}},
	} {
		if _, err := rows(r, []string{"a"}, 1); err == nil {
			t.Fatal("invalid rows accepted")
		}
	}
}

func TestSignedWebhookToHTTPProviders(t *testing.T) {
	w, _, _, _ := testWorker(t)
	modelCalls, sendCalls := 0, 0
	upstream := newMemoryServer(http.HandlerFunc(func(out http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sheet/values:batchGet":
			ranges := officialRanges()
			if len(r.URL.Query()["ranges"]) == 1 {
				ranges = []valueRange{{Values: [][]string{{"phone", "active"}, {"399000000001", "TRUE"}}}}
			}
			_ = json.NewEncoder(out).Encode(map[string]any{"valueRanges": ranges})
		case "/responses":
			modelCalls++
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), "399000000001") || !strings.Contains(string(body), "20:00") {
				t.Error("incorrect AI context")
			}
			_, _ = io.WriteString(out, `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"L’allenamento è alle 20:00."}]}],"usage":{"input_tokens":200,"output_tokens":30}}`)
		case "/messages":
			sendCalls++
			var body struct {
				To   string `json:"to"`
				Text struct {
					Body string `json:"body"`
				} `json:"text"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.To != "399000000001" || !strings.Contains(body.Text.Body, "20:00") {
				t.Error("wrong recipient or response")
			}
			_, _ = io.WriteString(out, `{"messages":[{"id":"sent-one"}]}`)
		default:
			t.Error("unexpected upstream path", r.URL.Path)
			http.NotFound(out, r)
		}
	}))
	defer upstream.Close()
	w.Directory = &Sheets{Client: upstream.Client(t), BaseURL: upstream.URL + "/", SheetID: "sheet"}
	w.Model = &OpenAI{Client: upstream.Client(t), URL: upstream.URL + "/responses", APIKey: "test", Model: "test"}
	w.Messenger = &Meta{Client: upstream.Client(t), URL: upstream.URL + "/messages", Token: "test"}
	h := &Webhook{Store: w.Store, Config: Config{AppSecret: "secret", PhoneID: "ours"}, Now: w.Now}
	body := webhookBody(w.Now(), "ours", "incoming", "Quando ci alleniamo?")
	for range 2 {
		response := httptest.NewRecorder()
		h.Handler().ServeHTTP(response, signedRequest(body, "secret"))
		if response.Code != 200 {
			t.Fatal(response.Code)
		}
	}
	stepTest(t, w)
	if modelCalls != 1 || sendCalls != 1 {
		t.Fatal("duplicate side effects", modelCalls, sendCalls)
	}
	j, err := w.Store.GetJob("incoming")
	if err != nil || j.State != Sent || j.ProviderID != "sent-one" {
		t.Fatal(j, err)
	}
	turns, err := w.Store.History("399000000001", w.Now())
	if err != nil || len(turns) != 2 {
		t.Fatal(turns, err)
	}
}
