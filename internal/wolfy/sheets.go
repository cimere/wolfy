package wolfy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Sheets struct {
	Client  *http.Client
	BaseURL string
	SheetID string
}

func NewSheets(c Config) (*Sheets, error) {
	data, err := os.ReadFile(c.GoogleCredentials)
	if err != nil {
		return nil, errors.New("cannot read Google service account file")
	}
	jwt, err := google.JWTConfigFromJSON(data, "https://www.googleapis.com/auth/spreadsheets.readonly")
	if err != nil || jwt.TokenURL != "https://oauth2.googleapis.com/token" {
		return nil, errors.New("invalid Google service account configuration")
	}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, HTTPClient())
	client := jwt.Client(ctx)
	client.Timeout = 30 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Sheets{Client: client, BaseURL: "https://sheets.googleapis.com/v4/spreadsheets/", SheetID: c.SheetID}, nil
}

type valueRange struct {
	Values [][]string `json:"values"`
}

func (s *Sheets) fetch(ctx context.Context, ranges []string) ([]valueRange, error) {
	query := url.Values{"majorDimension": {"ROWS"}, "valueRenderOption": {"FORMATTED_VALUE"}}
	for _, r := range ranges {
		query.Add("ranges", r)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.BaseURL+url.PathEscape(s.SheetID)+"/values:batchGet?"+query.Encode(), nil)
	if err != nil {
		return nil, errors.New("invalid Sheets URL")
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, errors.New("Sheets unavailable")
	}
	var body struct {
		ValueRanges []valueRange `json:"valueRanges"`
	}
	if err := decodeResponse(resp, &body); err != nil {
		return nil, err
	}
	if len(body.ValueRanges) != len(ranges) {
		return nil, errors.New("missing Sheets ranges")
	}
	return body.ValueRanges, nil
}

var phonePattern = regexp.MustCompile(`^[1-9][0-9]{7,14}$`)

func Phone(raw string) (string, error) {
	value := strings.TrimPrefix(strings.TrimSpace(raw), "+")
	if !phonePattern.MatchString(value) {
		return "", errors.New("invalid international phone number")
	}
	return value, nil
}
func rows(r valueRange, headers []string, max int) ([][]string, error) {
	if len(r.Values) == 0 || len(r.Values[0]) != len(headers) {
		return nil, errors.New("invalid sheet header")
	}
	for i, h := range headers {
		if r.Values[0][i] != h {
			return nil, errors.New("unexpected sheet header")
		}
	}
	if len(r.Values) > max+1 {
		return nil, errors.New("sheet row limit exceeded")
	}
	var out [][]string
	for _, row := range r.Values[1:] {
		if strings.TrimSpace(strings.Join(row, "")) == "" {
			continue
		}
		if len(row) > len(headers) {
			return nil, errors.New("unexpected sheet columns")
		}
		copy := make([]string, len(headers))
		for i, value := range row {
			if len(value) > 1000 {
				return nil, errors.New("sheet cell too large")
			}
			copy[i] = strings.TrimSpace(value)
		}
		out = append(out, copy)
	}
	return out, nil
}

func (s *Sheets) Authorized(ctx context.Context, user string) (bool, error) {
	ranges, err := s.fetch(ctx, []string{"'Accessi'!A1:B102"})
	if err != nil {
		return false, err
	}
	values, err := rows(ranges[0], []string{"phone", "active"}, 100)
	if err != nil {
		return false, err
	}
	active := map[string]bool{}
	seen := map[string]bool{}
	for _, row := range values {
		phone, err := Phone(row[0])
		if err != nil {
			return false, err
		}
		if seen[phone] {
			return false, errors.New("duplicate phone in Accessi")
		}
		seen[phone] = true
		flag, err := strconv.ParseBool(strings.ToLower(row[1]))
		if err != nil {
			return false, errors.New("active must be TRUE or FALSE")
		}
		if flag {
			active[phone] = true
		}
	}
	if len(active) > 30 {
		return false, errors.New("more than 30 active users")
	}
	return active[user], nil
}

func (s *Sheets) Knowledge(ctx context.Context) (Knowledge, error) {
	var k Knowledge
	ranges, err := s.fetch(ctx, []string{"'Eventi'!A1:I202", "'Luoghi'!A1:C52", "'FAQ'!A1:C52", "'Impostazioni'!A1:B12"})
	if err != nil {
		return k, err
	}
	events, err := rows(ranges[0], []string{"id", "kind", "date", "time", "venue_id", "opponent", "status", "notes", "updated_at"}, 200)
	if err != nil {
		return k, err
	}
	venues, err := rows(ranges[1], []string{"id", "name", "address"}, 50)
	if err != nil {
		return k, err
	}
	faqs, err := rows(ranges[2], []string{"question", "answer", "updated_at"}, 50)
	if err != nil {
		return k, err
	}
	settings, err := rows(ranges[3], []string{"key", "value"}, 10)
	if err != nil {
		return k, err
	}
	venueIDs := map[string]bool{}
	for _, row := range venues {
		if row[0] == "" || row[1] == "" || row[2] == "" || venueIDs[row[0]] {
			return k, errors.New("invalid or duplicate venue")
		}
		venueIDs[row[0]] = true
		k.Venues = append(k.Venues, Venue{ID: row[0], Name: row[1], Address: row[2]})
	}
	ids := map[string]bool{}
	for _, row := range events {
		if row[0] == "" || ids[row[0]] || (row[1] != "allenamento" && row[1] != "partita") {
			return k, errors.New("invalid or duplicate event")
		}
		if _, err := time.Parse("2006-01-02", row[2]); err != nil {
			return k, errors.New("event date must be YYYY-MM-DD")
		}
		if _, err := time.Parse("15:04", row[3]); err != nil {
			return k, errors.New("event time must be HH:MM")
		}
		if !venueIDs[row[4]] {
			return k, errors.New("unknown event venue")
		}
		if row[6] != "confermato" && row[6] != "annullato" {
			return k, errors.New("event status must be confermato or annullato")
		}
		if _, err := time.Parse(time.RFC3339, row[8]); err != nil {
			return k, errors.New("updated_at must be RFC3339")
		}
		ids[row[0]] = true
		k.Events = append(k.Events, Event{ID: row[0], Kind: row[1], Date: row[2], Time: row[3], VenueID: row[4], Opponent: row[5], Status: row[6], Notes: row[7], Updated: row[8]})
	}
	for _, row := range faqs {
		if row[0] == "" || row[1] == "" {
			return k, errors.New("empty FAQ")
		}
		if _, err := time.Parse(time.RFC3339, row[2]); err != nil {
			return k, errors.New("FAQ updated_at must be RFC3339")
		}
		k.FAQ = append(k.FAQ, FAQ{Question: row[0], Answer: row[1], Updated: row[2]})
	}
	for _, row := range settings {
		if row[0] != "contact" {
			return k, fmt.Errorf("unsupported setting key")
		}
		if k.Contact != "" || row[1] == "" {
			return k, errors.New("invalid contact setting")
		}
		k.Contact = row[1]
	}
	if k.Contact == "" {
		return k, errors.New("contact is required")
	}
	return k, nil
}
