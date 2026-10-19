package wolfy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type OpenAI struct {
	Client *http.Client
	URL    string
	APIKey string
	Model  string
}

func (a *OpenAI) Generate(ctx context.Context, input ModelRequest) (ModelResult, error) {
	var result ModelResult
	body, err := json.Marshal(map[string]any{
		"model": a.Model, "store": false, "instructions": input.Instructions,
		"input": input.Input, "max_output_tokens": input.MaxOutput,
		"reasoning": map[string]string{"effort": "none"},
	})
	if err != nil {
		return result, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.URL, bytes.NewReader(body))
	if err != nil {
		return result, errors.New("invalid OpenAI URL")
	}
	req.Header.Set("Authorization", "Bearer "+a.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.Client.Do(req)
	if err != nil {
		return result, errors.New("OpenAI unavailable")
	}
	var response struct {
		Status string `json:"status"`
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage *struct {
			Input  int64 `json:"input_tokens"`
			Output int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := decodeResponse(resp, &response); err != nil {
		return result, err
	}
	if response.Usage != nil && response.Usage.Input > 0 && response.Usage.Output >= 0 {
		result.InputTokens = response.Usage.Input
		result.OutputTokens = response.Usage.Output
		result.UsageKnown = true
	}
	if response.Status != "completed" {
		return result, errors.New("OpenAI response incomplete")
	}
	var texts []string
	for _, item := range response.Output {
		if item.Type == "message" {
			for _, content := range item.Content {
				if content.Type == "output_text" {
					texts = append(texts, content.Text)
				}
			}
		}
	}
	result.Text = strings.TrimSpace(strings.Join(texts, "\n"))
	if result.Text == "" || len([]rune(result.Text)) > 3500 {
		return result, errors.New("OpenAI response empty or too long")
	}
	return result, nil
}

type Meta struct {
	Client *http.Client
	URL    string
	Token  string
}

func (m *Meta) Send(ctx context.Context, user, message string) (Delivery, error) {
	body, err := json.Marshal(map[string]any{"messaging_product": "whatsapp", "to": user, "type": "text", "text": map[string]any{"body": message, "preview_url": false}})
	if err != nil {
		return Delivery{Certain: true}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.URL, bytes.NewReader(body))
	if err != nil {
		return Delivery{Certain: true}, errors.New("invalid Meta URL")
	}
	req.Header.Set("Authorization", "Bearer "+m.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.Client.Do(req)
	if err != nil {
		return Delivery{}, errors.New("Meta send outcome unknown")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		_ = resp.Body.Close()
		return Delivery{Certain: true, Retry: true}, errors.New("Meta rate limit")
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		_ = resp.Body.Close()
		return Delivery{Certain: true}, fmt.Errorf("Meta rejected send: HTTP %d", resp.StatusCode)
	}
	// Transport failures and server errors may follow an accepted side effect.
	if resp.StatusCode >= 500 || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return Delivery{}, errors.New("Meta send outcome unknown")
	}
	var response struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := decodeResponse(resp, &response); err != nil {
		return Delivery{}, errors.New("Meta send acknowledgement unreadable")
	}
	if len(response.Messages) != 1 || response.Messages[0].ID == "" {
		return Delivery{}, errors.New("Meta send acknowledgement missing")
	}
	return Delivery{ID: response.Messages[0].ID}, nil
}

// Demo integrations never contact a provider and never log message contents.
type DemoDirectory struct{}

func (DemoDirectory) Authorized(_ context.Context, user string) (bool, error) {
	return user == "399000000001", nil
}
func (DemoDirectory) Knowledge(context.Context) (Knowledge, error) {
	return Knowledge{Contact: "responsabile dimostrativo", FAQ: []FAQ{{Question: "Come funziona la demo?", Answer: "Questa è una prova locale senza invii WhatsApp."}}}, nil
}

type DemoModel struct{}

func (DemoModel) Generate(context.Context, ModelRequest) (ModelResult, error) {
	return ModelResult{Text: "Demo locale: il flusso funziona. Nessun messaggio è stato inviato a WhatsApp.", InputTokens: 100, OutputTokens: 30, UsageKnown: true}, nil
}

type DemoMessenger struct{}

func (DemoMessenger) Send(context.Context, string, string) (Delivery, error) {
	return Delivery{ID: "demo-accepted"}, nil
}
