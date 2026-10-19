package wolfy

import (
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Config struct {
	Mode              string
	Listen            string
	DBPath            string
	AppSecret         string
	VerifyToken       string
	PhoneID           string
	MetaToken         string
	GraphVersion      string
	GoogleCredentials string
	SheetID           string
	OpenAIKey         string
	Model             string
	FallbackContact   string
	BudgetNano        int64
	InputNano         int64
	OutputNano        int64
	MaxOutput         int
	DailyLimit        int
}

func LoadConfig() (Config, error) {
	c := Config{
		Mode: env("WOLFY_MODE", "production"), Listen: env("WOLFY_LISTEN", "127.0.0.1:8080"),
		DBPath:    env("WOLFY_DB", "data/wolfy.db"),
		AppSecret: os.Getenv("META_APP_SECRET"), VerifyToken: os.Getenv("META_VERIFY_TOKEN"),
		PhoneID: os.Getenv("META_PHONE_NUMBER_ID"), MetaToken: os.Getenv("META_ACCESS_TOKEN"),
		GraphVersion:      os.Getenv("META_GRAPH_VERSION"),
		GoogleCredentials: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"), SheetID: os.Getenv("GOOGLE_SHEET_ID"),
		OpenAIKey: os.Getenv("OPENAI_API_KEY"), Model: env("OPENAI_MODEL", "gpt-5.4-mini-2026-03-17"),
		FallbackContact: os.Getenv("WOLFY_CONTACT"),
	}
	var err error
	if c.BudgetNano, err = number("WOLFY_BUDGET_NANO_USD", 2_000_000_000, 0, 100_000_000_000); err != nil {
		return c, err
	}
	// Nano-USD per token: $0.75/M input, $4.50/M output. Explicit pricing is required for other models.
	if c.InputNano, err = number("OPENAI_INPUT_NANO_USD", 750, 1, 1_000_000); err != nil {
		return c, err
	}
	if c.OutputNano, err = number("OPENAI_OUTPUT_NANO_USD", 4500, 1, 1_000_000); err != nil {
		return c, err
	}
	out, err := number("OPENAI_MAX_OUTPUT_TOKENS", 500, 64, 1000)
	if err != nil {
		return c, err
	}
	c.MaxOutput = int(out)
	daily, err := number("WOLFY_DAILY_LIMIT", 20, 1, 100)
	if err != nil {
		return c, err
	}
	c.DailyLimit = int(daily)
	if c.Mode != "production" && c.Mode != "demo" {
		return c, errors.New("WOLFY_MODE must be production or demo")
	}
	host, _, listenErr := net.SplitHostPort(c.Listen)
	if listenErr != nil || (host != "127.0.0.1" && host != "::1") {
		return c, errors.New("WOLFY_LISTEN must be a loopback address; expose HTTPS through Caddy")
	}
	if c.Mode == "demo" {
		host, _, err := net.SplitHostPort(c.Listen)
		if err != nil || (host != "127.0.0.1" && host != "::1") {
			return c, errors.New("demo must listen on loopback")
		}
		c.AppSecret = "demo-secret-not-for-production"
		c.VerifyToken = "demo-verification"
		c.PhoneID = "demo-phone"
		c.FallbackContact = "responsabile dimostrativo"
		return c, nil
	}
	for key, value := range map[string]string{
		"META_APP_SECRET": c.AppSecret, "META_VERIFY_TOKEN": c.VerifyToken,
		"META_PHONE_NUMBER_ID": c.PhoneID, "META_ACCESS_TOKEN": c.MetaToken,
		"META_GRAPH_VERSION": c.GraphVersion, "GOOGLE_APPLICATION_CREDENTIALS": c.GoogleCredentials,
		"GOOGLE_SHEET_ID": c.SheetID, "OPENAI_API_KEY": c.OpenAIKey, "WOLFY_CONTACT": c.FallbackContact,
	} {
		if strings.TrimSpace(value) == "" {
			return c, fmt.Errorf("missing %s", key)
		}
	}
	if len(c.AppSecret) < 16 || len(c.VerifyToken) < 16 {
		return c, errors.New("Meta verification credentials must be at least 16 characters")
	}
	if len(c.FallbackContact) > 500 {
		return c, errors.New("WOLFY_CONTACT exceeds 500 bytes")
	}
	if !regexp.MustCompile(`^v[0-9]+\.0$`).MatchString(c.GraphVersion) {
		return c, errors.New("invalid META_GRAPH_VERSION")
	}
	if !regexp.MustCompile(`^[0-9]+$`).MatchString(c.PhoneID) {
		return c, errors.New("invalid META_PHONE_NUMBER_ID")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(c.SheetID) {
		return c, errors.New("invalid GOOGLE_SHEET_ID")
	}
	if c.Model != "gpt-5.4-mini-2026-03-17" && (os.Getenv("OPENAI_INPUT_NANO_USD") == "" || os.Getenv("OPENAI_OUTPUT_NANO_USD") == "") {
		return c, errors.New("custom model requires explicit input/output prices")
	}
	return c, nil
}

func env(key, fallback string) string {
	if s := os.Getenv(key); s != "" {
		return s
	}
	return fallback
}
func number(key string, fallback, min, max int64) (int64, error) {
	s := os.Getenv(key)
	if s == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < min || n > max {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return n, nil
}
