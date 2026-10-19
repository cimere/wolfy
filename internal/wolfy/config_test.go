package wolfy

import "testing"

func TestDemoConfigStaysLocal(t *testing.T) {
	t.Setenv("WOLFY_MODE", "demo")
	t.Setenv("WOLFY_LISTEN", "127.0.0.1:8080")
	t.Setenv("WOLFY_BUDGET_NANO_USD", "")
	t.Setenv("WOLFY_DAILY_LIMIT", "")
	c, err := LoadConfig()
	if err != nil || c.PhoneID != "demo-phone" || c.BudgetNano != 2_000_000_000 {
		t.Fatal(c, err)
	}
	t.Setenv("WOLFY_LISTEN", "0.0.0.0:8080")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("public demo allowed")
	}
}
func TestInvalidBudgetFailsClosed(t *testing.T) {
	t.Setenv("WOLFY_MODE", "demo")
	t.Setenv("WOLFY_BUDGET_NANO_USD", "-1")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("negative budget")
	}
}
func TestInternationalPhoneValidation(t *testing.T) {
	for _, raw := range []string{"0039 123", "not-a-number", "399", "0123456789", "399000000001<script>"} {
		if _, err := Phone(raw); err == nil {
			t.Fatal("accepted", raw)
		}
	}
	phone, err := Phone("+399000000001")
	if err != nil || phone != "399000000001" {
		t.Fatal(phone, err)
	}
}
